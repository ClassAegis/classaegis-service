// SPDX-License-Identifier: AGPL-3.0-only
// Package netops implements the delegated network operations that core
// executes on behalf of the ability process: Wake-on-LAN magic packets,
// ICMP+ARP subnet discovery, and ARP table snapshots. Pure-Go logic here;
// Windows syscalls live in netops_windows.go.
package netops

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Result of one delegated operation (map form matches NetExecResponse).
type Result = map[string]any

// Execute dispatches one delegated operation.
func Execute(op string, args map[string]any) (Result, error) {
	switch op {
	case "wol":
		return wol(args)
	case "discover":
		return discover(args)
	case "arp":
		return arpSnapshot()
	default:
		return nil, fmt.Errorf("unknown netops op: %s", op)
	}
}

// MagicPacket builds the 102-byte AMD Magic Packet: 6x 0xFF + 16x MAC.
func MagicPacket(mac net.HardwareAddr) []byte {
	pkt := make([]byte, 6+16*len(mac))
	for i := 0; i < 6; i++ {
		pkt[i] = 0xFF
	}
	for i := 0; i < 16; i++ {
		copy(pkt[6+i*6:], mac)
	}
	return pkt
}

// ParseMAC accepts AA:BB:CC:DD:EE:FF / AA-BB-CC-DD-EE-FF / AABB.CCDD.EEFF.
func ParseMAC(s string) (net.HardwareAddr, error) {
	s = strings.TrimSpace(s)
	sep := ""
	switch {
	case strings.Contains(s, ":"):
		sep = ":"
	case strings.Contains(s, "-"):
		sep = "-"
	}
	if sep != "" {
		parts := strings.Split(s, sep)
		if len(parts) != 6 {
			return nil, fmt.Errorf("bad mac: %s", s)
		}
		var out [6]byte
		for i, p := range parts {
			v, err := strconv.ParseUint(p, 16, 8)
			if err != nil {
				return nil, fmt.Errorf("bad mac octet %q: %w", p, err)
			}
			out[i] = byte(v)
		}
		return net.HardwareAddr(out[:]), nil
	}
	// dotted quad of 4 hex groups or 12 hex chars
	clean := strings.ReplaceAll(s, ".", "")
	clean = strings.ReplaceAll(clean, ":", "")
	clean = strings.ReplaceAll(clean, "-", "")
	if len(clean) != 12 {
		return nil, fmt.Errorf("bad mac: %s", s)
	}
	var out [6]byte
	for i := 0; i < 6; i++ {
		v, err := strconv.ParseUint(clean[2*i:2*i+2], 16, 8)
		if err != nil {
			return nil, fmt.Errorf("bad mac: %s", s)
		}
		out[i] = byte(v)
	}
	return net.HardwareAddr(out[:]), nil
}

// cidrHosts enumerates the unicast hosts of a CIDR, capped and rejecting
// ranges that are too large to scan politely.
func cidrHosts(cidr string, maxHosts int) ([]net.IP, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("bad cidr %q: %w", cidr, err)
	}
	ones, bits := ipnet.Mask.Size()
	if bits != 32 {
		return nil, fmt.Errorf("only IPv4 CIDR supported, got %q", cidr)
	}
	if ones < 20 { // larger than /20 = >4096 hosts: refuse
		return nil, fmt.Errorf("cidr too large: /%d (max /20)", ones)
	}
	base := binary.BigEndian.Uint32(ipnet.IP.To4())
	mask := binary.BigEndian.Uint32(ipnet.Mask)
	broadcast := base | ^mask
	var out []net.IP
	for cur := base + 1; cur < broadcast && len(out) < maxHosts; cur++ {
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], cur)
		out = append(out, net.IPv4(b[0], b[1], b[2], b[3]).To4())
	}
	return out, nil
}

// localCIDR guesses the /24 of the first non-loopback IPv4 address.
func localCIDR() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok && !ipn.IP.IsLoopback() && ipn.IP.To4() != nil {
			return ipn.IP.To4().Mask(ipnet24Mask()).String() + "/24"
		}
	}
	return ""
}

func ipnet24Mask() net.IPMask {
	return net.CIDRMask(24, 32)
}

