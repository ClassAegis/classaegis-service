package core

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// Protocol v0.1 helpers shared by agent and test tooling.
// Canonical string: msgId \n type \n payloadJson(sorted keys, compact) \n ts
//
// Go's encoding/json marshals map[string]any with sorted keys and compact
// separators, which matches the canonical rule. Decode with UseNumber so
// integer values survive the re-marshal byte-identically.
func MarshalCanonical(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

// ParseLoose decodes JSON preserving number literals (UseNumber).
func ParseLoose(raw string) (map[string]any, error) {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// Sign computes HMAC-SHA256 hex of the canonical string.
func Sign(secret, canonical string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks a hex signature in constant time.
func Verify(secret, canonical, expectedHex string) bool {
	computed := Sign(secret, canonical)
	return hmac.Equal([]byte(computed), []byte(expectedHex))
}

// CanonicalForCommand builds the signing string for one command message.
// rawPayload is the original payload JSON text from the message; it is
// re-marshaled to sorted-compact form before signing.
func CanonicalForCommand(msgId, msgType, rawPayload string, ts int64) (string, error) {
	payload, err := ParseLoose(rawPayload)
	if err != nil {
		return "", err
	}
	payloadStr, err := MarshalCanonical(payload)
	if err != nil {
		return "", err
	}
	return msgId + "\n" + msgType + "\n" + payloadStr + "\n" + strconv.FormatInt(ts, 10), nil
}

// NowMillis returns current Unix epoch milliseconds.
func NowMillis() int64 {
	return time.Now().UnixMilli()
}

// BuildReceipt assembles a cmd/up receipt as compact JSON.
func BuildReceipt(msgId, status string, result map[string]any, errCode, errMsg string) (string, error) {
	if result == nil {
		result = map[string]any{}
	}
	m := map[string]any{
		"msgId":  msgId,
		"status": status,
		"result": result,
		"error":  nil,
		"ts":     NowMillis(),
	}
	if errCode != "" {
		m["error"] = map[string]string{"code": errCode, "message": errMsg}
	}
	return MarshalCanonical(m)
}
