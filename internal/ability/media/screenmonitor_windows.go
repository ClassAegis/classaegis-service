// SPDX-License-Identifier: AGPL-3.0-only
//go:build windows

package media

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	mediaUser32     = windows.NewLazySystemDLL("user32.dll")
	mediaMessageBox = mediaUser32.NewProc("MessageBoxW")
)

func confirmScreenMonitoring(seconds, interval, width int) bool {
	text, _ := windows.UTF16PtrFromString(fmt.Sprintf("ClassAegis will capture the foreground window of a supported media player at %dpx width every %d seconds for up to %d seconds. Frames are uploaded to the configured analysis server. Start?", width, interval, seconds))
	title, _ := windows.UTF16PtrFromString("ClassAegis AI screen monitoring")
	result, _, _ := mediaMessageBox.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x00000004|0x00000030|0x00040000|0x00010000)
	return result == 6
}

func captureForegroundMediaWindow(path string, width int) error {
	script := `Add-Type -AssemblyName System.Drawing; Add-Type -TypeDefinition @'
using System; using System.Runtime.InteropServices;
public static class WinCapture { [StructLayout(LayoutKind.Sequential)] public struct RECT { public int L,T,R,B; }
[DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
[DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h, out uint p);
[DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r); }
'@; $h=[WinCapture]::GetForegroundWindow(); if($h -eq [IntPtr]::Zero){exit 20}; [uint32]$pid=0; [void][WinCapture]::GetWindowThreadProcessId($h,[ref]$pid); $p=Get-Process -Id $pid -ErrorAction SilentlyContinue; if(!$p){exit 20}; $allowed=@('vlc','mpc-hc','mpc-hc64','mpc-be','mpc-be64','wmplayer','video.ui'); if($allowed -notcontains $p.ProcessName.ToLowerInvariant()){exit 20}; $r=New-Object WinCapture+RECT; if(![WinCapture]::GetWindowRect($h,[ref]$r)){exit 21}; $sw=$r.R-$r.L; $sh=$r.B-$r.T; if($sw -lt 160 -or $sh -lt 90){exit 20}; $ow=WIDTH; $oh=[Math]::Max(1,[int]($sh*$ow/$sw)); $src=New-Object System.Drawing.Bitmap($sw,$sh); $g=[System.Drawing.Graphics]::FromImage($src); $g.CopyFromScreen($r.L,$r.T,0,0,$src.Size); $dst=New-Object System.Drawing.Bitmap($ow,$oh); $d=[System.Drawing.Graphics]::FromImage($dst); $d.InterpolationMode=[System.Drawing.Drawing2D.InterpolationMode]::Low; $d.DrawImage($src,0,0,$ow,$oh); $enc=[System.Drawing.Imaging.ImageCodecInfo]::GetImageEncoders() | Where-Object {$_.MimeType -eq 'image/jpeg'} | Select-Object -First 1; $ep=New-Object System.Drawing.Imaging.EncoderParameters(1); $ep.Param[0]=New-Object System.Drawing.Imaging.EncoderParameter([System.Drawing.Imaging.Encoder]::Quality,[long]45); $dst.Save('OUTPATH',$enc,$ep); $d.Dispose(); $dst.Dispose(); $g.Dispose(); $src.Dispose()`
	script = strings.ReplaceAll(script, "WIDTH", fmt.Sprint(width))
	script = strings.ReplaceAll(script, "OUTPATH", "'"+strings.ReplaceAll(path, "'", "''")+"'")
	encoded := base64.StdEncoding.EncodeToString(utf16LE(script))
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", encoded)
	if out, err := cmd.CombinedOutput(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 20 {
			return errNoMediaWindow
		}
		return fmt.Errorf("window capture failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func utf16LE(s string) []byte {
	u, _ := windows.UTF16FromString(s)
	b := make([]byte, len(u)*2)
	for i, v := range u {
		b[2*i] = byte(v)
		b[2*i+1] = byte(v >> 8)
	}
	return b
}
