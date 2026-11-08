// launcher is the OneClassMesh agent guardian (壹课管家启动器/守护者).
// Desktop shortcut target. Starts worker + core, watches them, restarts on
// crash with exponential backoff. Future Windows-service host (M1).
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"github.com/OneClassMesh/oneclassmesh-agent/internal/core"
	"github.com/OneClassMesh/oneclassmesh-agent/internal/core/logx"
)

const mutexName = `Local\OneClassMesh.Launcher`

type child struct {
	name      string
	args      []string
	cmd       *exec.Cmd
	running   bool
	mu        sync.Mutex
	failCount int
	nextDelay time.Duration
	lastStart time.Time
}

func (c *child) alive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

func (c *child) markExit() {
	c.mu.Lock()
	c.running = false
	c.failCount++
	if c.failCount >= 5 {
		c.nextDelay = 60 * time.Second
	} else if c.nextDelay < 30*time.Second {
		if c.nextDelay == 0 {
			c.nextDelay = time.Second
		} else {
			c.nextDelay *= 2
		}
	}
	dead := time.Since(c.lastStart) > 2*time.Minute
	if dead {
		c.failCount = 0
		c.nextDelay = 0
	}
	c.mu.Unlock()
	log.Printf("[launcher] %s exited (failCount=%d, next retry in %s)", c.name, c.failCount, c.nextDelay)
}

func (c *child) resetOK() {
	c.mu.Lock()
	c.failCount = 0
	c.nextDelay = 0
	c.mu.Unlock()
}

type supervisor struct {
	dir     string
	config  string
	withUI  bool
	mu      sync.Mutex
	stop    bool
	bin     string
	core    *child
	worker  *child
	ui      *exec.Cmd
}

func main() {
	config := flag.String("config", "", "path to config.toml (default: <exe dir>/conf/config.toml)")
	withUI := flag.Bool("with-ui", false, "also start the display app if present")
	flag.Parse()

	if err := logx.Setup("launcher"); err != nil {
		log.Printf("file log disabled: %v", err)
	}

	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("resolve executable failed: %v", err)
	}
	binDir := filepath.Dir(exePath)

	cfgPath := resolveConfigPath(*config, binDir)

	// single instance
	guard, err := createMutex()
	if err != nil {
		log.Fatalf("another launcher instance is running: %v", err)
	}
	defer windows.CloseHandle(guard)

	log.Printf("[launcher] started, dir=%s config=%s", binDir, cfgPath)

	sup := &supervisor{dir: binDir, config: cfgPath, withUI: *withUI, bin: binDir}
	sup.worker = &child{name: "worker.exe", args: []string{"-config", cfgPath}}
	sup.core = &child{name: "core.exe", args: []string{"-config", cfgPath}}

	// A fresh install ships no valid config.toml yet. Instead of letting
	// core crash-loop, wait until the config validates, then bring the
	// stack up (poll happens in watch()).
	ready := configValid(cfgPath)
	if ready {
		logIntegrity(binDir)
		sup.start(sup.worker)
		time.Sleep(500 * time.Millisecond)
		sup.start(sup.core)
	} else {
		log.Printf("[launcher] config not ready, waiting for a valid configuration")
	}

	if sup.withUI {
		sup.startUI()
	}

	go sup.watch()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Printf("[launcher] shutting down children")
	sup.stopAll()
}

// resolveConfigPath: -config flag > OCM_CONFIG env > config.toml next to the
// exe (dev layout) > <install>\conf\config.toml (packaged layout, exe in bin\).
func resolveConfigPath(configFlag, binDir string) string {
	if configFlag != "" {
		return configFlag
	}
	if v := os.Getenv("OCM_CONFIG"); v != "" {
		return v
	}
	if _, err := os.Stat(filepath.Join(binDir, "conf")); err == nil {
		return filepath.Join(binDir, "conf", "config.toml")
	}
	return filepath.Join(binDir, "..", "conf", "config.toml")
}

// configValid reports whether the config loads and validates.
func configValid(path string) bool {
	_, err := core.LoadConfig(path)
	return err == nil
}

