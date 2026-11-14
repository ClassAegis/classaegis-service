// SPDX-License-Identifier: AGPL-3.0-only
//go:build windows

// Package windows implements the platform adapter for Windows 10/11.
// M0 runs in console mode inside the user session, so LockWorkStation,
// Toast notifications and Core Audio are directly usable. Windows-service
// mode (Session 0) is planned for M0-late / M1, see design-m0.md section 5.
package windows

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
	"unsafe"

	ole "github.com/go-ole/go-ole"
	"github.com/go-toast/toast"
	"github.com/moutend/go-wca/pkg/wca"
	"golang.org/x/sys/windows"
)

type Windows struct{}

func New() *Windows { return &Windows{} }

func (w *Windows) Name() string { return "windows" }

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	procLockWorkStation = user32.NewProc("LockWorkStation")
	user32ChangeDisplaySettingsExW = user32.NewProc("ChangeDisplaySettingsExW")
	user32EnumDisplaySettingsW = user32.NewProc("EnumDisplaySettingsW")
	procMessageBoxW = user32.NewProc("MessageBoxW")
)

const enumCurrentSettings = 0xffffffff
const cdsFullscreen = 0x00000004
const dmPelsWidth = 0x00080000
const dmPelsHeight = 0x00100000
const dmDisplayFrequency = 0x00400000

type devMode struct {
	DeviceName [32]uint16
	SpecVersion, DriverVersion, Size, DriverExtra uint16
	Fields uint32
	PositionX, PositionY int32
	DisplayOrientation, DisplayFixedOutput uint32
	Color uint16
	Duplex, YResolution, TTOption, Collate uint16
	FormName [32]uint16
	LogPixels uint16
	BitsPerPel uint32
	PelsWidth, PelsHeight uint32
	DisplayFlags, DisplayFrequency uint32
	ICMMethod, ICMIntent, MediaType, DitherType, Reserved1, Reserved2 uint32
	DriverExtraData [32]byte
}

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
		AppID:    "ClassAegis.Agent",
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
	return w.SetAudioVolume("render", percent)
}

