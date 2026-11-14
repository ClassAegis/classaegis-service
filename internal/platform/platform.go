// SPDX-License-Identifier: AGPL-3.0-only
package platform

// Platform abstracts OS-specific device control so the command pipeline stays
// platform-agnostic. Windows implementation lives in platform/windows; a Linux
// adapter (UOS/Kylin via D-Bus) plugs in here in a later milestone.
type Platform interface {
	// Name returns the adapter name, e.g. "windows".
	Name() string

	// Lock locks the workstation (user32!LockWorkStation on Windows).
	Lock() error

	// Unlock dismisses overlays/notifications created by this agent.
	// Programmatically dismissing the OS secure desktop is not possible;
	// this is a deliberate security boundary, see design-m0.md section 5.
	Unlock() error

	// Notify shows a transient message to the user (Toast on Windows).
	Notify(title string, body string, ttlSeconds int) error

	// SetVolume sets master output volume, percent 0-100.
	SetVolume(percent int) error
	SetAudioVolume(flow string, percent int) error
	SetAudioMute(flow string, mute bool) error
	SetDisplayResolution(device string, width, height, refresh int) error
	LaunchApplication(app string) error
	RecordVoiceMessage(durationSeconds int) (string, int64, error)

	// Reboot restarts the machine.
	Reboot() error
}
