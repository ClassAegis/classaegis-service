//go:build windows

// Package windows implements the platform adapter for Windows 10/11.
// M0 runs in console mode inside the user session, so LockWorkStation,
// Toast notifications and Core Audio are directly usable. Windows-service
// mode (Session 0) is planned for M0-late / M1, see design-m0.md section 5.
package windows

import (
	"fmt"
	"os/exec"
	"unsafe"

	"github.com/go-toast/toast"
	"github.com/moutend/go-wca/pkg/wca"
	"golang.org/x/sys/windows"
	ole "github.com/go-ole/go-ole"
)

type Windows struct{}

func New() *Windows { return &Windows{} }

func (w *Windows) Name() string { return "windows" }

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	procLockWorkStation = user32.NewProc("LockWorkStation")
)

// Lock locks the workstation via user32!LockWorkStation.
func (w *Windows) Lock() error {
	r1, _, lastErr := procLockWorkStation.Call()
	if r1 == 0 {
		return fmt.Errorf("LockWorkStation failed: %w", lastErr)
	}
	return nil
}

// Unlock is a no-op in M0: notifications auto-expire and the OS lock screen
// cannot be dismissed programmatically (deliberate security boundary).
func (w *Windows) Unlock() error {
	return nil
}

// Notify shows a Windows 10 Toast notification via go-toast (PowerShell bridge).
func (w *Windows) Notify(title string, body string, ttlSeconds int) error {
	n := toast.Notification{
		AppID:    "OneClassMesh.Agent",
		Title:    title,
		Message:  body,
		Duration: toast.Short,
	}
	if ttlSeconds > 25 {
		n.Duration = toast.Long
	}
	if err := n.Push(); err != nil {
		return fmt.Errorf("toast push failed: %w", err)
	}
	return nil
}

// SetVolume sets the master endpoint volume via Core Audio (go-wca).
func (w *Windows) SetVolume(percent int) error {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		return fmt.Errorf("CoInitializeEx failed: %w", err)
	}
	defer ole.CoUninitialize()

	var enumerator *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &enumerator); err != nil {
		return fmt.Errorf("CoCreateInstance failed: %w", err)
	}
	defer enumerator.Release()

	var device *wca.IMMDevice
	if err := enumerator.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &device); err != nil {
		return fmt.Errorf("GetDefaultAudioEndpoint failed: %w", err)
	}
	defer device.Release()

	var endpoint *wca.IAudioEndpointVolume
	if err := device.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, unsafe.Pointer(nil), &endpoint); err != nil {
		return fmt.Errorf("IAudioEndpointVolume activate failed: %w", err)
	}
	defer endpoint.Release()

	if err := endpoint.SetMasterVolumeLevelScalar(float32(percent)/100.0, nil); err != nil {
		return fmt.Errorf("SetMasterVolumeLevelScalar failed: %w", err)
	}
	return nil
}

// Reboot restarts the machine. Console mode shells out to shutdown.exe;
// service mode (SYSTEM) will use InitiateSystemShutdownEx directly in M1.
func (w *Windows) Reboot() error {
	cmd := exec.Command("shutdown", "/r", "/t", "0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("shutdown failed: %v: %s", err, string(out))
	}
	return nil
}
