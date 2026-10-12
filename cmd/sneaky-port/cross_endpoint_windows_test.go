//go:build windows

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
	"sneaky-port/internal/control"
	"sneaky-port/internal/profile"
)

// TestWindowsLinuxCLI is an explicitly enabled real-process WSL workflow gate.
// It retains isolated artifacts, never enrolls into existing stores, and proves
// this named virtual-network case only, not native ARM64 or live-path behavior.
func TestWindowsLinuxCLI(t *testing.T) {
	distro := os.Getenv("SNEAKY_PORT_TASK23_WSL")
	if distro == "" {
		t.Skip("opt in with SNEAKY_PORT_TASK23_WSL set to an existing WSL distribution")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	t.Cleanup(cancel)
	workspace, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Join(workspace, "test-output"), 0700); err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(filepath.Join(workspace, "test-output"), "task23-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("retained Windows artifacts: %s", root)
	linux := func(args ...string) []byte { // Execute bounded commands in the explicitly selected existing distribution.
		return task23Command(t, ctx, "wsl.exe", append([]string{"-d", distro, "--"}, args...)...)
	}
	// /var/tmp survives an idle WSL restart during slower instrumented builds.
	lroot := strings.TrimSpace(string(linux("mktemp", "-d", "/var/tmp/sneaky-port-task23-XXXXXXXX")))
	t.Logf("retained Linux artifacts: %s", lroot)
	if !strings.HasPrefix(lroot, "/var/tmp/sneaky-port-task23-") || strings.ContainsAny(lroot, "\r\n '") {
		t.Fatal("unexpected owned Linux fixture path")
	}
	mount := strings.TrimSpace(string(linux("wslpath", "-a", "-u", filepath.ToSlash(root))))
	metadata := linux("sh", "-c", "cat /etc/os-release; uname -r; uname -m; id")
	task23Save(t, filepath.Join(root, "linux-environment.txt"), metadata)
	task23Save(t, filepath.Join(root, "windows-environment.txt"), append(task23Command(t, ctx, "cmd.exe", "/c", "ver"), []byte(runtime.Version()+" "+runtime.GOOS+"/"+runtime.GOARCH+"\n")...))
	ips := strings.Fields(string(linux("hostname", "-I")))
	var ip string
	for _, candidate := range ips {
		if parsed := net.ParseIP(candidate); parsed != nil && parsed.To4() != nil && !parsed.IsLoopback() {
			ip = candidate
			break
		}
	}
	if ip == "" {
		t.Fatal("no non-loopback WSL IPv4 address")
	}
	// Reserve-check both data transports together; a later collision must fail explicitly.
	ports := strings.Fields(string(linux("python3", "-c", "import socket; c=socket.socket(); c.bind(('0.0.0.0',0)); u=socket.socket(socket.AF_INET,socket.SOCK_DGRAM); u.bind(('0.0.0.0',0)); s=socket.socket(); s.bind(('0.0.0.0',u.getsockname()[1])); print(c.getsockname()[1],u.getsockname()[1])")))
	if len(ports) != 2 {
		t.Fatal("port discovery failed")
	}
	controlPort, err := strconv.Atoi(ports[0])
	if err != nil {
		t.Fatal(err)
	}
	dataPort, err := strconv.Atoi(ports[1])
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "client.exe")
	for _, target := range []struct{ os, name string }{{"windows", "client.exe"}, {"linux", "server"}} {
		cmd := exec.CommandContext(ctx, "go", "build", "-o", filepath.Join(root, target.name), "./cmd/sneaky-port")
		cmd.Dir = workspace
		cmd.Env = append(os.Environ(), "GOOS="+target.os, "GOARCH=amd64", "CGO_ENABLED=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s build: %v: %s", target.os, err, out)
		}
	}
	lbin, ldata := lroot+"/server", lroot+"/server-data"
	linux("cp", mount+"/server", lbin)
	linux("chmod", "700", lbin)
	policyBytes, err := os.ReadFile(filepath.Join(workspace, "examples", "readiness-policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	var policy control.ServerPolicy
	if err = json.Unmarshal(policyBytes, &policy); err != nil {
		t.Fatal(err)
	}
	policy.ControlListen = net.JoinHostPort(ip, ports[0])
	policy.DataBindIP, policy.PortMin, policy.PortMax = ip, dataPort, dataPort
	task23JSON(t, filepath.Join(root, "policy.json"), policy)
	cdata := filepath.Join(root, "client-data")
	task23Save(t, filepath.Join(root, "client-public.json"), task23Command(t, ctx, exe, "client-init", "-data-dir", cdata, "-client-id", "task23-client"))
	task23Save(t, filepath.Join(root, "server-public.json"), linux(lbin, "server-init", "-data-dir", ldata, "-server-id", "task23-server", "-advertise", net.JoinHostPort(ip, ports[0]), "-policy", mount+"/policy.json"))
	linux(lbin, "server-enroll", "-data-dir", ldata, "-enrollment", mount+"/client-public.json")
	task23Command(t, ctx, exe, "client-import", "-data-dir", cdata, "-pairing-id", "lab-server", "-bundle", filepath.Join(root, "server-public.json"))
	serverLog, err := os.Create(filepath.Join(root, "server.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // Close the log after the later server cleanup has joined its writer.
		serverLog.Close()
	})
	// Script files avoid WSL's extra command-line shell expansion of dollar signs and quotes.
	task23Save(t, filepath.Join(root, "start-server.sh"), []byte("echo $$ > '"+lroot+"/pid'\nexec '"+lbin+"' server -data-dir '"+ldata+"'\n"))
	task23Save(t, filepath.Join(root, "stop-server.sh"), []byte("p=$(cat '"+lroot+"/pid')\ntest \"$(readlink /proc/$p/exe)\" = '"+lbin+"' && kill -INT \"$p\"\n"))
	server := exec.CommandContext(ctx, "wsl.exe", "-d", distro, "--", "sh", mount+"/start-server.sh")
	server.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	server.Stdout, server.Stderr = serverLog, serverLog
	if err = server.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { // Join the owned WSL foreground server without leaving a detached process waiter.
		done <- server.Wait()
	}()
	t.Cleanup(func() { // Verify the exact owned executable before signaling; preserve all evidence and unrelated processes.
		stopCtx, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		cmd := exec.CommandContext(stopCtx, "wsl.exe", "-d", distro, "--", "sh", mount+"/stop-server.sh")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("owned server signal: %v: %s", err, output)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("server exit: %v", err)
			}
		case <-stopCtx.Done():
			t.Error("owned server did not join")
		}
		if conn, err := net.DialTimeout("tcp4", net.JoinHostPort(ip, strconv.Itoa(controlPort)), time.Second); err == nil {
			conn.Close()
			t.Error("owned SSH listener survived shutdown")
		}
	})
	readyDeadline := time.Now().Add(15 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp4", net.JoinHostPort(ip, ports[0]), 200*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		if time.Now().After(readyDeadline) {
			t.Fatal("owned server unreachable; inspect server.log")
		}
		time.Sleep(50 * time.Millisecond)
	}
	task23Console(t)
	profileBytes, err := os.ReadFile(filepath.Join(workspace, "examples", "readiness-udp.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := profile.Decode(bytes.NewReader(profileBytes))
	if err != nil {
		t.Fatal(err)
	}
	p.ServerAddress, p.DestinationPort = ip, dataPort
	// This functional gate measures a small explicit workload, not performance or recommended FPS settings.
	p.Phases[0].Duration, p.Phases[0].Client.RateNumerator, p.Phases[0].Server.RateNumerator = 2e9, 30, 20
	p.Calibration.Cadence = 200e6
	// Retain a bounded whole-trial cushion for slow disk publication; saturation is a separate gate.
	p.Limits.QueueMessages = 512
	for _, carrier := range []string{"udp", "tcp"} {
		p.Adapter, p.Options = carrier, map[string]json.RawMessage{}
		if carrier == "tcp" {
			p.Options["tcp_no_delay"] = json.RawMessage("true")
		}
		path := filepath.Join(root, carrier+".json")
		task23JSON(t, path, p)
		var ready control.ReadyResult
		b := task23Command(t, ctx, exe, "client-ready", "-data-dir", cdata, "-profile", path, "-hold-ns", "200000000")
		task23Save(t, filepath.Join(root, carrier+"-ready.json"), b)
		if err = json.Unmarshal(b, &ready); err != nil || ready.Status != "readiness_checked" || ready.MeasurementStarted {
			t.Fatalf("%s readiness: %v", carrier, err)
		}
		for _, interrupted := range []bool{false, true} {
			name := carrier + "-complete"
			if interrupted {
				name = carrier + "-cancel"
				p.Phases[0].Duration = 8e9
			} else {
				p.Phases[0].Duration = 2e9
			}
			task23JSON(t, path, p)
			result := task23Run(t, ctx, exe, cdata, path, root, name, interrupted)
			task23Reports(t, ctx, exe, root, name, result, interrupted)
			if _, err := os.Stat(filepath.Join(cdata, "active-run.lock")); !os.IsNotExist(err) {
				t.Fatal("client reservation survived normal cleanup")
			}
			// Rebind both server data transports only after stop/transfer has joined.
			linux("python3", "-c", "import socket; u=socket.socket(socket.AF_INET,socket.SOCK_DGRAM); u.bind(('"+ip+"',"+ports[1]+")); s=socket.socket(); s.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1); s.bind(('"+ip+"',"+ports[1]+")); print('owned data port released')")
			lreport := ldata + "/reports/" + result.Readiness.RunID + "-server"
			serverReplay := linux(lbin, "report-replay", "-report", lreport, "-max-bytes", "33554432")
			task23Save(t, filepath.Join(root, name+"-server-replay.json"), serverReplay)
			var sr control.ReplayedReport
			if err := json.Unmarshal(serverReplay, &sr); err != nil || sr.Verification != "matched" || sr.Status != result.Status || sr.Manifest.RunID != result.Readiness.RunID || sr.Manifest.PlanHash != result.Readiness.PlanHash || sr.Manifest.OS != "linux" || sr.Manifest.Arch != "amd64" {
				t.Fatalf("Linux endpoint replay: %v", err)
			}
			for _, format := range []string{"json", "jsonl", "csv"} {
				task23Save(t, filepath.Join(root, name+"-server."+format), linux(lbin, "report-export", "-report", lreport, "-max-bytes", "33554432", "-format", format))
			}
			transfer := "unavailable after cancellation"
			if result.Remote != nil {
				transfer = result.Remote.Status
			}
			t.Logf("%s: status=%s server=%s transfer=%s planned=%d attempted=%d receipts=%d", name, result.Status, result.ServerStatus, transfer, result.Local.Summary.Planned, result.Local.Summary.Attempted, result.Local.Summary.OnTime)
		}
	}
}

// task23Command captures stdout separately from diagnostics under the outer fixture deadline.
func task23Command(t *testing.T, ctx context.Context, exe string, args ...string) []byte {
	t.Helper()
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s: %v: %s", filepath.Base(exe), err, stderr.Bytes())
	}
	return b
}

