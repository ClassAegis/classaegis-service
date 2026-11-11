package media

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Ability exposes read-only device inventory. Capture and recording are kept
// behind the desktop module because they require an interactive consent UI.
type UploadFunc func(msgID, path string, meta map[string]any) (map[string]any, error)
type Ability struct {
	captureJob captureJob
	screenJob  screenJob
	upload     UploadFunc
}

func New() *Ability                            { return &Ability{} }
func NewWithUpload(upload UploadFunc) *Ability { return &Ability{upload: upload} }

func (a *Ability) Name() string { return "media" }

func (a *Ability) Types() []string {
	return []string{"DEVICE_INFO", "AUDIO_LIST_DEVICES", "DISPLAY_LIST", "DISPLAY_EDID", "DISPLAY_ADAPTERS", "CAMERA_LIST", "NET_CAPTURE_START", "NET_CAPTURE_STOP", "NET_CAPTURE_STATUS", "AI_SCREEN_MONITOR_START", "AI_SCREEN_MONITOR_STOP", "AI_SCREEN_MONITOR_STATUS"}
}

func (a *Ability) Handle(msgID string, msgType string, payload map[string]any, prog func(map[string]any)) (bool, string, string, map[string]any) {
	if msgType == "AI_SCREEN_MONITOR_START" || msgType == "AI_SCREEN_MONITOR_STOP" || msgType == "AI_SCREEN_MONITOR_STATUS" {
		return a.screenMonitor(msgID, msgType, payload, prog)
	}
	if msgType == "NET_CAPTURE_START" || msgType == "NET_CAPTURE_STOP" || msgType == "NET_CAPTURE_STATUS" {
		if runtime.GOOS != "windows" {
			return false, "E_PLATFORM_UNSUPPORTED", "network diagnostics require Windows", nil
		}
		return a.capture(msgType, payload)
	}
	if runtime.GOOS != "windows" {
		return false, "E_PLATFORM_UNSUPPORTED", "media inventory requires Windows", nil
	}
	var script string
	switch msgType {
	case "DEVICE_INFO":
		script = `$cpu=Get-CimInstance Win32_Processor | ForEach-Object {[ordered]@{name=$_.Name;cores=$_.NumberOfCores;logicalProcessors=$_.NumberOfLogicalProcessors;maxClockMHz=$_.MaxClockSpeed}}; $os=Get-CimInstance Win32_OperatingSystem; $disks=Get-CimInstance Win32_LogicalDisk -Filter "DriveType=3" | ForEach-Object {[ordered]@{deviceId=$_.DeviceID;size=$_.Size;freeSpace=$_.FreeSpace;filesystem=$_.FileSystem}}; $nets=Get-CimInstance Win32_NetworkAdapter -Filter "PhysicalAdapter=True" | ForEach-Object {[ordered]@{name=$_.Name;mac=$_.MACAddress;status=$_.NetConnectionStatus;speed=$_.Speed;deviceId=$_.PNPDeviceID}}; $memory=Get-CimInstance Win32_ComputerSystem; $items=[ordered]@{computer=$env:COMPUTERNAME;os=[ordered]@{caption=$os.Caption;version=$os.Version;build=$os.BuildNumber;architecture=$os.OSArchitecture};cpu=@($cpu);memoryBytes=$memory.TotalPhysicalMemory;disks=@($disks);networkAdapters=@($nets)}; $items | ConvertTo-Json -Compress -Depth 5`
	case "AUDIO_LIST_DEVICES":
		script = `$items=Get-CimInstance Win32_SoundDevice | ForEach-Object {[ordered]@{name=$_.Name;status=$_.Status;deviceId=$_.PNPDeviceID}}; @{devices=@($items)} | ConvertTo-Json -Compress -Depth 4`
	case "CAMERA_LIST":
		script = `$items=Get-PnpDevice -Class Camera -ErrorAction SilentlyContinue | ForEach-Object {[ordered]@{name=$_.FriendlyName;status=$_.Status;deviceId=$_.InstanceId}}; @{devices=@($items)} | ConvertTo-Json -Compress -Depth 4`
	case "DISPLAY_ADAPTERS":
		script = `$items=Get-CimInstance Win32_VideoController | ForEach-Object {[ordered]@{name=$_.Name;status=$_.Status;adapterRAM=$_.AdapterRAM;driverVersion=$_.DriverVersion;currentWidth=$_.CurrentHorizontalResolution;currentHeight=$_.CurrentVerticalResolution;refreshRate=$_.CurrentRefreshRate}}; @{adapters=@($items)} | ConvertTo-Json -Compress -Depth 4`
	case "DISPLAY_LIST":
		script = `$items=Get-CimInstance -Namespace root\wmi -ClassName WmiMonitorID -ErrorAction SilentlyContinue | ForEach-Object {$n=([Text.Encoding]::ASCII.GetString([byte[]]($_.UserFriendlyName | Where-Object {$_ -ne 0}))); [ordered]@{name=$n;instanceName=$_.InstanceName;active=$_.Active}}; @{displays=@($items)} | ConvertTo-Json -Compress -Depth 4`
	case "DISPLAY_EDID":
		script = `$base='HKLM:\SYSTEM\CurrentControlSet\Enum\DISPLAY'; $items=Get-ChildItem $base -Recurse -ErrorAction SilentlyContinue | Where-Object {$_.PSChildName -eq 'Device Parameters'} | ForEach-Object {$p=Get-ItemProperty $_.PSPath -Name EDID -ErrorAction SilentlyContinue; if($p){[ordered]@{path=$_.PSPath;edid=([Convert]::ToBase64String($p.EDID))}}}; @{edid=@($items)} | ConvertTo-Json -Compress -Depth 4`
	default:
		return false, "E_UNSUPPORTED_TYPE", "unknown media command", nil
	}
	out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return false, "E_PLATFORM_UNSUPPORTED", fmt.Sprintf("Windows inventory failed: %v", err), nil
	}
	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		return false, "E_FAILED", fmt.Sprintf("invalid inventory response: %v", err), nil
	}
	if msgType == "DISPLAY_EDID" {
		items, _ := result["edid"].([]any)
		parsed := make([]any, 0, len(items))
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			encoded, _ := entry["edid"].(string)
			raw, decodeErr := base64.StdEncoding.DecodeString(encoded)
			if decodeErr != nil {
				continue
			}
			delete(entry, "edid")
			for key, value := range parseEDID(raw) {
				entry[key] = value
			}
			parsed = append(parsed, entry)
		}
		result["edid"] = parsed
	}
	return true, "", "", result
}

