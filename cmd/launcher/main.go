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

	cfgPath := *config
	if cfgPath == "" {
		cfgPath = filepath.Join(binDir, "conf", "config.toml")
	}

	// single instance
	guard, err := createMutex()
	if err != nil {
		log.Fatalf("another launcher instance is running: %v", err)
	}
	defer windows.CloseHandle(guard)

	log.Printf("[launcher] started, dir=%s config=%s", binDir, cfgPath)
	logIntegrity(binDir)

	sup := &supervisor{dir: binDir, config: cfgPath, withUI: *withUI, bin: binDir}
	sup.worker = &child{name: "worker.exe", args: []string{"-config", cfgPath}}
	sup.core = &child{name: "core.exe", args: []string{"-config", cfgPath}}

	sup.start(sup.worker)
	time.Sleep(500 * time.Millisecond)
	sup.start(sup.core)

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

func createMutex() (windows.Handle, error) {
	name, _ := syscall.UTF16PtrFromString(mutexName)
	h, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		return 0, err
	}
	event, waitErr := windows.WaitForSingleObject(h, 0)
	if waitErr != nil || event == windows.WAIT_OBJECT_0+1 {
		return 0, fmt.Errorf("mutex already held by another instance")
	}
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
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
	uiPath := filepath.Join(s.dir, "ui", "壹课管家.exe")
	if _, err := os.Stat(uiPath); err != nil {
		p := filepath.Join(s.dir, "cmd", "ui", "node_modules", "electron", "dist", "electron.exe")
		if _, err2 := os.Stat(p); err2 != nil {
			log.Printf("[launcher] ui not found, skip")
			return
		}
		uiPath = p
	}
	go func() {
		cmd := exec.Command(uiPath)
		_ = cmd.Start()
		s.ui = cmd
	}()
}

func (s *supervisor) watch() {
	for {
		time.Sleep(3 * time.Second)
		s.mu.Lock()
		if s.stop {
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()

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