// task23Save preserves reviewable non-secret outputs in the ignored fixture directory.
func task23Save(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}

// task23JSON writes explicit typed plans/policies without shell quoting or precision conversion.
func task23JSON(t *testing.T, path string, value any) {
	t.Helper()
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	task23Save(t, path, b)
}

// task23Console supplies a hidden console only when absent, so targeted CTRL_BREAK
// reaches the new client process group as os.Interrupt without signaling the test or other apps.
func task23Console(t *testing.T) {
	t.Helper()
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	window, _, _ := kernel.NewProc("GetConsoleWindow").Call()
	if window != 0 {
		return
	}
	if ok, _, err := kernel.NewProc("AllocConsole").Call(); ok == 0 {
		// A headless pseudoconsole can be attached even when GetConsoleWindow returns zero.
		if err == windows.ERROR_ACCESS_DENIED {
			return
		}
		t.Fatalf("allocate hidden fixture console: %v", err)
	}
	window, _, _ = kernel.NewProc("GetConsoleWindow").Call()
	windows.NewLazySystemDLL("user32.dll").NewProc("ShowWindow").Call(window, 0)
	t.Cleanup(func() { // Free only the console allocated by this fixture after all client groups exit.
		kernel.NewProc("FreeConsole").Call()
	})
}