func parseEDID(raw []byte) map[string]any {
	if len(raw) < 128 {
		return map[string]any{"valid": false}
	}
	vendorCode := uint16(raw[8])<<8 | uint16(raw[9])
	vendor := []byte{byte((vendorCode >> 10) & 31), byte((vendorCode >> 5) & 31), byte(vendorCode & 31)}
	for i := range vendor {
		if vendor[i] >= 1 && vendor[i] <= 26 {
			vendor[i] += 'A' - 1
		}
	}
	name := ""
	for offset := 54; offset+18 <= 126; offset += 18 {
		d := raw[offset : offset+18]
		if d[0] == 0 && d[1] == 0 && d[2] == 0 && d[3] == 0xfc {
			name = strings.TrimSpace(strings.TrimRight(string(d[5:18]), "\x00\r\n "))
			break
		}
	}
	checksum := byte(0)
	for _, b := range raw[:128] {
		checksum += b
	}
	return map[string]any{"valid": true, "manufacturer": string(vendor), "productId": fmt.Sprintf("%04X", uint16(raw[11])<<8|uint16(raw[10])), "serial": uint32(raw[12]) | uint32(raw[13])<<8 | uint32(raw[14])<<16 | uint32(raw[15])<<24, "name": name, "checksumValid": checksum == 0}
}
