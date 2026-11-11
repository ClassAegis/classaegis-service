//go:build windows

package netops

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows bindings: IcmpSendEcho (icmp probe, no elevation needed) and
// GetIpNetTable (ARP table read), both from iphlpapi.dll.

var (
	iphlpapi       = windows.NewLazySystemDLL("iphlpapi.dll")
	procIcmpCreate = iphlpapi.NewProc("IcmpCreateFile")
	procIcmpSend   = iphlpapi.NewProc("IcmpSendEcho")
	procIcmpClose  = iphlpapi.NewProc("IcmpCloseHandle")
	procGetNetTab  = iphlpapi.NewProc("GetIpNetTable")
)

// ping sends one ICMP echo request, returns true on any reply.
func ping(ip net.IP, timeout time.Duration) bool {
	h, _, _ := procIcmpCreate.Call()
	if h == 0 || h == ^uintptr(0) { // INVALID_HANDLE_VALUE
		return false
	}
	defer procIcmpClose.Call(h)
	payload := []byte("ocm-probe")
	reply := make([]byte, 48+len(payload)+16)
	dest := binary.BigEndian.Uint32(ip.To4())
	r1, _, _ := procIcmpSend.Call(h,
		uintptr(dest),
		uintptr(unsafe.Pointer(&payload[0])),
		uintptr(len(payload)),
		0, // options
		uintptr(unsafe.Pointer(&reply[0])),
		uintptr(len(reply)),
		uintptr(timeout.Milliseconds()))
	return r1 > 0
}

// arpTable reads the ARP cache (dynamic/static entries) via GetIpNetTable.
func arpTable() []arpRow {
	var size uint32
	procGetNetTab.Call(0, uintptr(unsafe.Pointer(&size)), 0)
	if size == 0 {
		return nil
	}
	buf := make([]byte, size)
	r1, _, _ := procGetNetTab.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 1)
	if r1 != 0 {
		return nil
	}
	n := int(binary.LittleEndian.Uint32(buf[0:4]))
	const rowSize = 24
	if n > (len(buf)-4)/rowSize {
		n = (len(buf) - 4) / rowSize
	}
	var out []arpRow
	for i := 0; i < n; i++ {
		off := 4 + i*rowSize
		row := buf[off : off+rowSize]
		physLen := int(binary.LittleEndian.Uint32(row[4:8]))
		var mac string
		if physLen > 0 && physLen <= 8 {
			macBytes := row[8 : 8+physLen]
			mac = fmt.Sprintf("%02X-%02X-%02X-%02X-%02X-%02X",
				macBytes[0], macBytes[1], macBytes[2], macBytes[3], macBytes[4], macBytes[5])
		}
		addr := binary.LittleEndian.Uint32(row[16:20])
		var ipb [4]byte
		binary.LittleEndian.PutUint32(ipb[:], addr)
		ip := net.IPv4(ipb[0], ipb[1], ipb[2], ipb[3]).String()
		typCode := binary.LittleEndian.Uint32(row[20:24])
		typ := "other"
		switch typCode {
		case 2:
			typ = "invalid"
		case 3:
			typ = "dynamic"
		case 4:
			typ = "static"
		}
		if typ == "invalid" || typ == "other" {
			continue
		}
		out = append(out, arpRow{IP: ip, MAC: mac, Type: typ})
	}
	return out
}
