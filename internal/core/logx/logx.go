package logx

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

const (
	maxSize   = 200 * 1024 // 200KB per file
	maxBackup = 5
)

// rotator writes to <dir>/<name>.log, rotating at maxSize, keeping maxBackup.
type rotator struct {
	mu   sync.Mutex
	name string
	dir  string
	f    *os.File
	size int64
}

// Setup switches the standard logger to file+stdout with size rotation.
// Files land in %ProgramData%\oneclassmesh\logs\<name>.log — the data source
// for the display app log page.
func Setup(name string) error {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = "."
	}
	dir := filepath.Join(base, "oneclassmesh", "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	r := &rotator{name: name, dir: dir}
	if err := r.open(); err != nil {
		return err
	}
	log.SetOutput(io.MultiWriter(os.Stdout, r))
	log.SetFlags(log.LstdFlags)
	return nil
}

func (r *rotator) open() error {
	path := filepath.Join(r.dir, r.name+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	r.f = f
	r.size = 0
	if info, err := f.Stat(); err == nil {
		r.size = info.Size()
	}
	return nil
}

func (r *rotator) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.size+int64(len(p)) > maxSize {
		r.rotateLocked()
	}
	n, err := r.f.Write(p)
	r.size += int64(n)
	return n, err
}

func (r *rotator) rotateLocked() {
	_ = r.f.Close()
	base := filepath.Join(r.dir, r.name+".log")
	for i := maxBackup - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", base, i), fmt.Sprintf("%s.%d", base, i+1))
	}
	_ = os.Rename(base, base+".1")
	if err := r.open(); err != nil {
		log.SetOutput(os.Stdout)
	}
}
