//go:build linux

package process_linux

import (
	"fmt"

	"gomem/process"
)

// LinuxProcessHelper implements the process.ProcessHelper interface
type LinuxProcessHelper struct {
	Finder process.ProcessFinder
}

// NewHelper creates a new LinuxProcessHelper
func NewHelper() process.ProcessHelper {
	return &LinuxProcessHelper{
		Finder: NewProcessFinder(),
	}
}

// New creates a new Process instance
func (h *LinuxProcessHelper) New() process.Process {
	return New()
}

// NewWithPID creates a new Process instance and opens it with the given PID
func (h *LinuxProcessHelper) NewWithPID(pid process.ProcessID) (process.Process, error) {
	return NewWithPID(pid)
}

// OpenProcessByName opens a process by its name (returns the first match)
func (h *LinuxProcessHelper) OpenProcessByName(name string) (process.Process, error) {
	processes, err := h.Finder.FindProcessByName(name)
	if err != nil {
		return nil, err
	}

	if len(processes) == 0 {
		return nil, fmt.Errorf("no process found with name '%s'", name)
	}

	// Return the first matching process
	return NewWithPID(processes[0].PID)
}

// OpenProcessByPattern opens a process by its name pattern (returns the first match)
func (h *LinuxProcessHelper) OpenProcessByPattern(pattern string) (process.Process, error) {
	processes, err := h.Finder.FindProcessByNamePattern(pattern)
	if err != nil {
		return nil, err
	}

	if len(processes) == 0 {
		return nil, fmt.Errorf("no process found matching pattern '%s'", pattern)
	}

	// Return the first matching process
	return NewWithPID(processes[0].PID)
}

// OpenProcessByCommandLine opens a process by searching for a command line argument
func (h *LinuxProcessHelper) OpenProcessByCommandLine(arg string) (process.Process, error) {
	processes, err := h.Finder.FindProcessByCommandLine(arg)
	if err != nil {
		return nil, err
	}

	if len(processes) == 0 {
		return nil, fmt.Errorf("no process found with command line argument '%s'", arg)
	}

	// Return the first matching process
	return NewWithPID(processes[0].PID)
}

// OpenProcessByCommandLinePattern opens a process by matching command line arguments with a pattern
func (h *LinuxProcessHelper) OpenProcessByCommandLinePattern(pattern string) (process.Process, error) {
	processes, err := h.Finder.FindProcessByCommandLinePattern(pattern)
	if err != nil {
		return nil, err
	}

	if len(processes) == 0 {
		return nil, fmt.Errorf("no process found with command line matching pattern '%s'", pattern)
	}

	// Return the first matching process
	return NewWithPID(processes[0].PID)
}