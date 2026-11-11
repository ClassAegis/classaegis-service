package ability

import "testing"

func TestValidateStreamURL(t *testing.T) {
	valid := []string{"rtmp://example.test/live/a", "rtp://239.0.0.1:5004"}
	for _, raw := range valid {
		if err := validateStreamURL(raw); err != nil {
			t.Errorf("valid URL rejected: %s: %v", raw, err)
		}
	}
	invalid := []string{"", "https://example.test/a", "rtmp://user:pass@example.test/a", "rtp://example.test/a#x", "rtmp://"}
	for _, raw := range invalid {
		if err := validateStreamURL(raw); err == nil {
			t.Errorf("invalid URL accepted: %s", raw)
		}
	}
}
