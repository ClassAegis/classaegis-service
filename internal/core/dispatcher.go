package core

import (
	"encoding/json"
	"log"
	"strconv"
)

// Executor performs one verified command. Implemented by the loopback IPC
// executor (core -> ability) and by the all-in-one debug executor (in-process).
type Executor interface {
	// Execute runs the command; returns ok=true on success, or code/message
	// describing the failure (E_PLATFORM_UNSUPPORTED, E_FAILED, ...).
	// result carries command-specific data for the receipt result field.
	Execute(msgId, msgType string, payload map[string]any) (ok bool, code string, message string, result map[string]any)
}

// Dispatcher runs the command pipeline:
// parse -> msgId dedup -> HMAC verify -> EXECUTING receipt -> execute -> final receipt.
type Dispatcher struct {
	executor    Executor
	seen        *SeenSet
	secretFn    func() string
	sendReceipt func(string)
}

func NewDispatcher(
	executor Executor,
	seen *SeenSet,
	secretFn func() string,
	sendReceipt func(string),
) *Dispatcher {
	return &Dispatcher{executor: executor, seen: seen, secretFn: secretFn, sendReceipt: sendReceipt}
}

// ExecutorFunc adapts a plain function to the Executor interface
// (used by the all-in-one debug mode).
type ExecutorFunc func(msgId, msgType string, payload map[string]any) (ok bool, code string, message string, result map[string]any)

func (f ExecutorFunc) Execute(msgId, msgType string, payload map[string]any) (bool, string, string, map[string]any) {
	return f(msgId, msgType, payload)
}

// OnCommand processes one raw cmd/down JSON message.
func (d *Dispatcher) OnCommand(raw string) {
	msg, err := ParseLoose(raw)
	if err != nil {
		log.Printf("[dispatcher] parse failed: %v", err)
		return
	}
	msgId := str(msg, "msgId")
	msgType := str(msg, "type")
	if msgId == "" || msgType == "" {
		log.Printf("[dispatcher] missing msgId or type")
		return
	}
	ts := num(msg, "ts")
	if ts == 0 {
		ts = NowMillis()
	}

	if !d.seen.Add(msgId) {
		log.Printf("[dispatcher] duplicate msgId skipped: %s", msgId)
		return
	}

	rawPayload := mustJSON(msg["payload"])
	canonical, err := CanonicalForCommand(msgId, msgType, rawPayload, ts)
	if err != nil {
		log.Printf("[dispatcher] canonical failed: %v", err)
		d.seen.Remove(msgId)
		return
	}
	secret := d.secretFn()
	if secret != "" && !Verify(secret, canonical, str(msg, "sig")) {
		log.Printf("[dispatcher] bad signature, msgId=%s", msgId)
		d.receipt(msgId, "FAILED", nil, "E_BAD_SIG", "signature mismatch")
		d.seen.Remove(msgId)
		return
	}

	d.receipt(msgId, "EXECUTING", nil, "", "")

	payload, _ := msg["payload"].(map[string]any)
	ok, code, message, result := d.executor.Execute(msgId, msgType, payload)
	if ok {
		d.receipt(msgId, "SUCCESS", result, "", "")
		return
	}
	d.seen.Remove(msgId)
	d.receipt(msgId, "FAILED", nil, code, message)
}

// receipt builds and sends a cmd/up receipt.
func (d *Dispatcher) receipt(msgId, status string, result map[string]any, code, message string) {
	jsonStr, err := BuildReceipt(msgId, status, result, code, message)
	if err != nil {
		log.Printf("[dispatcher] receipt marshal failed: %v", err)
		return
	}
	d.sendReceipt(jsonStr)
}

// payload field helpers (UseNumber-decoded values).

func str(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func num(m map[string]any, key string) int64 {
	return getInt(m, key, 0)
}

func getInt(m map[string]any, key string, def int64) int64 {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case json.Number:
			if i, err := n.Int64(); err == nil {
				return i
			}
		case float64:
			return int64(n)
		case string:
			if i, err := strconv.ParseInt(n, 10, 64); err == nil {
				return i
			}
		}
	}
	return def
}

func mustJSON(v any) string {
	s, err := MarshalCanonical(v)
	if err != nil {
		return "{}"
	}
	return s
}
