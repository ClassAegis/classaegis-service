// SPDX-License-Identifier: AGPL-3.0-only
package ability

import "testing"

type desktopPlatformStub struct{ recordCalls int }

func (p *desktopPlatformStub) Name() string { return "test" }
func (p *desktopPlatformStub) Lock() error { return nil }
func (p *desktopPlatformStub) Unlock() error { return nil }
func (p *desktopPlatformStub) Notify(string, string, int) error { return nil }
func (p *desktopPlatformStub) SetVolume(int) error { return nil }
func (p *desktopPlatformStub) SetAudioVolume(string, int) error { return nil }
func (p *desktopPlatformStub) SetAudioMute(string, bool) error { return nil }
func (p *desktopPlatformStub) SetDisplayResolution(string, int, int, int) error { return nil }
func (p *desktopPlatformStub) LaunchApplication(string) error { return nil }
func (p *desktopPlatformStub) RecordVoiceMessage(int) (string, int64, error) {
	p.recordCalls++
	return "voice.wav", 128, nil
}
func (p *desktopPlatformStub) Reboot() error { return nil }

func TestVoiceMessageRequiresDeclaredConsent(t *testing.T) {
	p := &desktopPlatformStub{}
	d := NewDesktop(p)
	ok, code, _, _ := d.Handle("m1", "AUDIO_VOICE_MESSAGE", map[string]any{"durationSeconds": 5}, nil)
	if ok || code != "E_USER_CONSENT_REQUIRED" || p.recordCalls != 0 {
		t.Fatalf("recording was not blocked without consent: ok=%v code=%s calls=%d", ok, code, p.recordCalls)
	}
}

func TestVoiceMessageDurationIsBounded(t *testing.T) {
	p := &desktopPlatformStub{}
	d := NewDesktop(p)
	for _, duration := range []int{0, 61} {
		ok, code, _, _ := d.Handle("m1", "AUDIO_VOICE_MESSAGE", map[string]any{"durationSeconds": duration, "consent": true}, nil)
		if ok || code != "E_PARAM" || p.recordCalls != 0 {
			t.Fatalf("invalid duration %d was not rejected: ok=%v code=%s calls=%d", duration, ok, code, p.recordCalls)
		}
	}
}
