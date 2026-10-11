package control

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// ExportReport emits a validated snapshot without opening sockets or modifying source files.
// JSONL is a typed manifest/summary/event bundle; CSV uses stable long-form columns with exact text values.
func ExportReport(out io.Writer, r *ReplayedReport, format string) error {
	if r == nil {
		return errors.New("nil replayed report")
	}
	switch format {
	case "json":
		e := json.NewEncoder(out)
		e.SetIndent("", "  ")
		return e.Encode(r)
	case "jsonl":
		e := json.NewEncoder(out)
		if err := e.Encode(struct {
			Type   string          `json:"record_type"`
			Report *ReplayedReport `json:"report"`
		}{"local_report", r}); err != nil {
			return err
		}
		for _, raw := range r.Events {
			if err := e.Encode(struct {
				Type  string          `json:"record_type"`
				Event json.RawMessage `json:"event"`
			}{"raw_event", raw}); err != nil {
				return err
			}
		}
		return nil
	case "csv":
		w := csv.NewWriter(out)
		if err := w.Write([]string{"schema_version", "run_id", "session_id", "endpoint", "report_status", "record_type", "event_id", "field", "unit", "value", "value_status", "reason"}); err != nil {
			return err
		}
		b, err := json.Marshal(r)
		if err != nil {
			return err
		}
		if err = exportCSVObject(w, r, "local_report", "", b); err != nil {
			return err
		}
		for i, raw := range r.Events {
			if err = exportCSVObject(w, r, "raw_event", strconv.Itoa(i+1), raw); err != nil {
				return err
			}
		}
		w.Flush()
		return w.Error()
	default:
		return errors.New("format must be json, jsonl or csv")
	}
}

// exportCSVObject decodes with UseNumber so metadata integers and decimal strings never pass through float64.
func exportCSVObject(w *csv.Writer, r *ReplayedReport, kind, id string, b []byte) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return err
	}
	return exportCSVValue(w, r, kind, id, "", v)
}

// exportCSVValue flattens stable JSON paths, preserving empty collections, nulls and phase/sample indexes.
// Missing receiver/packet-loss evidence is explicitly unknown; absent eligible duration samples are unavailable.
func exportCSVValue(w *csv.Writer, r *ReplayedReport, kind, id, path string, v any) error {
	value, status, reason := "", "known", ""
	switch item := v.(type) {
	case map[string]any:
		if len(item) == 0 {
			value = "{}"
			break
		}
		keys := make([]string, 0, len(item))
		for key := range item {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child := key
			if path != "" {
				child = path + "." + key
			}
			if err := exportCSVValue(w, r, kind, id, child, item[key]); err != nil {
				return err
			}
		}
		return nil
	case []any:
		if len(item) == 0 {
			value = "[]"
			break
		}
		for i, child := range item {
			if err := exportCSVValue(w, r, kind, id, fmt.Sprintf("%s[%d]", path, i), child); err != nil {
				return err
			}
		}
		return nil
	case nil:
		status, reason = "unavailable", "no eligible observation/sample"
		if strings.HasSuffix(path, "Received") || strings.HasSuffix(path, "MissingAtReceiver") || strings.HasSuffix(path, "TransportPacketLoss") {
			status, reason = "unknown", "no complete compatible remote evidence; stream delivery does not establish packet loss"
		}
	case string:
		value = item
	case json.Number:
		value = item.String()
	case bool:
		value = strconv.FormatBool(item)
	default:
		return errors.New("unsupported CSV value")
	}
	return w.Write([]string{"1", r.Manifest.RunID, r.Manifest.SessionID, r.Manifest.Endpoint, r.Status, kind, id, path, reportUnit(path), value, status, reason})
}

// reportUnit labels version-1 duration, byte and count fields; identities and settings retain explicit metadata units.
func reportUnit(path string) string {
	parts := strings.Split(path, ".")
	field := parts[len(parts)-1]
	if i := strings.IndexByte(field, '['); i >= 0 {
		field = field[:i]
	}
	if strings.HasSuffix(field, "_ns") {
		return "ns"
	}
	switch field {
	case "At", "Scheduled", "Call", "Attempt", "PeerArrival", "PeerAttempt", "RTTMedian", "RTTP95", "RTTP99", "RTTMin", "RTTMax", "RTTVariation", "ScheduledMedian", "ScheduledP95", "ScheduledP99", "CorrectedP95", "Threshold", "BaselineMedian", "Start", "End", "Duration", "Peak":
		return "ns"
	case "Bytes", "raw_bytes", "discarded_tail_bytes", "queue_bytes", "event_bytes", "file_bytes", "content_bytes":
		return "bytes"
	case "Planned", "Skipped", "Attempted", "Written", "WriteFailed", "OnTime", "Late", "Unresolved", "DuplicateReceipts", "Received", "MissingAtReceiver", "RTTSamples", "VariationGaps", "Accepted", "LocalMisses", "DeadlineMisses", "AcceptedMisses", "Pending", "CorrectedSamples", "InvalidCorrections", "Count", "raw_events", "queue_messages", "receipt_reserve", "outstanding_data", "ledger_messages", "samples", "window", "control_requests":
		return "count"
	case "time_ns":
		return "ns"
	case "sequence", "event_id", "clock_epoch_id", "phase", "Phase", "lifecycle", "Lifecycle":
		return "identity"
	case "rate_numerator":
		return "messages"
	case "rate_denominator":
		return "s"
	case "destination_port", "source_port", "bound_port", "observed_port":
		return "port"
	case "drift_ppb":
		return "ppb"
	default:
		return "metadata"
	}
}
