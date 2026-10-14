package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"sneaky-port/internal/engine"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
)

const irttOverlapBinarySHA256 = "4f1a294f8b89a601729cb9bcf4fe93f43084b690130444a5b8edb9f0e6c45d96"

// irttOverlapTrial retains exact observed populations and raw elapsed samples;
// missing or locally skipped work is not silently removed from compatibility.
type irttOverlapTrial struct {
	Tool                  string            `json:"tool"`
	Status                string            `json:"status"`
	PopulationsObserved   bool              `json:"populations_observed"`
	Error                 string            `json:"error,omitempty"`
	Arguments             []string          `json:"arguments,omitempty"`
	StartedUTC            string            `json:"started_utc"`
	FinishedUTC           string            `json:"finished_utc"`
	Planned               uint64            `json:"planned"`
	Attempted             uint64            `json:"attempted"`
	Written               uint64            `json:"written"`
	Skipped               uint64            `json:"skipped"`
	Completed             uint64            `json:"completed"`
	Unresolved            uint64            `json:"unresolved"`
	BytesSent             uint64            `json:"measurement_bytes_sent"`
	BytesReceived         uint64            `json:"measurement_bytes_received"`
	ElapsedNS             []uint64          `json:"elapsed_ns"`
	AttemptOffsetsNS      []uint64          `json:"attempt_offsets_ns"`
	ElapsedMedianNS       *string           `json:"nearest_rank_elapsed_median_ns"`
	ElapsedP95NS          *string           `json:"nearest_rank_elapsed_p95_ns"`
	SourcePort            string            `json:"source_port"`
	DestinationPort       string            `json:"destination_port"`
	ClockSource           string            `json:"clock_source"`
	ClockDetails          json.RawMessage   `json:"clock_details,omitempty"`
	ProductWriteRecords   []engine.Record   `json:"product_write_boundaries,omitempty"`
	ObservedConfiguration json.RawMessage   `json:"observed_configuration,omitempty"`
	CompatibilityGaps     []string          `json:"compatibility_gaps"`
	Files                 map[string]string `json:"file_sha256"`
}

// MarshalJSON retains null populations when an external process fails without
// publishing raw observations; unknown counts must never become measured zero.
func (trial irttOverlapTrial) MarshalJSON() ([]byte, error) {
	type plain irttOverlapTrial
	b, err := json.Marshal(plain(trial))
	if err != nil || trial.PopulationsObserved {
		return b, err
	}
	var public map[string]json.RawMessage
	if err = json.Unmarshal(b, &public); err != nil {
		return nil, err
	}
	for _, field := range []string{"attempted", "written", "skipped", "completed", "unresolved", "measurement_bytes_sent", "measurement_bytes_received", "elapsed_ns", "attempt_offsets_ns"} {
		public[field] = json.RawMessage("null")
	}
	return json.Marshal(public)
}

// irttOverlapPair keeps separate alternating runs; its delta is descriptive,
// with no engineering band or held-out designation when boundary bounds are absent.
type irttOverlapPair struct {
	Index               int              `json:"index"`
	Order               []string         `json:"order"`
	Product             irttOverlapTrial `json:"product"`
	Reference           irttOverlapTrial `json:"reference"`
	AbsoluteMedianDelta *string          `json:"absolute_median_delta_ns"`
}

