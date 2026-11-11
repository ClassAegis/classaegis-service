//go:build windows

package popup

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WindowInfo is one top-level window snapshot for rule matching.
type WindowInfo struct {
	HWND    uintptr
	PID     uint32
	Process string // executable base name
	Title   string
	Class   string
}

// WindowSource abstracts window enumeration and actions (test seam).
type WindowSource interface {
	// List returns currently visible top-level windows.
	List() ([]WindowInfo, error)
	// Close asks the window to close gracefully (WM_CLOSE).
	Close(hwnd uintptr) error
	// Hide hides the window (ShowWindow SW_HIDE).
	Hide(hwnd uintptr) error
}

// windowsSource implements WindowSource via user32.
type windowsSource struct{}

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procEnumWindows         = user32.NewProc("EnumWindows")
	procIsWindowVisible     = user32.NewProc("IsWindowVisible")
	procGetWindowTextW      = user32.NewProc("GetWindowTextW")
	procGetClassNameW       = user32.NewProc("GetClassNameW")
	procGetWindowThreadProc = user32.NewProc("GetWindowThreadProcessId")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procQueryFullImage      = kernel32.NewProc("QueryFullProcessImageNameW")
)

// List enumerates visible top-level windows and resolves owning processes.
func (s *windowsSource) List() ([]WindowInfo, error) {
	var out []WindowInfo
	cb := syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		vis, _, _ := procIsWindowVisible.Call(hwnd)
		if vis == 0 {
			return 1
		}
		wi := WindowInfo{HWND: hwnd}
		wi.Title = windowText(hwnd)
		wi.Class = className(hwnd)
		var pid uint32
		procGetWindowThreadProc.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		wi.PID = pid
		wi.Process = processName(pid)
		out = append(out, wi)
		return 1
	})
	procEnumWindows.Call(cb, 0)
	return out, nil
}

// windowText reads the window title.
func windowText(hwnd uintptr) string {
	var buf [512]uint16
	n, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return strings.TrimSpace(windows.UTF16ToString(buf[:n]))
}

// className reads the window class name.
func className(hwnd uintptr) string {
	var buf [256]uint16
	n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf[:n])
}

// processName resolves the executable base name of the window owner.
// Unresolvable owners return "" and are never acted on.
func processName(pid uint32) string {
	if pid == 0 {
		return ""
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	var buf [1024]uint16
	size := uint32(len(buf))
	r1, _, _ := procQueryFullImage.Call(uintptr(h), 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r1 == 0 {
		return ""
	}
	full := windows.UTF16ToString(buf[:size])
	if i := strings.LastIndexByte(full, '\\'); i >= 0 {
		return full[i+1:]
	}
	return full
}

// Close asks the window to close gracefully (WM_CLOSE, fire-and-forget).
func (s *windowsSource) Close(hwnd uintptr) error {
	r1, _, _ := procPostMessageW.Call(hwnd, 0x0010, 0, 0)
	if r1 == 0 {
		return fmt.Errorf("PostMessageW(WM_CLOSE) failed")
	}
	return nil
}

// Hide hides the window (ShowWindow SW_HIDE).
func (s *windowsSource) Hide(hwnd uintptr) error {
	r1, _, _ := procShowWindow.Call(hwnd, 0 /* SW_HIDE */)
	if r1 == 0 {
		return fmt.Errorf("ShowWindow(SW_HIDE) failed")
	}
	return nil
}
