package worker

import "container/heap"

// expiry retains an admission's deadline separately from its permanent receipt identity.
type expiry struct{ at, index uint64 }

// expiryHeap orders finite admitted identities by their scheduled-origin deadline.
type expiryHeap []expiry

// Len supplies the number of bounded expiry nodes to container/heap.
func (h expiryHeap) Len() int { return len(h) }

// Less orders deadlines, not arrival times or original sequence.
func (h expiryHeap) Less(i, j int) bool { return h[i].at < h[j].at }

// Swap maintains heap nodes without changing permanent attempt records.
func (h expiryHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

// Push adds one admitted identity; total nodes cannot exceed run-wide slots.
func (h *expiryHeap) Push(v any) { *h = append(*h, v.(expiry)) }

// Pop removes an elapsed deadline while keeping late receipts matchable.
func (h *expiryHeap) Pop() any { n := len(*h) - 1; v := (*h)[n]; *h = (*h)[:n]; return v }

// expireLocked frees outstanding capacity only strictly after the scheduled deadline.
// Expiry is evaluated before every admission, including after quiet phases; no timer is needed
// to release capacity while there is no work. Identity/late receipt evidence is retained.
func (u *Endpoint) expireLocked(now uint64) {
	for len(u.expiries) > 0 && u.expiries[0].at < now {
		e := heap.Pop(&u.expiries).(expiry)
		a := &u.attempts[e.index]
		if a.active {
			a.active = false
			u.outstanding--
		}
	}
}

// recordStop freezes the first observed local stop boundary before canceling owned work.
func (u *Endpoint) recordStop() uint64 {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.stopAt == 0 {
		u.stopAt = u.now()
	}
	return u.stopAt
}

// stoppedAt returns the frozen boundary without racing shutdown or explicit cancellation.
func (u *Endpoint) stoppedAt() uint64 { u.mu.Lock(); defer u.mu.Unlock(); return u.stopAt }