// wol sends magic packets via UDP broadcast.
func wol(args map[string]any) (Result, error) {
	macs := []string{}
	if m, ok := args["mac"].(string); ok && m != "" {
		macs = append(macs, m)
	}
	if raw, ok := args["macs"].([]any); ok {
		for _, v := range raw {
			if m, ok := v.(string); ok && m != "" {
				macs = append(macs, m)
			}
		}
	}
	if len(macs) == 0 {
		return nil, fmt.Errorf("mac or macs required")
	}
	port := 9
	if n, ok := args["port"].(int); ok && n > 0 {
		port = n
	} else if n, ok := intOf(args["port"]); ok && n > 0 {
		port = int(n)
	}
	var targets []string
	var hw []net.HardwareAddr
	for _, m := range macs {
		h, err := ParseMAC(m)
		if err != nil {
			return nil, err
		}
		targets = append(targets, h.String())
		hw = append(hw, h)
	}
	sent := 0
	for _, h := range hw {
		pkt := MagicPacket(h)
		for i := 0; i < 3; i++ {
			conn, err := net.DialTimeout("udp", fmt.Sprintf("255.255.255.255:%d", port), 2*time.Second)
			if err != nil {
				return nil, fmt.Errorf("dial broadcast: %w", err)
			}
			if _, err := conn.Write(pkt); err != nil {
				conn.Close()
				return nil, fmt.Errorf("write magic packet: %w", err)
			}
			conn.Close()
			sent++
		}
	}
	return Result{
		"sent":    sent,
		"packets": 3,
		"port":    port,
		"targets": targets,
	}, nil
}

// intOf reads a numeric arg from int / int64 / float64 / string / json-ish.
func intOf(v any) (int64, bool) {
	switch t := v.(type) {
	case int:
		return int64(t), true
	case int64:
		return t, true
	case float64:
		return int64(t), true
	case string:
		n, err := strconv.ParseInt(t, 10, 64)
		return n, err == nil
	}
	return 0, false
}

// discover probes a CIDR with ICMP then correlates the ARP table.
func discover(args map[string]any) (Result, error) {
	cidr, _ := args["cidr"].(string)
	if cidr == "" {
		cidr = localCIDR()
	}
	if cidr == "" {
		return nil, fmt.Errorf("no cidr and no local interface address")
	}
	timeoutMs := 300
	if n, ok := intOf(args["timeout_ms"]); ok && n > 0 {
		if n > 2000 {
			n = 2000
		}
		timeoutMs = int(n)
	}
	hosts, err := cidrHosts(cidr, 4096)
	if err != nil {
		return nil, err
	}
	probe(hosts, time.Duration(timeoutMs)*time.Millisecond)
	table := arpTable()
	ipMAC := map[string]string{}
	for _, row := range table {
		if row.MAC == "" {
			continue
		}
		ipMAC[row.IP] = row.MAC
	}
	var found []Result
	set := map[string]bool{}
	for _, ip := range hosts {
		if mac, ok := ipMAC[ip.String()]; ok {
			key := ip.String() + mac
			if set[key] {
				continue
			}
			set[key] = true
			found = append(found, Result{"ip": ip.String(), "mac": mac})
		}
	}
	if found == nil {
		found = []Result{}
	}
	return Result{
		"cidr":   cidr,
		"probed": len(hosts),
		"hosts":  found,
		"errors": []string{},
	}, nil
}

// arpSnapshot returns the current ARP table.
func arpSnapshot() (Result, error) {
	table := arpTable()
	var rows []Result
	for _, row := range table {
		if row.MAC == "" {
			continue
		}
		rows = append(rows, Result{"ip": row.IP, "mac": row.MAC, "type": row.Type})
	}
	if rows == nil {
		rows = []Result{}
	}
	return Result{"hosts": rows, "count": len(rows)}, nil
}

// arpRow is one ARP table entry.
type arpRow struct {
	IP   string
	MAC  string
	Type string
}

// probe sends ICMP echo requests concurrently.
func probe(hosts []net.IP, timeout time.Duration) {
	const workers = 32
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for _, h := range hosts {
		wg.Add(1)
		go func(ip net.IP) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ping(ip, timeout)
		}(h)
	}
	wg.Wait()
}
