package control

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"testing"
	"time"

	"sneaky-port/internal/profile"
)

// kernelInterval retains actual mutation boundaries, including native command
// overhead; these are fixture-host timestamps, never cross-host delay evidence.
type kernelInterval struct {
	Start, End string
}

// TestIsolatedLinuxNetworkCases runs authentic SSH/UDP/TCP, persistence and
// authenticated reconciliation under kernel impairment in a disposable netns.
// WSL execution remains a named virtual-network case, not native platform support.
func TestIsolatedLinuxNetworkCases(t *testing.T) {
	if os.Getenv("SNEAKY_PORT_ISOLATED_NETNS") != "1" {
		t.Skip("opt in inside unshare --net with SNEAKY_PORT_ISOLATED_NETNS=1")
	}
	guardIsolatedNetwork(t)
	networkCommand(t, "ip", "link", "set", "lo", "up")
	for _, kind := range []string{"udp", "tcp"} {
		for _, scenario := range []string{"delay", "jitter", "periodic_spikes", "brief_interruption", "restrictive_mtu", "rate_limit"} {
			t.Run(kind+"/"+scenario, func(t *testing.T) { // Each scenario owns a fresh pinned SSH run and exact frozen plan.
				r := finiteRig(t, kind)
				p := &r.plan
				p.Warmup = 100e6
				p.Phases[0].Duration = 1200e6
				p.Phases[0].Client.RateNumerator = 30
				p.Phases[0].Server.RateNumerator = 20
				p.Grace, p.Deadline = 600e6, 20e6
				p.Limits.MaxLag = 100e6
				p.Limits.LedgerMessages = 256
				p.Limits.HeartbeatInterval = 100e6
				impaired := true
				t.Cleanup(func() { // Restore only this disposable namespace's qdisc and MTU.
					if impaired {
						networkCommand(t, "tc", "qdisc", "del", "dev", "lo", "root")
					}
					networkCommand(t, "ip", "link", "set", "lo", "mtu", "65536")
				})
				configureMeasurementNetem(t, kind, p.DestinationPort, []string{"delay", "10ms"}, []string{"delay", "20ms"})
				switch scenario {
				case "jitter":
					changeMeasurementNetem(t, "20:", "1:2", "delay", "10ms", "4ms")
					changeMeasurementNetem(t, "30:", "1:3", "delay", "20ms", "6ms")
				case "restrictive_mtu":
					networkCommand(t, "ip", "link", "set", "lo", "mtu", "1200")
					p.Phases[0].Client.ContentBytes, p.Phases[0].Server.ContentBytes = 1400, 1400
				case "rate_limit":
					p.Phases[0].Client.ContentBytes, p.Phases[0].Server.ContentBytes = 1400, 1400
					changeMeasurementNetem(t, "20:", "1:2", "delay", "10ms", "rate", "128kbit", "limit", "64")
					changeMeasurementNetem(t, "30:", "1:3", "delay", "20ms", "rate", "128kbit", "limit", "64")
				}
				var stopTrigger func()
				var intervals []kernelInterval
				if scenario == "periodic_spikes" || scenario == "brief_interruption" {
					stopTrigger = afterRunEvidence(t, r.client.Root, 200*time.Millisecond, func() { // Record actual command intervals; SSH/control is excluded by port filters.
						for i := 0; i < 2; i++ {
							begin := time.Now()
							args := []string{"delay", "80ms"}
							if scenario == "brief_interruption" {
								args = []string{"loss", "100%"}
							}
							if err := changeNetworkCommand("20:", "1:2", args...); err != nil {
								t.Error(err)
								return
							}
							time.Sleep(150 * time.Millisecond)
							if err := changeNetworkCommand("20:", "1:2", "delay", "10ms"); err != nil {
								t.Error(err)
								return
							}
							intervals = append(intervals, kernelInterval{begin.UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano)})
							t.Logf("upstream %s interval %d command start=%s completed=%s duration=%s; boundary includes tc invocation overhead", scenario, i+1, begin.UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), time.Since(begin))
							time.Sleep(150 * time.Millisecond)
						}
					})
					defer stopTrigger()
				}
				if err := p.Validate(); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
				defer cancel()
				result, err := Run(ctx, r.client, r.signer, *p)
				if stopTrigger != nil {
					stopTrigger()
					if len(intervals) != 2 {
						t.Fatalf("required two actual kernel intervals, observed %d", len(intervals))
					}
				}
				if err != nil || result.Local == nil || !result.Local.Complete || result.Remote == nil || result.Remote.Status != "verified" || result.Reconciliation == nil || result.Reconciliation.Status != "reconciled" {
					t.Fatalf("kernel impairment run/reconciliation: status=%s error=%v", result.Status, err)
				}
				replayed, err := ReplayReport(result.ReportPath, 8<<20)
				if err != nil || !reflect.DeepEqual(replayed.Evidence.Summary, result.Evidence.Summary) || !reflect.DeepEqual(replayed.Reconciliation, result.Reconciliation) {
					t.Fatalf("exact replay/accounting failed: %v", err)
				}
				var samples, late int
				var minimum, maximum profile.Decimal
				for _, entry := range result.Evidence.Entries {
					if entry.Identity.Lifecycle != 2 || entry.Attempt == nil || entry.Receipt == nil {
						continue
					}
					elapsed := *entry.Receipt - *entry.Attempt
					if samples == 0 || elapsed < minimum {
						minimum = elapsed
					}
					if elapsed > maximum {
						maximum = elapsed
					}
					samples++
					if *entry.Receipt >= entry.Deadline {
						late++
					}
				}
				if samples == 0 || scenario == "delay" && late == 0 {
					t.Fatal("no completed RTT population or expected baseline deadline misses")
				}
				if result.Reconciliation.Client.Summary.TransportPacketLoss != nil || result.Reconciliation.Server.Summary.TransportPacketLoss != nil {
					t.Fatal("application evidence invented underlying packet loss")
				}
				for _, format := range []string{"json", "jsonl", "csv"} {
					var exported bytes.Buffer
					if err := ExportReport(&exported, replayed, format); err != nil || exported.Len() == 0 {
						t.Fatalf("%s export: %v", format, err)
					}
				}
				stats := networkCommand(t, "tc", "-s", "qdisc", "show", "dev", "lo")
				if !bytes.Contains(stats, []byte("netem")) {
					t.Fatal("independent configured impairment evidence missing")
				}
				upstream := kernelQueueTruth(t, stats, "20:")
				downstream := kernelQueueTruth(t, stats, "30:")
				if upstream[0] == 0 || downstream[0] == 0 {
					t.Fatal("measurement did not traverse both independent impairment queues")
				}
				if scenario == "brief_interruption" && upstream[1] == 0 {
					t.Fatal("100-percent intervals did not drop any upstream packets")
				}
				if scenario == "periodic_spikes" && maximum < 80e6 {
					t.Fatal("no completed sample observed the declared 80-ms spike floor")
				}
				if scenario == "rate_limit" && (!bytes.Contains(stats, []byte("rate 128Kbit")) || !bytes.Contains(stats, []byte("limit 64")) || upstream[2]+downstream[2] == 0) {
					t.Fatal("declared rate/queue stimulus did not create observed backlog")
				}
				link := networkCommand(t, "ip", "-details", "link", "show", "lo")
				if scenario == "restrictive_mtu" {
					lo, err := net.InterfaceByName("lo")
					if err != nil || lo.MTU != 1200 || p.Phases[0].Client.ContentBytes <= lo.MTU {
						t.Fatal("requested restrictive MTU was not observed on the actual interface")
					}
				}
				retainNetworkEvidence(t, kind, scenario, result, stats, link, intervals)
				t.Logf("%s/%s: exact endpoint+authenticated replay; completed RTT samples=%d late=%d elapsed_min_ns=%d elapsed_max_ns=%d; qdisc truth:\n%s", kind, scenario, samples, late, minimum, maximum, stats)
				// Remove deliberate residual network backlog before testing resource reuse.
				// A shaped readiness timeout is not evidence of an endpoint reservation leak.
				networkCommand(t, "tc", "qdisc", "del", "dev", "lo", "root")
				impaired = false
				networkCommand(t, "ip", "link", "set", "lo", "mtu", "65536")
				assertReadyAfterDrain(t, r)
			})
		}
	}
}

