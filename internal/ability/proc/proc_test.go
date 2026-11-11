package proc

import (
	"errors"
	"testing"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// fakeOps drives fold-based traffic tests deterministically.
type fakeOps struct {
	samples [][]connSample
	calls   int
	isElev  bool
}

func (f *fakeOps) listProcesses() ([]procInfo, error) { return nil, nil }

func (f *fakeOps) trimWorkingSet(pid uint32) (int64, error) { return 0, nil }

func (f *fakeOps) setPriority(pid uint32, pri uint32) error { return nil }

func (f *fakeOps) connSamples() ([]connSample, error) {
	i := f.calls
	if f.calls < len(f.samples) {
		f.calls++
	}
	if len(f.samples) == 0 {
		return nil, nil
	}
	return f.samples[min(i, len(f.samples)-1)], nil
}

func (f *fakeOps) qosApply(exe string, rateKBps uint32) error { return nil }

func (f *fakeOps) isElevated() bool { return f.isElev }

func (f *fakeOps) foregroundPID() (uint32, error) { return 0, nil }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestFoldDeltas: first sight seeds counters, later ticks add deltas and
// compute rates over the elapsed interval.
func TestFoldDeltas(t *testing.T) {
	e := New(Options{PollIntervalMS: 5000})
	t0 := time.Now()
	e.fold([]connSample{{PID: 100, In: 1000, Out: 500}}, t0, 0)
	acc := e.accum[uint32(100)]
	if acc == nil || acc.TotalIn != 0 || !acc.seen {
		t.Fatalf("first sight must seed, got %+v", acc)
	}
	e.fold([]connSample{{PID: 100, In: 3000, Out: 1500}}, t0.Add(5*time.Second), 0)
	acc = e.accum[uint32(100)]
	if acc.TotalIn != 2000 || acc.TotalOut != 1000 {
		t.Fatalf("want deltas 2000/1000, got %+v", acc)
	}
	if acc.RateIn != 400 || acc.RateOut != 200 {
		t.Fatalf("want rates 400/200, got %+v", acc)
	}
	// a brand-new pid appears; pid 100 stays retained until cutoff
	e.fold([]connSample{{PID: 200, In: 10, Out: 0}}, t0.Add(10*time.Second), 0)
	if e.accum[uint32(200)] == nil {
		t.Fatal("new pid must appear")
	}
	if e.accum[uint32(100)] == nil {
		t.Fatal("pid 100 must be retained before the fadeout cutoff")
	}
}

// TestFoldFadeout: pids unseen for 5 intervals are dropped.
func TestFoldFadeout(t *testing.T) {
	e := New(Options{PollIntervalMS: 1000})
	t0 := time.Now()
	e.fold([]connSample{{PID: 7, In: 100, Out: 0}}, t0, 0)
	for i := 1; i <= 6; i++ {
		e.fold(nil, t0.Add(time.Duration(i)*time.Second), 0)
	}
	if _, ok := e.accum[uint32(7)]; ok {
		t.Fatal("pid 7 must be faded out after 6 empty ticks")
	}
}

// TestFoldClampNegative: a shrinking counter (pid reuse / reset) clamps at 0.
func TestFoldClampNegative(t *testing.T) {
	e := New(Options{})
	t0 := time.Now()
	e.fold([]connSample{{PID: 7, In: 5000, Out: 0}}, t0, 0)
	e.fold([]connSample{{PID: 7, In: 10, Out: 0}}, t0.Add(5*time.Second), 0)
	if e.accum[uint32(7)].TotalIn != 0 {
		t.Fatalf("negative delta must clamp to 0, got %d", e.accum[uint32(7)].TotalIn)
	}
}

func TestFoldSeparatesLanAndWanDeltas(t *testing.T) {
	e := New(Options{})
	t0 := time.Now()
	e.fold([]connSample{{PID: 8, In: 100, Out: 50, Class: "lan"}, {PID: 8, In: 200, Out: 80, Class: "wan"}}, t0, 0)
	e.fold([]connSample{{PID: 8, In: 160, Out: 90, Class: "lan"}, {PID: 8, In: 270, Out: 100, Class: "wan"}}, t0.Add(time.Second), 0)
	a := e.accum[8]
	if a.LanIn != 60 || a.LanOut != 40 || a.WanIn != 70 || a.WanOut != 20 {
		t.Fatalf("unexpected classified byte deltas: %+v", a)
	}
}

// TestPriorityClassMap: protocol names map to Windows classes.
func TestPriorityClassMap(t *testing.T) {
	if pri, ok := priorityClass("below_normal"); !ok || pri != priorityBelowNormal {
		t.Fatalf("below_normal wrong: %v %v", pri, ok)
	}
	if pri, ok := priorityClass("HIGH"); !ok || pri != priorityHigh {
		t.Fatalf("HIGH wrong: %v %v", pri, ok)
	}
	if _, ok := priorityClass("realtime"); ok {
		t.Fatal("realtime must not be accepted")
	}
	if _, ok := priorityClass(""); ok {
		t.Fatal("empty must not be accepted")
	}
}

// TestQosPolicyName: exe names sanitize into stable policy keys.
func TestQosPolicyName(t *testing.T) {
	if got := qosPolicyName("SomeApp.EXE"); got != "ocm-someapp" {
		t.Fatalf("got %q", got)
	}
	if got := qosPolicyName("A B/C.exe"); got != "ocm-a-b-c" {
		t.Fatalf("got %q", got)
	}
}

// TestSkipProc: critical and own processes are protected.
func TestSkipProc(t *testing.T) {
	for _, n := range []string{"csrss.exe", "lsass.exe", "ability.exe", "core.exe", ""} {
		if !skipProc(n) {
			t.Fatalf("%q must be skipped", n)
		}
	}
	for _, n := range []string{"notepad.exe", "explorer.exe"} {
		if skipProc(n) {
			t.Fatalf("%q must not be skipped", n)
		}
	}
}

// TestExeNameRe: throttle accepts bare exe names only.
func TestExeNameRe(t *testing.T) {
	good := []string{"app.exe", "My-App_1.0.exe"}
	bad := []string{"", `C:\evil\app.exe`, `a\b.exe`, "app", "app.EXE1", "*.exe"}
	for _, g := range good {
		if !exeNameRe.MatchString(g) {
			t.Fatalf("%q should match", g)
		}
	}
	for _, b := range bad {
		if exeNameRe.MatchString(b) {
			t.Fatalf("%q should NOT match", b)
		}
	}
}

// TestThrottleRequiresElevation: unelevated processes get
// E_PLATFORM_UNSUPPORTED before touching the registry.
func TestThrottleRequiresElevation(t *testing.T) {
	e := New(Options{})
	e.ops = &fakeOps{isElev: false}
	ok, code, _, _ := e.Handle("m1", "PROC_THROTTLE", map[string]any{"process": "app.exe", "rateKBps": 500}, nil)
	if ok || code != "E_PLATFORM_UNSUPPORTED" {
		t.Fatalf("want E_PLATFORM_UNSUPPORTED, got ok=%v code=%s", ok, code)
	}
}

// TestQosRegistryRoundtrip exercises the real registry code path against
// HKCU (the QoS root is injectable); HKLM policy writes need elevation.
func TestQosRegistryRoundtrip(t *testing.T) {
	oldRoot, oldBase := qosRoot, qosBase
	qosRoot = registry.CURRENT_USER
	qosBase = `Software\OneClassMesh\test-qos`
	t.Cleanup(func() {
		qosRoot, qosBase = oldRoot, oldBase
		_ = registry.DeleteKey(registry.CURRENT_USER, `Software\OneClassMesh\test-qos\ocm-testapp`)
		_ = registry.DeleteKey(registry.CURRENT_USER, `Software\OneClassMesh\test-qos`)
		_ = registry.DeleteKey(registry.CURRENT_USER, `Software\OneClassMesh`)
	})
	w := &windowsOps{}
	if err := w.qosApply("TestApp.exe", 500); err != nil {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			t.Skipf("registry access is restricted in this test environment: %v", err)
		}
		t.Fatalf("apply failed: %v", err)
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\OneClassMesh\test-qos\ocm-testapp`, registry.QUERY_VALUE)
	if err != nil {
		t.Fatalf("policy key missing: %v", err)
	}
	defer k.Close()
	if rate, _, err := k.GetIntegerValue("Throttle Rate"); err != nil || rate != 500 {
		t.Fatalf("throttle rate wrong: %v %v", rate, err)
	}
	if app, _, err := k.GetStringValue("App Name"); err != nil || app != "TestApp.exe" {
		t.Fatalf("app name wrong: %v %v", app, err)
	}
	// remove
	if err := w.qosApply("TestApp.exe", 0); err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	if _, err := registry.OpenKey(registry.CURRENT_USER, `Software\OneClassMesh\test-qos\ocm-testapp`, registry.QUERY_VALUE); err == nil {
		t.Fatal("policy key must be gone after rate=0")
	}
}
