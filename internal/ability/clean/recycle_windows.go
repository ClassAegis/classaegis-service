// SPDX-License-Identifier: AGPL-3.0-only
//go:build windows

package clean

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// shellRecycler implements recycleAPI via shell32 (SHQueryRecycleBinW /
// SHEmptyRecycleBinW). Not COM, plain C API calls.
type shellRecycler struct{}

var (
	shell32                = windows.NewLazySystemDLL("shell32.dll")
	procSHQueryRecycleBinW = shell32.NewProc("SHQueryRecycleBinW")
	procSHEmptyRecycleBinW = shell32.NewProc("SHEmptyRecycleBinW")
)

// shQueryRBINFO mirrors SHQUERYRBINFO.
type shQueryRBINFO struct {
	CbSize      uint32
	I64Size     int64
	I64NumItems int32
}

// Query reports aggregate item count and byte size for one drive root.
func (s *shellRecycler) Query(drive string) (int64, int64, error) {
	root, err := windows.UTF16PtrFromString(drive)
	if err != nil {
		return 0, 0, err
	}
	var info shQueryRBINFO
	info.CbSize = uint32(unsafe.Sizeof(info))
	r1, _, _ := procSHQueryRecycleBinW.Call(uintptr(unsafe.Pointer(root)), uintptr(unsafe.Pointer(&info)))
	if r1 != 0 {
		return 0, 0, fmt.Errorf("SHQueryRecycleBinW(%s) failed: 0x%x", drive, r1)
	}
	return int64(info.I64NumItems), info.I64Size, nil
}

// Empty empties the recycle bin of one drive root (no confirmation, no UI).
func (s *shellRecycler) Empty(drive string) error {
	root, err := windows.UTF16PtrFromString(drive)
	if err != nil {
		return err
	}
	const flags = 0x1 | 0x2 | 0x4 // SHERB_NOCONFIRMATION | NOPROGRESSUI | NOSOUND
	r1, _, _ := procSHEmptyRecycleBinW.Call(0, uintptr(unsafe.Pointer(root)), flags)
	if r1 != 0 {
		return fmt.Errorf("SHEmptyRecycleBinW(%s) failed: 0x%x", drive, r1)
	}
	return nil
}

// fixedDrives lists existing fixed drive roots C:\ .. Z:\.
func fixedDrives() []string {
	var roots []string
	for _, d := range "CDEFGHIJKLMNOPQRSTUVWXYZ" {
		root := string(d) + `:\`
		if _, err := os.Stat(root); err == nil {
			roots = append(roots, root)
		}
	}
	return roots
}
