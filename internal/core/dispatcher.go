package core

import (
	"encoding/json"
	"log"
	"strconv"

	"github.com/OneClassMesh/oneclassmesh-agent/internal/platform"
)

// Dispatcher runs the command pipeline:
// parse -> msgId dedup -> HMAC verify -> EXECUTING receipt -> execute -> final receipt.
type Dispatcher struct {
	platform    platform.Platform
	seen        *SeenSet
	secretFn    func() string
	sendReceipt func(string)
}

func NewDispatcher(
	p platform.Platform,
	seen *SeenSet,
	secretFn func() string,
	sendReceipt func(string),
) *Dispatcher {
	return &Dispatcher{platform: p, seen: seen, secretFn: secretFn, sendReceipt: sendReceipt}
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
	errPair := d.execute(msgType, payload)
	if errPair != nil {
		d.seen.Remove(msgId)
		d.receipt(msgId, "FAILED", nil, errPair.code, errPair.message)
		return
	}
	d.receipt(msgId, "SUCCESS", nil, "", "")
}

type execError struct {
	code    string
	message string
}

func (d *Dispatcher) execute(msgType string, payload map[string]any) *execError {
	switch msgType {
	case "LOCK_SCREEN":
		if err := d.platform.Lock(); err != nil {
			return &execError{"E_PLATFORM_UNSUPPORTED", err.Error()}
		}
	case "UNLOCK_SCREEN":
		if err := d.platform.Unlock(); err != nil {
			return &execError{"E_FAILED", err.Error()}
		}
	case "SEND_MESSAGE":
		title := str(payload, "title")
		body := str(payload, "body")
		ttl := int(getInt(payload, "ttlSeconds", 10))
		if err := d.platform.Notify(title, body, ttl); err != nil {
			return &execError{"E_PLATFORM_UNSUPPORTED", err.Error()}
		}
	case "SET_VOLUME":
		vol := int(getInt(payload, "volume", 50))
		if err := d.platform.SetVolume(vol); err != nil {
			return &execError{"E_FAILED", err.Error()}
		}
	case "REBOOT":
		if err := d.platform.Reboot(); err != nil {
			return &execError{"E_PLATFORM_UNSUPPORTED", err.Error()}
		}
	default:
		return &execError{"E_UNSUPPORTED_TYPE", "unknown type: " + msgType}
	}
	return nil
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

// --- payload field helpers (UseNumber-decoded values) ---

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