// retainNetworkEvidence preserves only secret-free reports and independent
// qdisc/interface truth when explicitly requested; private stores stay temporary.
func retainNetworkEvidence(t *testing.T, kind, scenario string, result RunResult, stats, link []byte, intervals []kernelInterval) {
	t.Helper()
	root := os.Getenv("SNEAKY_PORT_NETWORK_EVIDENCE")
	if root == "" {
		return
	}
	destination := filepath.Join(root, kind+"-"+scenario, filepath.Base(result.ReportPath))
	if err := os.CopyFS(destination, os.DirFS(result.ReportPath)); err != nil {
		t.Fatal(err)
	}
	truth, err := json.MarshalIndent(map[string]any{
		"schema_version": 1, "case": kind + "/" + scenario, "captured_at_utc": time.Now().UTC().Format(time.RFC3339Nano),
		"qdisc": string(stats), "interface": string(link), "result": result,
		"actual_kernel_intervals": intervals,
		"scope":                   "isolated Linux IPv4 loopback kernel impairment; no physical/native-platform or ISP claim",
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(destination, "fixture-truth.json"), truth, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("retained secret-free raw reports and network truth: %s", destination)
}

// kernelQueueTruth reads independent tc packet/drop/backlog counters for one
// owned directional class. It never converts these into product packet-loss claims.
func kernelQueueTruth(t *testing.T, stats []byte, handle string) [3]uint64 {
	t.Helper()
	pattern := regexp.MustCompile(`qdisc netem ` + regexp.QuoteMeta(handle) + `[^\n]*\n\s*Sent [0-9]+ bytes ([0-9]+) pkt \(dropped ([0-9]+),[^\n]*\n\s*backlog [0-9]+b ([0-9]+)p`)
	matched := pattern.FindSubmatch(stats)
	if len(matched) != 4 {
		t.Fatalf("independent queue counters absent: %s", handle)
	}
	var out [3]uint64
	for i := range out {
		value, err := strconv.ParseUint(string(matched[i+1]), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		out[i] = value
	}
	return out
}

// guardIsolatedNetwork refuses global or multi-interface namespaces before any
// network mutation, so explicit opt-in cannot accidentally impair user traffic.
func guardIsolatedNetwork(t *testing.T) {
	t.Helper()
	self, err := os.Readlink("/proc/self/ns/net")
	if err != nil {
		t.Fatal(err)
	}
	initial, err := os.Readlink("/proc/1/ns/net")
	if err != nil || self == initial {
		t.Fatal("must use a new disposable network namespace distinct from PID 1")
	}
	interfaces, err := net.Interfaces()
	if err != nil || len(interfaces) != 1 || interfaces[0].Name != "lo" {
		t.Fatal("refusing namespace with user-owned/non-loopback interfaces")
	}
	t.Logf("isolated namespace=%s initial=%s; only owned loopback; UID=%d", self, initial, os.Geteuid())
}

// networkCommand records a bounded native command and stops on failure rather
// than silently substituting impairment settings or host tools.
func networkCommand(t *testing.T, program string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, program, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v %s", program, args, err, output)
	}
	return output
}

// configureMeasurementNetem selects only real measurement packets by carrier
// and actual server port; unrelated SSH/calibration/control remains unshaped.
func configureMeasurementNetem(t *testing.T, kind string, port int, upstream, downstream []string) {
	t.Helper()
	networkCommand(t, "tc", "qdisc", "add", "dev", "lo", "root", "handle", "1:", "prio", "bands", "3", "priomap", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0")
	for i, config := range [][]string{upstream, downstream} {
		networkCommand(t, "tc", append([]string{"qdisc", "add", "dev", "lo", "parent", fmt.Sprintf("1:%d", i+2), "handle", fmt.Sprintf("%d:", (i+2)*10), "netem"}, config...)...)
		field := "dst_port"
		if i == 1 {
			field = "src_port"
		}
		networkCommand(t, "tc", "filter", "add", "dev", "lo", "protocol", "ip", "parent", "1:", "prio", strconv.Itoa(i+1), "flower", "ip_proto", kind, field, strconv.Itoa(port), "classid", fmt.Sprintf("1:%d", i+2))
	}
}

// changeNetworkCommand applies a bounded change to an existing owned class and
// returns errors safely to trigger goroutines without exiting them via Fatal.
func changeNetworkCommand(handle, parent string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	command := append([]string{"qdisc", "change", "dev", "lo", "parent", parent, "handle", handle, "netem"}, args...)
	output, err := exec.CommandContext(ctx, "tc", command...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tc %v: %w %s", command, err, output)
	}
	return nil
}

// changeMeasurementNetem requires the exact declared qdisc change to succeed.
func changeMeasurementNetem(t *testing.T, handle, parent string, args ...string) {
	t.Helper()
	if err := changeNetworkCommand(handle, parent, args...); err != nil {
		t.Fatal(err)
	}
}
