package main

import (
	"bytes"
	"context"
	"testing"
)

// TestImplementedCommands checks registry discovery and rejects nonexistent endpoint workflows.
func TestImplementedCommands(t *testing.T) {
	var out bytes.Buffer
	if err := run(context.Background(), []string{"adapters"}, &out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"udp"`)) || !bytes.Contains(out.Bytes(), []byte(`"tcp"`)) {
		t.Fatal("missing real adapter descriptors")
	}
	for _, args := range [][]string{nil, {"server"}, {"client"}, {"client-api"}, {"client-api", "-shutdown-timeout", "0s"}, {"validate"}, {"keygen"}, {"preflight"}, {"report-replay"}, {"report-export"}, {"report-export", "-report", "unused", "-max-bytes", "100", "-format", "xml"}} {
		if err := run(context.Background(), args, &out); err == nil {
			t.Fatalf("unsupported/incomplete invocation %v succeeded", args)
		}
	}
}
