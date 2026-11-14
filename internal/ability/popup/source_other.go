// SPDX-License-Identifier: AGPL-3.0-only
//go:build !windows

package popup

// WindowInfo is one top-level window snapshot for rule matching.
type WindowInfo struct {
	HWND    uintptr
	PID     uint32
	Process string
	Title   string
	Class   string
}

// WindowSource abstracts window enumeration and actions (test seam).
type WindowSource interface {
	List() ([]WindowInfo, error)
	Close(hwnd uintptr) error
	Hide(hwnd uintptr) error
}

// otherSource is a placeholder until the Linux/UOS/麒麟 platform adapter
// (X11/Wayland window enumeration) is designed.
type otherSource struct{}

func (s *otherSource) List() ([]WindowInfo, error) {
	return nil, nil
}

func (s *otherSource) Close(hwnd uintptr) error {
	return nil
}

func (s *otherSource) Hide(hwnd uintptr) error {
	return nil
}
