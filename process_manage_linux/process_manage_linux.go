//go:build linux

package process_manage_linux

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Process represents a system process
type Process struct {
	PID     int    `json:"pid"`
	PPID    int    `json:"ppid"`
	Name    string `json:"name"`
	State   string `json:"state"`
	VmSize  int64  `json:"vm_size"` // Virtual memory size in KB
	VmRSS   int64  `json:"vm_rss"`  // Resident set size in KB
	Threads int    `json:"threads"`
	Cmdline string `json:"cmdline"`
}

// ProcessManager handles process operations
type ProcessManager struct{}

// NewProcessManager creates a new ProcessManager instance
func NewProcessManager() *ProcessManager {
	return &ProcessManager{}
}

// ListProcesses returns a list of all running processes
func (pm *ProcessManager) ListProcesses() ([]Process, error) {
	var processes []Process

	// Read /proc directory
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if directory name is a number (PID)
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue // Skip non-numeric directories
		}

		process, err := pm.getProcessInfo(pid)
		if err != nil {
			// Process might have disappeared, skip it
			continue
		}

		processes = append(processes, process)
	}

	return processes, nil
}

// GetProcess returns information about a specific process
func (pm *ProcessManager) GetProcess(pid int) (Process, error) {
	return pm.getProcessInfo(pid)
}

// FindProcessesByName finds processes matching the given name
func (pm *ProcessManager) FindProcessesByName(name string) ([]Process, error) {
	processes, err := pm.ListProcesses()
	if err != nil {
		return nil, err
	}

	var matches []Process
	for _, proc := range processes {
		if strings.Contains(proc.Name, name) {
			matches = append(matches, proc)
		}
	}

	return matches, nil
}

// KillProcess sends SIGKILL to a process
func (pm *ProcessManager) KillProcess(pid int) error {
	return pm.SendSignal(pid, syscall.SIGKILL)
}

// TerminateProcess sends SIGTERM to a process
func (pm *ProcessManager) TerminateProcess(pid int) error {
	return pm.SendSignal(pid, syscall.SIGTERM)
}

// SendSignal sends a specific signal to a process
func (pm *ProcessManager) SendSignal(pid int, sig syscall.Signal) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	err = process.Signal(sig)
	if err != nil {
		return fmt.Errorf("failed to send signal %v to process %d: %w", sig, pid, err)
	}

	return nil
}

// ProcessExists checks if a process with the given PID exists
func (pm *ProcessManager) ProcessExists(pid int) bool {
	_, err := pm.getProcessInfo(pid)
	return err == nil
}

// getProcessInfo reads process information from /proc/[pid]/
func (pm *ProcessManager) getProcessInfo(pid int) (Process, error) {
	proc := Process{PID: pid}

	// Read /proc/[pid]/stat for basic info
	statPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	statData, err := os.ReadFile(statPath)
	if err != nil {
		return proc, fmt.Errorf("failed to read %s: %w", statPath, err)
	}

	err = pm.parseStatFile(string(statData), &proc)
	if err != nil {
		return proc, fmt.Errorf("failed to parse stat file: %w", err)
	}

	// Read /proc/[pid]/status for additional info
	statusPath := filepath.Join("/proc", strconv.Itoa(pid), "status")
	statusData, err := os.ReadFile(statusPath)
	if err != nil {
		// Status file might not be readable, continue without it
	} else {
		pm.parseStatusFile(string(statusData), &proc)
	}

	// Read /proc/[pid]/cmdline for command line
	cmdlinePath := filepath.Join("/proc", strconv.Itoa(pid), "cmdline")
	cmdlineData, err := os.ReadFile(cmdlinePath)
	if err == nil {
		// Replace null bytes with spaces
		cmdline := strings.ReplaceAll(string(cmdlineData), "\x00", " ")
		proc.Cmdline = strings.TrimSpace(cmdline)
	}

	return proc, nil
}

// parseStatFile parses /proc/[pid]/stat file
func (pm *ProcessManager) parseStatFile(data string, proc *Process) error {
	fields := strings.Fields(data)
	if len(fields) < 24 {
		return fmt.Errorf("invalid stat file format")
	}

	// Parse PPID (field 4, 0-indexed)
	if ppid, err := strconv.Atoi(fields[3]); err == nil {
		proc.PPID = ppid
	}

	// Parse state (field 3, 0-indexed)
	proc.State = fields[2]

	// Parse number of threads (field 20, 0-indexed)
	if threads, err := strconv.Atoi(fields[19]); err == nil {
		proc.Threads = threads
	}

	// Extract process name from field 2 (remove parentheses)
	name := fields[1]
	if len(name) >= 2 && name[0] == '(' && name[len(name)-1] == ')' {
		proc.Name = name[1 : len(name)-1]
	} else {
		proc.Name = name
	}

	return nil
}

// parseStatusFile parses /proc/[pid]/status file for memory info
func (pm *ProcessManager) parseStatusFile(data string, proc *Process) {
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		switch parts[0] {
		case "VmSize:":
			if len(parts) >= 2 {
				if size, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					proc.VmSize = size
				}
			}
		case "VmRSS:":
			if len(parts) >= 2 {
				if rss, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					proc.VmRSS = rss
				}
			}
		case "Name:":
			if len(parts) >= 2 {
				proc.Name = parts[1]
			}
		}
	}
}

// GetProcessTree builds a tree of processes (children under parents)
func (pm *ProcessManager) GetProcessTree() (map[int][]Process, error) {
	processes, err := pm.ListProcesses()
	if err != nil {
		return nil, err
	}

	tree := make(map[int][]Process)
	for _, proc := range processes {
		tree[proc.PPID] = append(tree[proc.PPID], proc)
	}

	return tree, nil
}

// GetChildren returns all child processes of a given PID
func (pm *ProcessManager) GetChildren(pid int) ([]Process, error) {
	tree, err := pm.GetProcessTree()
	if err != nil {
		return nil, err
	}

	return tree[pid], nil
}

// KillProcessTree kills a process and all its children
func (pm *ProcessManager) KillProcessTree(pid int) error {
	children, err := pm.GetChildren(pid)
	if err != nil {
		return err
	}

	// Kill children first
	for _, child := range children {
		if err := pm.KillProcessTree(child.PID); err != nil {
			// Log error but continue with other children
			fmt.Printf("Failed to kill child process %d: %v\n", child.PID, err)
		}
	}

	// Kill the parent process
	return pm.KillProcess(pid)
}
