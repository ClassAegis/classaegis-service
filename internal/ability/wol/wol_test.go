package wol

import (
	"errors"
	"testing"
)

// fakeNetExec records delegated ops and scripts replies.
type fakeNetExec struct {
	ops   []string
	args  []map[string]any
	reply map[string]any
	err   error
}

func (f *fakeNetExec) call(op string, args map[string]any) (map[string]any, error) {
	f.ops = append(f.ops, op)
	f.args = append(f.args, args)
	if f.err != nil {
		return nil, f.err
	}
	return f.reply, nil
}

// TestWakeDelegatesAndValidates: valid MAC delegates; missing MAC fails.
func TestWakeDelegatesAndValidates(t *testing.T) {
	fx := &fakeNetExec{reply: map[string]any{"sent": 3}}
	e := New(Options{NetExec: fx.call})
	ok, code, message, res := e.Handle("m1", "WOL_WAKE", map[string]any{"mac": "AA:BB:CC:DD:EE:FF", "port": 9}, nil)
	if !ok || res["sent"] != 3 {
		t.Fatalf("wake failed: %v %s %s", ok, code, message)
	}
	if fx.ops[0] != "wol" || fx.args[0]["mac"] != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("delegation wrong: %+v", fx.args[0])
	}
	ok, code, _, _ = e.Handle("m2", "WOL_WAKE", map[string]any{}, nil)
	if ok || code != "E_PARAM" {
		t.Fatalf("missing mac must fail, got ok=%v code=%s", ok, code)
	}
	ok, code, _, _ = e.Handle("m3", "WOL_WAKE", map[string]any{"mac": "not-a-mac"}, nil)
	if ok || code != "E_PARAM" {
		t.Fatalf("bad mac must fail locally with E_PARAM, got ok=%v code=%s", ok, code)
	}
}

// TestDiscoverAndARPDelegate: both pass through with args intact.
func TestDiscoverAndARPDelegate(t *testing.T) {
	fx := &fakeNetExec{reply: map[string]any{"hosts": []any{}}}
	e := New(Options{NetExec: fx.call})
	ok, _, _, res := e.Handle("m1", "WOL_DISCOVER", map[string]any{"cidr": "192.168.1.0/24"}, nil)
	if !ok || res == nil {
		t.Fatal("discover failed")
	}
	if fx.args[0]["cidr"] != "192.168.1.0/24" {
		t.Fatalf("cidr arg lost: %+v", fx.args[0])
	}
	ok, _, _, _ = e.Handle("m2", "WOL_ARP", nil, nil)
	if !ok || fx.ops[1] != "arp" {
		t.Fatal("arp delegation failed")
	}
	// proxy failure maps to E_FAILED
	fx2 := &fakeNetExec{err: errors.New("core offline")}
	e2 := New(Options{NetExec: fx2.call})
	ok, code, _, _ := e2.Handle("m3", "WOL_DISCOVER", nil, nil)
	if ok || code != "E_FAILED" {
		t.Fatalf("proxy failure must be E_FAILED, got %v %s", ok, code)
	}
}

// TestNilNetExec: engine without proxy answers E_FAILED cleanly.
func TestNilNetExec(t *testing.T) {
	e := New(Options{})
	ok, code, _, _ := e.Handle("m1", "WOL_WAKE", map[string]any{"mac": "AA:BB:CC:DD:EE:FF"}, nil)
	if ok || code != "E_FAILED" {
		t.Fatalf("nil proxy must E_FAILED, got ok=%v code=%s", ok, code)
	}
}
