// SPDX-License-Identifier: AGPL-3.0-only
package media

import "testing"

func TestParseEDID(t *testing.T) {
	edid := make([]byte, 128)
	// Manufacturer ABC, product 0x1234, serial 0x01020304.
	const code = uint16(1<<10 | 2<<5 | 3)
	edid[8], edid[9] = byte(code>>8), byte(code&0xff)
	edid[10], edid[11] = 0x34, 0x12
	edid[12], edid[13], edid[14], edid[15] = 4, 3, 2, 1
	d := 54
	edid[d+3] = 0xfc
	copy(edid[d+5:d+18], "Panel X\n")
	var sum byte
	for _, b := range edid[:127] {
		sum += b
	}
	edid[127] = -sum

	got := parseEDID(edid)
	if got["valid"] != true || got["manufacturer"] != "ABC" || got["productId"] != "1234" || got["serial"] != uint32(0x01020304) || got["name"] != "Panel X" || got["checksumValid"] != true {
		t.Fatalf("unexpected EDID parse: %#v", got)
	}
}

func TestParseEDIDRejectsShortData(t *testing.T) {
	if got := parseEDID(make([]byte, 127)); got["valid"] != false {
		t.Fatalf("short EDID should be invalid: %#v", got)
	}
}
