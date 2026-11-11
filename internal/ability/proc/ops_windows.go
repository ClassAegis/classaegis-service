//go:build windows

package proc

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func newSysOps() sysOps { return &windowsOps{} }

type windowsOps struct{}

const (
	procSetQuota         = 0x0100
	procQueryInformation = 0x0400
	procSetInformation   = 0x0200
	procQueryLimitedInfo = 0x1000
	priorityIdle         = 0x00000040
	priorityBelowNormal  = 0x00004000
	priorityNormal       = 0x00000020
	priorityAboveNormal  = 0x00008000
	priorityHigh         = 0x00000080
	tcpTableOwnerPIDAll  = 5
	mibTcpStateEstab     = 5
	tcpConnectionEstData = 1
	afInet               = 2
	afInet6              = 23
)

var (
	iphlpapi                     = windows.NewLazySystemDLL("iphlpapi.dll")
	psapi                        = windows.NewLazySystemDLL("psapi.dll")
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procGetExtTcp                = iphlpapi.NewProc("GetExtendedTcpTable")
	procGetPerTcp                = iphlpapi.NewProc("GetPerTcpConnectionEStats")
	procGetPerTcp6               = iphlpapi.NewProc("GetPerTcp6ConnectionEStats")
	procGetProcMem               = psapi.NewProc("GetProcessMemoryInfo")
	procEmptyWS                  = psapi.NewProc("EmptyWorkingSet")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
)

// listProcesses snapshots pid/name/working set via toolhelp + psapi.
func (w *windowsOps) listProcesses() ([]procInfo, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)
	var out []procInfo
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if err := windows.Process32First(snap, &entry); err != nil {
		return nil, err
	}
	for {
		name := windows.UTF16ToString(entry.ExeFile[:])
		ws := uint64(0)
		if h, err := windows.OpenProcess(procQueryLimitedInfo, false, entry.ProcessID); err == nil {
			ws = workingSet(h)
			windows.CloseHandle(h)
		}
		out = append(out, procInfo{PID: entry.ProcessID, Name: name, WorkingSet: ws})
		if err := windows.Process32Next(snap, &entry); err != nil {
			break
		}
	}
	return out, nil
}

// workingSet reads WorkingSetSize via GetProcessMemoryInfo.
func workingSet(h windows.Handle) uint64 {
	var pmc struct {
		Cb                      uint32
		PageFaultCount          uint32
		PeakWorkingSetSize      uintptr
		WorkingSetSize          uintptr
		QuotaPeakPagedPoolUsage uintptr
		QuotaPagedPoolUsage     uintptr
		QuotaNonPagedPoolUsage  uintptr
		PagefileUsage           uintptr
		PeakPagefileUsage       uintptr
	}
	pmc.Cb = uint32(unsafe.Sizeof(pmc))
	r1, _, _ := procGetProcMem.Call(uintptr(h), uintptr(unsafe.Pointer(&pmc)), uintptr(pmc.Cb))
	if r1 == 0 {
		return 0
	}
	return uint64(pmc.WorkingSetSize)
}

// trimWorkingSet empties the working set and reports freed physical bytes.
func (w *windowsOps) trimWorkingSet(pid uint32) (int64, error) {
	h, err := windows.OpenProcess(procSetQuota|procQueryInformation, false, pid)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(h)
	before := workingSet(h)
	if r1, _, _ := procEmptyWS.Call(uintptr(h)); r1 == 0 {
		return 0, fmt.Errorf("EmptyWorkingSet failed for pid %d", pid)
	}
	after := workingSet(h)
	freed := int64(before) - int64(after)
	if freed < 0 {
		freed = 0
	}
	return freed, nil
}

// setPriority sets the priority class of one process.
func (w *windowsOps) setPriority(pid uint32, pri uint32) error {
	h, err := windows.OpenProcess(procSetInformation|procQueryLimitedInfo, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.SetPriorityClass(h, pri)
}

// connSamples reads established TCP connections (v4+v6) with per-connection
// cumulative byte counters from TCP estats, attributed to owning pids.
// UDP has no per-socket counters and is not accounted (documented limit).
func (w *windowsOps) connSamples() ([]connSample, error) {
	var out []connSample
	out = append(out, tcpSamples(afInet, procGetPerTcp, 24)...)
	out = append(out, tcpSamples(afInet6, procGetPerTcp6, 56)...)
	return out, nil
}

func (w *windowsOps) foregroundPID() (uint32, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return 0, nil
	}
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return pid, nil
}