func createMutex() (windows.Handle, error) {
	name, _ := syscall.UTF16PtrFromString(mutexName)
	h, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		return 0, err
	}
	event, waitErr := windows.WaitForSingleObject(h, 0)
	if waitErr != nil {
		return 0, waitErr
	}
	if event == uint32(windows.WAIT_TIMEOUT) {
		windows.CloseHandle(h)
		return 0, fmt.Errorf("mutex already held by another instance")
	}
	// WAIT_OBJECT_0 = acquired; WAIT_ABANDONED = acquired after a crashed
	// previous owner, still counts as ours.
	return h, nil
}

// logIntegrity records SHA256 of the critical binaries at boot.
func logIntegrity(binDir string) {
	for _, name := range []string{"core.exe", "worker.exe"} {
		p := filepath.Join(binDir, name)
		sum, err := fileSHA256(p)
		if err != nil {
			log.Printf("[launcher] integrity: %s unreadable: %v", name, err)
			continue
		}
		log.Printf("[launcher] integrity %s sha256=%s", name, sum[:16]+"...")
	}
}

func (s *supervisor) start(c *child) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return
	}
	path := filepath.Join(s.dir, c.name)
	if _, err := os.Stat(path); err != nil {
		log.Printf("[launcher] %s missing: %v", c.name, err)
		return
	}
	cmd := exec.Command(path, c.args...)
	// Children log to their own files via logx; keep them windowless even
	// when this launcher runs console-less (GUI subsystem).
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
	if err := cmd.Start(); err != nil {
		log.Printf("[launcher] start %s failed: %v", c.name, err)
		return
	}
	c.cmd = cmd
	c.running = true
	c.lastStart = time.Now()
	go func() {
		_ = cmd.Wait()
		c.markExit()
	}()
	log.Printf("[launcher] started %s (pid=%d)", c.name, cmd.Process.Pid)
}

func (s *supervisor) startUI() {
	// candidates: packaged layout (<install>\壹课管家\壹课管家.exe, launcher
	// lives in bin\) > dev repo layout > electron dev binary.
	candidates := []string{
		filepath.Join(s.dir, "..", "壹课管家", "壹课管家.exe"),
		filepath.Join(s.dir, "ui", "壹课管家.exe"),
		filepath.Join(s.dir, "cmd", "ui", "node_modules", "electron", "dist", "electron.exe"),
	}
	var uiPath string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			uiPath = p
			break
		}
	}
	if uiPath == "" {
		log.Printf("[launcher] ui not found, skip")
		return
	}
	go func() {
		cmd := exec.Command(uiPath)
		_ = cmd.Start()
		s.ui = cmd
	}()
}

func (s *supervisor) watch() {
	ready := configValid(s.config)
	loggedWaiting := false
	for {
		time.Sleep(3 * time.Second)
		s.mu.Lock()
		if s.stop {
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()

		if !ready {
			if !configValid(s.config) {
				if !loggedWaiting {
					log.Printf("[launcher] still waiting for a valid configuration at %s", s.config)
					loggedWaiting = true
				}
				continue
			}
			ready = true
			log.Printf("[launcher] configuration ready, starting children")
			logIntegrity(s.dir)
			s.start(s.worker)
			time.Sleep(500 * time.Millisecond)
			s.start(s.core)
			continue
		}

		for _, c := range []*child{s.worker, s.core} {
			if !c.alive() {
				c.mu.Lock()
				delay := c.nextDelay
				c.mu.Unlock()
				if delay > 0 {
					log.Printf("[launcher] waiting %s before restarting %s", delay, c.name)
					time.Sleep(delay)
				}
				s.start(c)
			}
		}
	}
}

func (s *supervisor) stopAll() {
	s.mu.Lock()
	s.stop = true
	s.mu.Unlock()
	for _, c := range []*child{s.core, s.worker} {
		c.mu.Lock()
		if c.running && c.cmd != nil && c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
			log.Printf("[launcher] killed %s", c.name)
		}
		c.mu.Unlock()
	}
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