// TestIRTTElapsedOverlap optionally runs a pinned external reference only against
// fresh owned loopback endpoints. Ten exploratory pairs cannot establish a band
// without independently justified capture-boundary uncertainty; no held-out trial
// or calibrated/full-engine/native-Linux/live-path acceptance is inferred.
func TestIRTTElapsedOverlap(t *testing.T) {
	binary := os.Getenv("SNEAKY_PORT_IRTT_BINARY")
	if binary == "" {
		t.Skip("set SNEAKY_PORT_IRTT_BINARY and SNEAKY_PORT_IRTT_EVIDENCE for the optional pinned Windows reference")
	}
	base := os.Getenv("SNEAKY_PORT_IRTT_EVIDENCE")
	if base == "" || !filepath.IsAbs(binary) || !filepath.IsAbs(base) {
		t.Fatal("explicit absolute external binary and evidence paths are required")
	}
	if referenceFileHash(t, binary) != irttOverlapBinarySHA256 {
		t.Fatal("reference binary differs from the audited IRTT 0.9.2 Windows artifact")
	}
	if err := os.MkdirAll(base, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(base, "windows-loopback-")
	if err != nil {
		t.Fatal(err)
	}
	address := startOverlapReferenceServer(t, binary, root)
	var pairs []irttOverlapPair
	for index := 0; index < 10; index++ {
		t.Run(fmt.Sprintf("exploratory_pair_%02d", index+1), func(t *testing.T) { // Finish each matched-shape pair before admitting the next, alternating order.
			directory := filepath.Join(root, fmt.Sprintf("pair-%02d", index+1))
			if err := os.Mkdir(directory, 0700); err != nil {
				t.Fatal(err)
			}
			pair := irttOverlapPair{Index: index + 1, Order: []string{"sneaky-port", "irtt"}}
			if index%2 == 0 {
				pair.Product = runOverlapProduct(t, directory)
				pair.Reference = runOverlapIRTT(t, binary, address, directory)
			} else {
				pair.Order = []string{"irtt", "sneaky-port"}
				pair.Reference = runOverlapIRTT(t, binary, address, directory)
				pair.Product = runOverlapProduct(t, directory)
			}
			if pair.Product.ElapsedMedianNS != nil && pair.Reference.ElapsedMedianNS != nil {
				a, _ := strconv.ParseUint(*pair.Product.ElapsedMedianNS, 10, 64)
				b, _ := strconv.ParseUint(*pair.Reference.ElapsedMedianNS, 10, 64)
				delta := max(a, b) - min(a, b)
				text := strconv.FormatUint(delta, 10)
				pair.AbsoluteMedianDelta = &text
			}
			pairs = append(pairs, pair)
			writeOverlapJSON(t, filepath.Join(directory, "pair.json"), pair)
			productMedian, referenceMedian := "unavailable", "unavailable"
			if pair.Product.ElapsedMedianNS != nil {
				productMedian = *pair.Product.ElapsedMedianNS
			}
			if pair.Reference.ElapsedMedianNS != nil {
				referenceMedian = *pair.Reference.ElapsedMedianNS
			}
			productCounts, referenceCounts := "unavailable", "unavailable"
			if pair.Product.PopulationsObserved {
				productCounts = fmt.Sprintf("%d/%d", pair.Product.Written, pair.Product.Completed)
			}
			if pair.Reference.PopulationsObserved {
				referenceCounts = fmt.Sprintf("%d/%d", pair.Reference.Written, pair.Reference.Completed)
			}
			t.Logf("pair %d order=%v product sent/samples=%s reference sent/samples=%s median ns=%s/%s; descriptive only", pair.Index, pair.Order, productCounts, referenceCounts, productMedian, referenceMedian)
		})
	}
	manifest := map[string]any{
		"schema": "sneaky-port/irtt-elapsed-overlap/v1", "recorded_at_utc": time.Now().UTC().Format(time.RFC3339Nano),
		"status": "inconclusive", "scope": "native Windows IPv4 loopback, separate alternating request/reply exploratory trials; no full-engine, Linux/native-platform or live-path acceptance",
		"reference_binary_sha256": irttOverlapBinarySHA256, "reference_release": "IRTT v0.9.2", "reference_commit": "0d612bb790034e5257a4979bcf655609f644d08a",
		"reference_bind_address": address, "reference_bind_reason": "Supported 127.0.0.2 loopback bind avoids pinned IRTT's Windows exact-interface MTU sentinel -1 receive-buffer bug; external source/binary remain unmodified. Product uses a separate 127.0.0.1 loopback binding; this is not a physical path equivalence claim.",
		"requested_shape":         map[string]any{"generation_ns": 1000000000, "interval_ns": 50000000, "expected_data_count": 20, "request_bytes": 124, "reply_bytes": 124, "warmup_ns": 0, "grace_or_wait_ns": 100000000, "independent_downstream_data": false},
		"quantity":                "per-tool client-local complete reply arrival minus pre-send attempt capture; nearest-rank median and p95 recomputed from uncorrected elapsed raw timestamps",
		"product_boundary":        "attempt precedes per-message header/encoding/HMAC and physical write; receipt capture follows complete application datagram read and precedes frame authentication",
		"irtt_boundary":           "recorder.recordPreSend follows prior packet preparation/HMAC; conn.receive captures after native read; raw client receive.monotonic-send.monotonic is used, not the processing-corrected delay.rtt field",
		"source_evidence":         []string{"IRTT recorder.go:recordPreSend/RoundTripData.RTT, client.go:send, conn.go:cconn.receive, timesrc_win.go:WindowsTimeSource; retained pinned source archive", "sneaky-port internal/worker/udp.go and internal/engine/endpoint.go; retained public product raw records"},
		"boundary_uncertainty_ns": nil, "engineering_band_ns": nil, "calibrated_baseline_pairs": 0, "exploratory_pairs": len(pairs), "held_out_pairs": 0,
		"reason":     "Independent bounds for differing preparation/capture costs, clock-source quantization/rate and process-local timestamp overhead are not established. Baseline variation cannot manufacture those bounds; no band was frozen and no held-out acceptance was attempted.",
		"other_gaps": []string{"Protocol identity/authentication/server processing and control traffic differ despite matched datagram sizes/rates", "IRTT may finish its reply wait early when all replies arrive; missing-population cutoffs are not interchangeable", "Product retains SSH heartbeat/calibration/report control; IRTT open/close control is separately logged", "IRTT scheduling may skip intervals under load; observed counts/attempt offsets are retained", "No offload/outer marking/physical path/packet-loss/cross-host clock/loaded scenario claim"},
		"pairs":      pairs,
	}
	writeOverlapJSON(t, filepath.Join(root, "manifest.json"), manifest)
	t.Logf("INCONCLUSIVE: retained ten exploratory pairs at %s; independent boundary uncertainty unavailable, band=null, held_out_pairs=0", root)
}

// referenceFileHash records bytes from explicitly selected public artifacts,
// never inspecting a credential store or inferring provenance from a filename.
func referenceFileHash(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// writeOverlapJSON preserves exact raw integer/decimal values in a newly owned
// evidence directory; these offline artifacts do not renew authentication.
func writeOverlapJSON(t *testing.T, path string, value any) {
	t.Helper()
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}

// startOverlapReferenceServer binds the audited external tool only to a fresh
// loopback UDP port, owns its bounded process lifetime and joins it on cleanup.
// The 127.0.0.2 alias avoids IRTT 0.9.2 treating Windows's exact-interface MTU
// sentinel -1 as a 92-byte receive capacity; its normal unmatched-IP fallback
// preserves 124-byte test packets without modifying the tool or OS interfaces.
func startOverlapReferenceServer(t *testing.T, binary, root string) string {
	t.Helper()
	reservation, err := net.ListenPacket("udp4", "127.0.0.2:0")
	if err != nil {
		t.Fatal(err)
	}
	address := reservation.LocalAddr().String()
	reservation.Close()
	log, err := os.OpenFile(filepath.Join(root, "irtt-server.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	command := exec.CommandContext(ctx, binary, "server", "-4", "-b", address, "--fill=none", "--tstamp=none", "--no-dscp", "--timeout=5s")
	command.Stdout, command.Stderr, command.Dir = log, log, root
	if err = command.Start(); err != nil {
		cancel()
		log.Close()
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { // Retain the sole process wait owner so no external server is left running after the fixture.
		done <- command.Wait()
	}()
	t.Cleanup(func() { // Kill only this fixture-created server and join it before closing its owned log.
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			command.Process.Kill()
			t.Error("IRTT server exceeded bounded process cleanup")
		}
		log.Close()
	})
	writeOverlapJSON(t, filepath.Join(root, "irtt-server-command.json"), map[string]any{"arguments": command.Args[1:], "address": address, "binary_sha256": irttOverlapBinarySHA256, "bind_configuration_reason": "IRTT 0.9.2 mtu.go:detectMTU returns Windows exact 127.0.0.1 interface MTU -1; server.go:newListener passes it to packet.go:newPacket, which clamps capacity to maxHeaderLen=92, below requested 124 bytes. Supported 127.0.0.2 binding selects normal unmatched-interface fallback; no tool-source or interface-setting changes."})
	return address
}

// runOverlapProduct records a genuine 20-Hz upstream-only UDP run with complete
// authenticated receiver transfer/replay; actual skips/missing populations survive.
func runOverlapProduct(t *testing.T, directory string) irttOverlapTrial {
	t.Helper()
	r := finiteRig(t, "udp")
	p := &r.plan
	p.Warmup, p.Grace, p.Deadline = 0, 100e6, 100e6
	p.Phases[0].Duration = 1e9
	p.Phases[0].Client.RateNumerator, p.Phases[0].Client.RateDenominator = 20, 1
	p.Phases[0].Client.ContentBytes, p.Phases[0].Client.ContentPattern = 0, ""
	p.Phases[0].Server.Mode, p.Phases[0].Server.RateNumerator, p.Phases[0].Server.RateDenominator = "quiet", 0, 1
	p.Phases[0].Server.ContentBytes, p.Phases[0].Server.ContentPattern = 0, ""
	p.Limits.MaxLag, p.Limits.HeartbeatInterval = 100e6, 100e6
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	trial := irttOverlapTrial{Tool: "sneaky-port", StartedUTC: time.Now().UTC().Format(time.RFC3339Nano), ClockSource: "unavailable; no finalized public clock provenance read yet", Files: map[string]string{}}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	result, err := Run(ctx, r.client, r.signer, *p)
	trial.FinishedUTC, trial.Status = time.Now().UTC().Format(time.RFC3339Nano), result.Status
	if err != nil {
		trial.Error = err.Error()
		trial.CompatibilityGaps = append(trial.CompatibilityGaps, "product run incomplete")
	}
	if result.ReportPath == "" {
		return trial
	}
	replayed, replayErr := ReplayReport(result.ReportPath, 8<<20)
	if replayErr != nil || result.Reconciliation == nil || replayed == nil || !reflect.DeepEqual(replayed.Reconciliation, result.Reconciliation) {
		t.Fatalf("product exact public replay/reconciliation: %v", replayErr)
	}
	trial.ClockSource = replayed.Manifest.Clock
	if replayed.Manifest.ClockDetails != nil {
		trial.ClockDetails, err = json.Marshal(replayed.Manifest.ClockDetails)
		if err != nil {
			t.Fatal(err)
		}
	}
	summary := replayed.Evidence.Summary
	trial.Planned, trial.Attempted, trial.Written, trial.Skipped, trial.Completed, trial.Unresolved = uint64(summary.Planned), uint64(summary.Attempted), uint64(summary.Written), uint64(summary.Skipped), uint64(summary.RTTSamples), uint64(summary.Unresolved)
	trial.PopulationsObserved = true
	var firstAttempt *profile.Decimal
	for _, entry := range replayed.Evidence.Entries {
		if entry.Identity.Lifecycle != 2 {
			continue
		}
		if entry.Attempt != nil {
			if firstAttempt == nil {
				firstAttempt = entry.Attempt
			}
			trial.AttemptOffsetsNS = append(trial.AttemptOffsetsNS, uint64(*entry.Attempt-*firstAttempt))
		}
		if entry.Attempt != nil && entry.Receipt != nil && *entry.Receipt < replayed.Evidence.ObservationCutoff {
			trial.ElapsedNS = append(trial.ElapsedNS, uint64(*entry.Receipt-*entry.Attempt))
		}
	}
	for _, raw := range replayed.Evidence.Records {
		var record engine.Record
		if err := json.Unmarshal(raw, &record); err != nil {
			t.Fatal(err)
		}
		if record.Identity.Lifecycle != 2 {
			continue
		}
		if record.Kind == "write" && record.FrameKind == probe.Data {
			trial.BytesSent += uint64(record.Bytes)
			trial.ProductWriteRecords = append(trial.ProductWriteRecords, record)
			if record.Bytes != 124 {
				trial.CompatibilityGaps = append(trial.CompatibilityGaps, "product data write differs from 124 bytes")
			}
		}
		if record.Kind == "arrival" && record.FrameKind == probe.Receipt {
			trial.BytesReceived += uint64(record.Bytes)
			if record.Bytes != 124 {
				trial.CompatibilityGaps = append(trial.CompatibilityGaps, "product receipt differs from 124 bytes")
			}
		}
	}
	trial.ElapsedMedianNS, trial.ElapsedP95NS = measure.Percentile(trial.ElapsedNS, 50), measure.Percentile(trial.ElapsedNS, 95)
	if trial.Planned != 20 || trial.Written != 20 || trial.Completed != 20 || trial.Skipped != 0 || trial.Unresolved != 0 || replayed.ServerReport == nil {
		trial.CompatibilityGaps = append(trial.CompatibilityGaps, "product offered/completed population or receiver evidence differs from the requested 20 replies")
	}
	if replayed.ServerReport != nil && replayed.ServerReport.Evidence.Summary.Planned != 0 {
		trial.CompatibilityGaps = append(trial.CompatibilityGaps, "product server emitted independent downstream data")
	}
	trial.SourcePort, trial.DestinationPort = referenceAddressPort(result.Readiness.Client.Local), referenceAddressPort(result.Readiness.Server.Local)
	trial.ObservedConfiguration, _ = json.Marshal(replayed.Manifest.Plan)
	if err = os.CopyFS(filepath.Join(directory, "product-report"), os.DirFS(result.ReportPath)); err != nil {
		t.Fatal(err)
	}
	writeOverlapJSON(t, filepath.Join(directory, "product-public-result.json"), result)
	trial.Files["product-report/manifest.json"] = referenceFileHash(t, filepath.Join(directory, "product-report", "manifest.json"))
	trial.Files["product-public-result.json"] = referenceFileHash(t, filepath.Join(directory, "product-public-result.json"))
	return trial
}

// runOverlapIRTT parses only the audited reference's raw local send/receive
// timestamps. Its reported corrected RTT, one-way estimates and loss percentages
// never substitute for product elapsed samples or complete receiver evidence.
func runOverlapIRTT(t *testing.T, binary, address, directory string) irttOverlapTrial {
	t.Helper()
	output := filepath.Join(directory, "irtt-raw.json")
	args := []string{"client", "-4", "-Q", "-d", "1s", "-i", "50ms", "-l", "124", "--tstamp=none", "--clock=monotonic", "--stats=both", "--fill=none", "--wait=100ms", "--timeouts=200ms,400ms", "-o", output, address}
	trial := irttOverlapTrial{Tool: "irtt", Planned: 20, Arguments: args, StartedUTC: time.Now().UTC().Format(time.RFC3339Nano), Files: map[string]string{}}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = directory
	log, err := command.CombinedOutput()
	trial.FinishedUTC = time.Now().UTC().Format(time.RFC3339Nano)
	if writeErr := os.WriteFile(filepath.Join(directory, "irtt-client.log"), log, 0600); writeErr != nil {
		t.Fatal(writeErr)
	}
	if err != nil {
		trial.Status, trial.Error = "partial", err.Error()
		trial.CompatibilityGaps = append(trial.CompatibilityGaps, "reference process failed")
		return trial
	}
	trial.Status = "completed"
	var raw struct {
		Config struct {
			LocalAddress  string `json:"local_address"`
			RemoteAddress string `json:"remote_address"`
			TimeSource    string `json:"time_source"`
			Params        struct {
				Duration int64 `json:"duration"`
				Interval int64 `json:"interval"`
				Length   int   `json:"length"`
			} `json:"params"`
		} `json:"config"`
		Stats struct {
			PacketsSent     uint64 `json:"packets_sent"`
			PacketsReceived uint64 `json:"packets_received"`
			BytesSent       uint64 `json:"bytes_sent"`
			BytesReceived   uint64 `json:"bytes_received"`
			Duplicates      uint64 `json:"duplicates"`
		} `json:"stats"`
		RoundTrips []struct {
			Sequence   uint64 `json:"seqno"`
			Timestamps struct {
				Client struct {
					Send, Receive struct {
						Monotonic int64 `json:"monotonic"`
					}
				} `json:"client"`
			} `json:"timestamps"`
		} `json:"round_trips"`
	}
	bytes, readErr := os.ReadFile(output)
	if readErr != nil || json.Unmarshal(bytes, &raw) != nil {
		t.Fatal("reference raw JSON missing or invalid")
	}
	trial.ClockSource = raw.Config.TimeSource
	trial.PopulationsObserved = true
	trial.Planned, trial.Attempted, trial.Written = 20, raw.Stats.PacketsSent, raw.Stats.PacketsSent
	trial.BytesSent, trial.BytesReceived = raw.Stats.BytesSent, raw.Stats.BytesReceived
	seen := map[uint64]bool{}
	var firstSend int64
	for _, round := range raw.RoundTrips {
		if seen[round.Sequence] {
			t.Fatal("reference duplicate sequence in retained raw records")
		}
		seen[round.Sequence] = true
		send, receive := round.Timestamps.Client.Send.Monotonic, round.Timestamps.Client.Receive.Monotonic
		if firstSend == 0 {
			firstSend = send
		}
		if send < firstSend {
			t.Fatal("reference send order moved backward")
		}
		trial.AttemptOffsetsNS = append(trial.AttemptOffsetsNS, uint64(send-firstSend))
		if send > 0 && receive >= send {
			trial.ElapsedNS = append(trial.ElapsedNS, uint64(receive-send))
		} else {
			trial.Unresolved++
		}
	}
	trial.Completed = uint64(len(trial.ElapsedNS))
	if trial.Written <= trial.Planned {
		trial.Skipped = trial.Planned - trial.Written
	}
	if raw.Config.Params.Length != 124 || raw.Config.Params.Duration != 1e9 || raw.Config.Params.Interval != 50e6 || trial.Written != 20 || trial.Completed != 20 || raw.Stats.PacketsReceived != 20 || raw.Stats.Duplicates != 0 || trial.BytesSent != 2480 || trial.BytesReceived != 2480 {
		trial.CompatibilityGaps = append(trial.CompatibilityGaps, "reference actual size/rate/population differs from 20 complete 124-byte request/reply exchanges")
	}
	trial.ElapsedMedianNS, trial.ElapsedP95NS = measure.Percentile(trial.ElapsedNS, 50), measure.Percentile(trial.ElapsedNS, 95)
	trial.SourcePort, trial.DestinationPort = referenceAddressPort(raw.Config.LocalAddress), referenceAddressPort(raw.Config.RemoteAddress)
	var config struct {
		Config json.RawMessage `json:"config"`
	}
	json.Unmarshal(bytes, &config)
	trial.ObservedConfiguration = config.Config
	trial.Files["irtt-raw.json"] = referenceFileHash(t, output)
	trial.Files["irtt-client.log"] = referenceFileHash(t, filepath.Join(directory, "irtt-client.log"))
	return trial
}

// referenceAddressPort extracts only a public observed binding; unavailable or
// unparsable values remain explicit rather than receiving a requested port.
func referenceAddressPort(address string) string {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return "unavailable"
	}
	return port
}
