// SPDX-License-Identifier: AGPL-3.0-only
package netops

import (
	"strings"
	"testing"
)

// TestMagicPacket: 6x 0xFF + 16x MAC, 102 bytes total.
func TestMagicPacket(t *testing.T) {
	mac, err := ParseMAC("AA-BB-CC-DD-EE-FF")
	if err != nil {
		t.Fatal(err)
	}
	pkt := MagicPacket(mac)
	if len(pkt) != 102 {
		t.Fatalf("want 102 bytes, got %d", len(pkt))
	}
	for i := 0; i < 6; i++ {
		if pkt[i] != 0xFF {
			t.Fatalf("sync stream broken at %d", i)
		}
	}
	for i := 0; i < 16; i++ {
		got := pkt[6+i*6 : 6+i*6+6]
		if string(got) != string(mac) {
			t.Fatalf("repetition %d wrong", i)
		}
	}
}

// TestParseMAC: colon/hyphen/dotted/clean-hex formats normalize identically.
func TestParseMAC(t *testing.T) {
	want := "aa:bb:cc:dd:ee:ff"
	for _, in := range []string{
		"AA:BB:CC:DD:EE:FF", "aa-bb-cc-dd-ee-ff", "aabb.ccdd.eeff", "aabbccddeeff",
	} {
		mac, err := ParseMAC(in)
		if err != nil || mac.String() != want {
			t.Fatalf("ParseMAC(%q) = %v, %v", in, mac, err)
		}
	}
	for _, bad := range []string{"", "AA:BB:CC:DD:EE", "zz:bb:cc:dd:ee:ff", "11"} {
		if _, err := ParseMAC(bad); err == nil {
			t.Fatalf("%q should fail", bad)
		}
	}
}

// TestCidrHosts: enumeration bounds and refusal of oversized ranges.
func TestCidrHosts(t *testing.T) {
	hosts, err := cidrHosts("192.168.1.0/30", 4096)
	if err != nil || len(hosts) != 2 {
		t.Fatalf("/30: %d hosts, err=%v", len(hosts), err)
	}
	hosts, err = cidrHosts("192.168.1.0/24", 4096)
	if err != nil || len(hosts) != 254 {
		t.Fatalf("/24: %d hosts, err=%v", len(hosts), err)
	}
	if _, err := cidrHosts("10.0.0.0/8", 4096); err == nil {
		t.Fatal("/8 must be refused")
	}
	if _, err := cidrHosts("fe80::/64", 4096); err == nil {
		t.Fatal("IPv6 must be refused for now")
	}
}

// TestLocalCIDR: best-effort local /24 detection (smoke).
func TestLocalCIDR(t *testing.T) {
	c := localCIDR()
	if c != "" && !strings.HasSuffix(c, "/24") {
		t.Fatalf("bad local cidr: %q", c)
	}
}

// TestDiscoverSmokeAndArpSmoke: real ops on the local machine.
func TestDiscoverSmokeAndArpSmoke(t *testing.T) {
	res, err := arpSnapshot()
	if err != nil {
		t.Fatalf("arp snapshot: %v", err)
	}
	if _, ok := res["count"]; !ok {
		t.Fatal("arp result missing count")
	}
	// discover loopback-only /32-ish range to keep the probe cheap
	res, err = discover(map[string]any{"cidr": "127.0.0.0/30", "timeout_ms": 100})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if res["probed"] != 2 {
		t.Fatalf("want 2 probed, got %v", res["probed"])
	}
}
