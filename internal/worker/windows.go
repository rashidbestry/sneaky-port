package worker

import (
	"errors"
	"math"
	"sort"

	"sneaky-port/internal/profile"
)

// Window is a frozen half-open local interval; peer clocks are never mapped here.
// Deadline is scheduled-origin duration, shared by both directions of this phase.
type Window struct {
	Offset, Deadline uint64
	Local, Peer      Schedule
}

// PlanWindows restarts main patterns after warmup and assigns run-wide sequences.
// It validates the shared plan but performs no start, socket marking or clock negotiation.
func PlanWindows(p profile.Plan, direction byte) ([]Window, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if direction < 1 || direction > 2 {
		return nil, errors.New("invalid origin direction")
	}
	windows := make([]Window, 0, len(p.Phases)+1)
	var offset, local, peer uint64
	add := func(ph profile.Phase, lifecycle byte, index uint32, duration uint64) { // Freeze inherited deadlines and preserve independent directional sequence ranges.
		deadline := uint64(p.Deadline)
		if ph.Deadline != nil {
			deadline = uint64(*ph.Deadline)
		}
		a, b := ph.Client, ph.Server
		if direction == 2 {
			a, b = b, a
		}
		windows = append(windows, Window{Offset: offset, Deadline: deadline, Local: Schedule{a, duration, lifecycle, index, local}, Peer: Schedule{b, duration, lifecycle, index, peer}})
		n, _ := a.Pattern().Count(duration)
		local += n
		n, _ = b.Pattern().Count(duration)
		peer += n
		offset += duration
	}
	if p.Warmup > 0 {
		add(p.Phases[0], 1, 0, uint64(p.Warmup))
	}
	for i, ph := range p.Phases {
		add(ph, 2, uint32(i), uint64(ph.Duration))
	}
	return windows, nil
}

// runtimeWindow retains validated counts and content without allocating one payload per slot.
type runtimeWindow struct {
	Window
	localCount, peerCount     uint64
	localPayload, peerPayload []byte
}

// prepareWindows checks contiguity, identities and total budgets before transferring I/O.
// An omitted timeline preserves the earlier one-window component without deadline expiry.
func prepareWindows(c Config, maxFrame int) ([]runtimeWindow, uint64, uint64, error) {
	if c.Limits.LedgerMessages < 1 || c.Limits.LedgerMessages > 1<<20 {
		return nil, 0, 0, errors.New("invalid identity budget")
	}
	ws := c.Windows
	if len(ws) == 0 {
		ws = []Window{{Local: c.Local, Peer: c.Peer}}
	}
	if len(ws) > 1025 {
		return nil, 0, 0, errors.New("too many measurement windows")
	}
	out := make([]runtimeWindow, 0, len(ws))
	var local, peer, end uint64
	ids := map[[2]uint32]bool{}
	for i, w := range ws {
		n, a, err := validateSchedule(w.Local, maxFrame)
		if err != nil {
			return nil, 0, 0, err
		}
		m, b, err := validateSchedule(w.Peer, maxFrame)
		if err != nil {
			return nil, 0, 0, err
		}
		id := [2]uint32{uint32(w.Local.Lifecycle), w.Local.Phase}
		if w.Offset != end || len(c.Windows) > 0 && (w.Local.Duration != w.Peer.Duration || w.Local.Lifecycle != w.Peer.Lifecycle || w.Local.Phase != w.Peer.Phase) || ids[id] || w.Local.Duration > math.MaxInt64-end || w.Deadline > math.MaxInt64-end-w.Local.Duration || len(c.Windows) > 0 && w.Deadline == 0 || i > 0 && (w.Local.FirstSequence != ws[0].Local.FirstSequence+local || w.Peer.FirstSequence != ws[0].Peer.FirstSequence+peer) || n > uint64(c.Limits.LedgerMessages)-local || m > uint64(c.Limits.LedgerMessages)-peer {
			return nil, 0, 0, errors.New("invalid measurement timeline/budget")
		}
		ids[id] = true
		out = append(out, runtimeWindow{w, n, m, a, b})
		local += n
		peer += m
		end += w.Local.Duration
	}
	return out, local, peer, nil
}

// windowFor finds an original identity across warmup, quiet windows and changed payloads.
func (u *Endpoint) windowFor(sequence uint64, local bool) *runtimeWindow {
	i := sort.Search(len(u.windows), func(i int) bool { // Empty quiet ranges are skipped using sorted exclusive sequence ends.
		w := &u.windows[i]
		s, n := w.Peer, w.peerCount
		if local {
			s, n = w.Local, w.localCount
		}
		return s.FirstSequence+n > sequence
	})
	if i < len(u.windows) {
		w := &u.windows[i]
		s := w.Peer
		if local {
			s = w.Local
		}
		if sequence >= s.FirstSequence {
			return w
		}
	}
	return nil
}