// task23Run launches the product CLI; cancellation waits for raw data attempts,
// then uses a real console interrupt and requires a bounded nonzero partial exit.
func task23Run(t *testing.T, ctx context.Context, exe, data, plan, root, name string, interrupted bool) control.RunResult {
	t.Helper()
	before, _ := filepath.Glob(filepath.Join(data, "reports", "*-client"))
	cmd := exec.CommandContext(ctx, exe, "client-run", "-data-dir", data, "-profile", plan)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP, HideWindow: true}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { // Wait once; channel synchronization protects the completed output buffers.
		done <- cmd.Wait()
	}()
	joined := false
	defer func() { // Kill only this owned client if an assertion prevents normal joined cleanup.
		if !joined {
			cmd.Process.Kill()
			<-done
		}
	}()
	if interrupted {
		deadline := time.Now().Add(10 * time.Second)
		for {
			paths, _ := filepath.Glob(filepath.Join(data, "reports", "*-client"))
			observed := false
			for _, path := range paths {
				if !task23Contains(before, path) {
					b, _ := os.ReadFile(filepath.Join(path, "events.jsonl"))
					observed = bytes.Contains(b, []byte(`"event_type":"attempt"`))
				}
			}
			if observed {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("no persisted data attempt before interrupt")
			}
			time.Sleep(20 * time.Millisecond)
		}
		if err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(cmd.Process.Pid)); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case err := <-done:
		joined = true
		if interrupted != (err != nil) {
			t.Fatalf("unexpected %s exit: %v: %s", name, err, stderr.Bytes())
		}
	case <-time.After(20 * time.Second):
		t.Fatal("CLI exceeded bounded completion/drain")
	}
	task23Save(t, filepath.Join(root, name+"-run.json"), stdout.Bytes())
	task23Save(t, filepath.Join(root, name+"-stderr.txt"), stderr.Bytes())
	var result control.RunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Local == nil || !result.Readiness.MeasurementStarted || result.Local.Complete == interrupted || result.Local.Summary.Attempted == 0 {
		t.Fatalf("%s missing finite/transfer evidence", name)
	}
	if interrupted && (result.Status != "partial" || result.ServerStatus != "unknown" || result.Remote != nil || result.Reconciliation != nil || !strings.Contains(result.Reason, "canceled")) || !interrupted && (result.Status != "completed" || result.ServerStatus != "completed" || result.Remote == nil || result.Remote.Status != "verified" || result.Reconciliation == nil || result.Reconciliation.Status != "reconciled") {
		t.Fatalf("%s lifecycle/reconciliation mismatch", name)
	}
	return result
}