func (w *Windows) SetAudioVolume(flow string, percent int) error {
	dataFlow := uint32(wca.ERender)
	if flow == "capture" {
		dataFlow = wca.ECapture
	} else if flow != "render" {
		return fmt.Errorf("unsupported audio flow %q", flow)
	}
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
	if err := enumerator.GetDefaultAudioEndpoint(dataFlow, wca.EConsole, &device); err != nil {
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

func (w *Windows) SetAudioMute(flow string, mute bool) error {
	dataFlow := uint32(wca.ERender)
	if flow == "capture" {
		dataFlow = wca.ECapture
	} else if flow != "render" {
		return fmt.Errorf("unsupported audio flow %q", flow)
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
	if err := enumerator.GetDefaultAudioEndpoint(dataFlow, wca.EConsole, &device); err != nil {
		return fmt.Errorf("GetDefaultAudioEndpoint failed: %w", err)
	}
	defer device.Release()
	var endpoint *wca.IAudioEndpointVolume
	if err := device.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, unsafe.Pointer(nil), &endpoint); err != nil {
		return fmt.Errorf("IAudioEndpointVolume activate failed: %w", err)
	}
	defer endpoint.Release()
	if err := endpoint.SetMute(mute, nil); err != nil {
		return fmt.Errorf("SetMute failed: %w", err)
	}
	return nil
}

func (w *Windows) SetDisplayResolution(device string, width, height, refresh int) error {
	if width < 640 || width > 7680 || height < 480 || height > 4320 || refresh < 0 || refresh > 360 { return fmt.Errorf("invalid display mode") }
	var name uintptr
	if device != "" { p, err := windows.UTF16PtrFromString(device); if err != nil { return err }; name = uintptr(unsafe.Pointer(p)) }
	var dm devMode
	dm.Size = uint16(unsafe.Sizeof(dm))
	if r, _, err := user32EnumDisplaySettingsW.Call(name, enumCurrentSettings, uintptr(unsafe.Pointer(&dm))); r == 0 { return fmt.Errorf("EnumDisplaySettings failed: %w", err) }
	dm.PelsWidth, dm.PelsHeight = uint32(width), uint32(height)
	dm.Fields = dmPelsWidth | dmPelsHeight
	if refresh > 0 { dm.DisplayFrequency = uint32(refresh); dm.Fields |= dmDisplayFrequency }
	r, _, err := user32ChangeDisplaySettingsExW.Call(name, uintptr(unsafe.Pointer(&dm)), 0, cdsFullscreen, 0)
	if r != 0 { return fmt.Errorf("ChangeDisplaySettingsEx failed: %d: %w", r, err) }
	return nil
}

func (w *Windows) LaunchApplication(app string) error {
	paths := map[string]string{"notepad": "notepad.exe", "calculator": "calc.exe", "paint": "mspaint.exe", "explorer": "explorer.exe"}
	path, ok := paths[app]
	if !ok { return fmt.Errorf("application is not allowlisted: %s", app) }
	if err := exec.Command(path).Start(); err != nil { return fmt.Errorf("launch %s failed: %w", app, err) }
	return nil
}

func (w *Windows) RecordVoiceMessage(durationSeconds int) (string, int64, error) {
	if durationSeconds < 1 || durationSeconds > 60 {
		return "", 0, fmt.Errorf("durationSeconds must be 1..60")
	}
	text, _ := windows.UTF16PtrFromString(fmt.Sprintf("ClassAegis will record from the default microphone for up to %d seconds. The WAV file stays on this device. Start recording?", durationSeconds))
	title, _ := windows.UTF16PtrFromString("ClassAegis microphone recording")
	result, _, callErr := procMessageBoxW.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x00000004|0x00000030|0x00040000)
	if result != 6 {
		return "", 0, fmt.Errorf("E_USER_CONSENT_REQUIRED: user declined or consent dialog unavailable: %w", callErr)
	}
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		return "", 0, fmt.Errorf("CoInitializeEx failed: %w", err)
	}
	defer ole.CoUninitialize()
	var enumerator *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &enumerator); err != nil {
		return "", 0, err
	}
	defer enumerator.Release()
	var device *wca.IMMDevice
	if err := enumerator.GetDefaultAudioEndpoint(wca.ECapture, wca.EConsole, &device); err != nil {
		return "", 0, err
	}
	defer device.Release()
	var client *wca.IAudioClient
	if err := device.Activate(wca.IID_IAudioClient, wca.CLSCTX_ALL, unsafe.Pointer(nil), &client); err != nil {
		return "", 0, err
	}
	defer client.Release()
	var format *wca.WAVEFORMATEX
	if err := client.GetMixFormat(&format); err != nil {
		return "", 0, err
	}
	defer ole.CoTaskMemFree(uintptr(unsafe.Pointer(format)))
	if format.NBlockAlign == 0 || format.NChannels == 0 || format.NSamplesPerSec == 0 {
		return "", 0, fmt.Errorf("unsupported microphone format")
	}
	if err := client.Initialize(wca.AUDCLNT_SHAREMODE_SHARED, 0, 10000000, 0, format, nil); err != nil {
		return "", 0, fmt.Errorf("audio client initialize failed: %w", err)
	}
	var capture *wca.IAudioCaptureClient
	if err := client.GetService(wca.IID_IAudioCaptureClient, &capture); err != nil {
		return "", 0, err
	}
	defer capture.Release()
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "ClassAegis", "audio")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", 0, err
	}
	path := filepath.Join(dir, fmt.Sprintf("voice-%s.wav", time.Now().Format("20060102T150405.000000000")))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return "", 0, err
	}
	keep := false
	defer func() {
		_ = f.Close()
		if !keep {
			_ = os.Remove(path)
		}
	}()
	formatSize := 18 + int(format.CbSize)
	if formatSize > 4096 {
		return "", 0, fmt.Errorf("unsupported microphone format extension")
	}
	wavHeader := make([]byte, 20+formatSize+8)
	copy(wavHeader[:4], "RIFF")
	copy(wavHeader[8:12], "WAVE")
	copy(wavHeader[12:16], "fmt ")
	binary.LittleEndian.PutUint32(wavHeader[16:20], uint32(formatSize))
	copy(wavHeader[20:20+formatSize], unsafe.Slice((*byte)(unsafe.Pointer(format)), formatSize))
	dataHeader := 20 + formatSize
	copy(wavHeader[dataHeader:dataHeader+4], "data")
	if _, err := f.Write(wavHeader); err != nil {
		return "", 0, err
	}
	if err := client.Start(); err != nil {
		return "", 0, err
	}
	deadline := time.Now().Add(time.Duration(durationSeconds) * time.Second)
	maxBytes := format.NAvgBytesPerSec*uint32(durationSeconds+1)
	var dataBytes uint32
	for time.Now().Before(deadline) {
		var frames uint32
		if err := capture.GetNextPacketSize(&frames); err != nil {
			_ = client.Stop()
			return "", 0, err
		}
		if frames == 0 {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		var samples *byte
		var flags uint32
		if err := capture.GetBuffer(&samples, &frames, &flags, nil, nil); err != nil {
			_ = client.Stop()
			return "", 0, err
		}
		chunkBytes := int(frames) * int(format.NBlockAlign)
		if dataBytes+uint32(chunkBytes) > maxBytes {
			_ = capture.ReleaseBuffer(frames)
			_ = client.Stop()
			return "", 0, fmt.Errorf("recording exceeded size limit")
		}
		if flags&wca.AUDCLNT_BUFFERFLAGS_SILENT != 0 {
			_, err = f.Write(make([]byte, chunkBytes))
		} else {
			_, err = f.Write(unsafe.Slice(samples, chunkBytes))
		}
		releaseErr := capture.ReleaseBuffer(frames)
		if err != nil {
			_ = client.Stop()
			return "", 0, err
		}
		if releaseErr != nil {
			_ = client.Stop()
			return "", 0, releaseErr
		}
		dataBytes += uint32(chunkBytes)
	}
	if err := client.Stop(); err != nil {
		return "", 0, err
	}
	if dataBytes == 0 {
		return "", 0, fmt.Errorf("microphone returned no audio samples")
	}
	if _, err := f.Seek(4, 0); err != nil {
		return "", 0, err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(dataHeader)+dataBytes); err != nil {
		return "", 0, err
	}
	if _, err := f.Seek(int64(dataHeader+4), 0); err != nil {
		return "", 0, err
	}
	if err := binary.Write(f, binary.LittleEndian, dataBytes); err != nil {
		return "", 0, err
	}
	if err := f.Sync(); err != nil {
		return "", 0, err
	}
	keep = true
	return path, int64(dataBytes), nil
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
