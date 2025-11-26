//go:build linux

package process_linux

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

type Process struct {
	PID  int
	Name string // best-effort: comm or exe basename
}

// ListByName returns all processes whose comm or exe basename equals name.
// name match is case-sensitive (like pidof). Use strings.EqualFold yourself if you want case-insensitive.
func ListByName(name string) ([]*Process, error) {
	if name == "" {
		return nil, errors.New("empty name")
	}

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("read /proc: %w", err)
	}

	selfPID := os.Getpid()
	var out []*Process

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue // not a PID dir
		}
		if pid == selfPID {
			continue // skip ourselves
		}

		comm, _ := os.ReadFile(filepath.Join("/proc", e.Name(), "comm"))
		comm = bytesTrimNL(comm)
		if string(comm) == name {
			out = append(out, &Process{PID: pid, Name: string(comm)})
			continue
		}

		// Resolve /proc/<pid>/exe symlink; may fail if zombie or permission
		exe, _ := os.Readlink(filepath.Join("/proc", e.Name(), "exe"))
		if exe != "" && filepath.Base(exe) == name {
			out = append(out, &Process{PID: pid, Name: filepath.Base(exe)})
			continue
		}
	}

	return out, nil
}

// OneByName returns the first match for name (lowest PID), or os.ErrNotExist if none.
func OneByName(name string) (*Process, error) {
	ps, err := ListByName(name)
	if err != nil {
		return nil, err
	}
	if len(ps) == 0 {
		return nil, os.ErrNotExist
	}
	// pick the lowest PID for determinism
	minIdx := 0
	for i := 1; i < len(ps); i++ {
		if ps[i].PID < ps[minIdx].PID {
			minIdx = i
		}
	}
	return ps[minIdx], nil
}

func (p *Process) Signal(sig syscall.Signal) error {
	if p == nil {
		return errors.New("nil Process")
	}
	// Use raw syscall kill so it works for non-child processes.
	if err := syscall.Kill(p.PID, sig); err != nil {
		// ESRCH means it's already gone — treat as success for idempotency?
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
	return nil
}

func (p *Process) Kill() error {
	return p.Signal(syscall.SIGKILL)
}

// WaitClose waits until the PID disappears from /proc or until timeout.
// Returns true if the process exited within the timeout.
func (p *Process) WaitClose(timeout time.Duration) bool {
	if p == nil {
		return true
	}
	deadline := time.Now().Add(timeout)
	tick := 25 * time.Millisecond
	for {
		if !procExists(p.PID) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(tick)
		// Exponential-ish backoff up to 250ms to reduce pressure on /proc
		if tick < 250*time.Millisecond {
			tick += 10 * time.Millisecond
		}
	}
}

// ----- helpers -----

func procExists(pid int) bool {
	// Fast path: stat /proc/<pid>
	_, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid)))
	if err == nil {
		return true
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	// For transient errors (permission, EIO): fall back to kill 0
	return syscall.Kill(pid, 0) == nil
}

func bytesTrimNL(b []byte) []byte {
	// Trim trailing '\n' if present (comm has a newline).
	for len(b) > 0 {
		switch b[len(b)-1] {
		case '\n', '\r', ' ', '\t':
			b = b[:len(b)-1]
		default:
			return b
		}
	}
	return b
}