// task23Contains identifies preexisting fixture report paths without touching user reports.
func task23Contains(paths []string, target string) bool {
	for _, path := range paths {
		if path == target {
			return true
		}
	}
	return false
}

// task23Reports replays and exports through fresh CLI processes, preserving exact
// saved summaries, local/remote platform identities and partial delivery limits.
func task23Reports(t *testing.T, ctx context.Context, exe, root, name string, result control.RunResult, interrupted bool) {
	t.Helper()
	args := []string{"-report", result.ReportPath, "-max-bytes", "33554432"}
	b := task23Command(t, ctx, exe, append([]string{"report-replay"}, args...)...)
	task23Save(t, filepath.Join(root, name+"-replay.json"), b)
	var replay control.ReplayedReport
	if err := json.Unmarshal(b, &replay); err != nil {
		t.Fatal(err)
	}
	if replay.Verification != "matched" || replay.Status != result.Status || replay.Manifest.OS != "windows" || replay.Manifest.Arch != "amd64" || !reflect.DeepEqual(replay.Summary.Local, result.Local) {
		t.Fatalf("%s replay/platform/summary mismatch", name)
	}
	if interrupted {
		if replay.ServerReport != nil || replay.Reconciliation != nil || replay.Summary.Local.Summary.Received != nil {
			t.Fatal("partial run incorrectly acquired remote delivery evidence")
		}
	} else if replay.ServerReport == nil || replay.ServerReport.Manifest.OS != "linux" || replay.ServerReport.Verification != "matched" || replay.Reconciliation == nil || replay.Reconciliation.Status != "reconciled" {
		t.Fatal("complete run missing compatible remote evidence")
	}
	for _, format := range []string{"json", "jsonl", "csv"} {
		b := task23Command(t, ctx, exe, append(append([]string{"report-export"}, args...), "-format", format)...)
		task23Save(t, filepath.Join(root, name+"."+format), b)
		switch format {
		case "json":
			var exported control.ReplayedReport
			if err := json.Unmarshal(b, &exported); err != nil || !reflect.DeepEqual(exported.Summary, replay.Summary) {
				t.Fatalf("JSON exact summary: %v", err)
			}
		case "jsonl":
			for _, line := range bytes.Split(bytes.TrimSpace(b), []byte{'\n'}) {
				if !json.Valid(line) {
					t.Fatal("invalid JSONL export record")
				}
			}
		case "csv":
			if records, err := csv.NewReader(bytes.NewReader(b)).ReadAll(); err != nil || len(records) < 2 {
				t.Fatalf("CSV export: %v", err)
			}
		}
	}
	// Hash the persisted original raw evidence for the retained environment/result index.
	raw, err := os.ReadFile(filepath.Join(result.ReportPath, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	task23Save(t, filepath.Join(root, name+"-raw-sha256.txt"), []byte(fmt.Sprintf("%x\n", sha256.Sum256(raw))))
}