// tcpSamples reads one family's TCP table and queries estats per row.
// rowSize: 24 (v4) / 48 (v6); the first 20/44 bytes are the MIB_TCP{,6}ROW
// the estats API needs, so we pass pointers straight into the table buffer.
func tcpSamples(family uint32, est *windows.LazyProc, rowSize int) []connSample {
	// size probe then fetch
	var size uint32
	procGetExtTcp.Call(0, 0, uintptr(family), tcpTableOwnerPIDAll, 0, uintptr(unsafe.Pointer(&size)), 0)
	if size == 0 {
		return nil
	}
	buf := make([]byte, size)
	r1, _, _ := procGetExtTcp.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(size),
		uintptr(family), tcpTableOwnerPIDAll, 0, uintptr(unsafe.Pointer(&size)), 0)
	if r1 == 0 {
		return nil
	}
	n := int(binary.LittleEndian.Uint32(buf[0:4]))
	if n > (len(buf)-4)/rowSize {
		n = (len(buf) - 4) / rowSize
	}
	var out []connSample
	stateOffset, pidOffset := 0, rowSize-4
	if family == afInet6 {
		stateOffset, pidOffset = 48, 52
	}
	for i := 0; i < n; i++ {
		off := 4 + i*rowSize
		row := buf[off : off+rowSize]
		state := binary.LittleEndian.Uint32(row[stateOffset : stateOffset+4])
		if state != mibTcpStateEstab {
			continue
		}
		pid := binary.LittleEndian.Uint32(row[pidOffset : pidOffset+4])
		in, outBytes, ok := queryEstats(est, row, pidOffset)
		if !ok {
			continue
		}
		class := "wan"
		var ip net.IP
		if family == afInet && len(row) >= 16 {
			ip = net.IPv4(row[12], row[13], row[14], row[15])
		}
		if family == afInet6 && len(row) >= 40 {
			ip = net.IP(row[24:40])
		}
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			class = "lan"
		}
		out = append(out, connSample{PID: pid, In: in, Out: outBytes, Class: class})
	}
	return out
}

// queryEstats reads DataBytesIn/DataBytesOut (first two uint64s of
// TCP_ESTATS_DATA_ROD_v0) for one row. ok=false means unavailable.
func queryEstats(est *windows.LazyProc, row []byte, rowLen int) (uint64, uint64, bool) {
	var version, rodSize uint32
	var rod [16]byte
	r1, _, _ := est.Call(uintptr(unsafe.Pointer(&row[0])), tcpConnectionEstData,
		0, 0, 0,
		uintptr(unsafe.Pointer(&rod[0])), 0, uintptr(len(rod)),
		uintptr(unsafe.Pointer(&version)), uintptr(unsafe.Pointer(&rodSize)))
	if r1 == 0 || rodSize < 16 {
		return 0, 0, false
	}
	out := binary.LittleEndian.Uint64(rod[0:8])
	in := binary.LittleEndian.Uint64(rod[8:16])
	return in, out, true
}

// qosRoot/qosBase form the policy-based QoS registry location (both
// injectable for tests; prod writes HKLM, tests may aim at HKCU).
var (
	qosRoot = registry.LOCAL_MACHINE
	qosBase = `SOFTWARE\Policies\Microsoft\Windows\QoS`
)

// qosApply writes (rateKBps > 0) or removes (= 0) a Policy-QoS throttle for
// one executable. EXPERIMENTAL: schema per Policy-based QoS registry layout;
// an unreadable policy is ignored by psched, so worst case is "no throttle".
func (w *windowsOps) qosApply(exe string, rateKBps uint32) error {
	name := qosPolicyName(exe)
	path := qosBase + `\` + name
	if rateKBps == 0 {
		k, err := registry.OpenKey(qosRoot, qosBase, registry.QUERY_VALUE)
		if err == nil {
			defer k.Close()
			_ = registry.DeleteKey(qosRoot, path)
		}
		return nil
	}
	k, _, err := registry.CreateKey(qosRoot, path, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	values := []struct {
		name string
		val  any
	}{
		{"Version", uint32(1)},
		{"Throttle Rate", rateKBps},
		{"App Name", exe},
		{"Protocol", uint32(0)},
		{"Direction", uint32(1)},
		{"Local IP", ""},
		{"Remote IP", ""},
		{"Local Port", ""},
		{"Remote Port", ""},
		{"DSCP Value", uint32(0)},
	}
	for _, v := range values {
		switch t := v.val.(type) {
		case uint32:
			if err := k.SetDWordValue(v.name, t); err != nil {
				return err
			}
		case string:
			if err := k.SetStringValue(v.name, t); err != nil {
				return err
			}
		}
	}
	return nil
}

// qosPolicyName sanitizes an exe name into a policy key name.
func qosPolicyName(exe string) string {
	s := strings.TrimSuffix(strings.ToLower(exe), ".exe")
	s = strings.Map(func(r rune) rune {
		ok := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_'
		if ok {
			return r
		}
		return '-'
	}, s)
	if len(s) > 48 {
		s = s[:48]
	}
	return "ocm-" + s
}

// isElevated reports whether the current process token is elevated.
func (w *windowsOps) isElevated() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()
	var elev uint32
	var ret uint32
	if err := windows.GetTokenInformation(token, windows.TokenElevation, (*byte)(unsafe.Pointer(&elev)), uint32(unsafe.Sizeof(elev)), &ret); err != nil {
		return false
	}
	return elev != 0
}
