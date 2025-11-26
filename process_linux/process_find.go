//go:build linux

package process_linux

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gomem/process"
)

// LinuxProcessFinder implements the process.ProcessFinder interface
type LinuxProcessFinder struct{}

// NewProcessFinder creates a new LinuxProcessFinder
func NewProcessFinder() process.ProcessFinder {
	return &LinuxProcessFinder{}
}

// FindProcess finds a process by name and returns its PID
// This is kept for backward compatibility
func FindProcess(name string) (process.ProcessID, error) {
	finder := NewProcessFinder()
	processes, err := finder.FindProcessByName(name)
	if err != nil {
		return 0, err
	}

	if len(processes) == 0 {
		return 0, fmt.Errorf("no process found with name '%s'", name)
	}

	return processes[0].PID, nil
}

// FindProcessByPID finds a process by its PID
func (f *LinuxProcessFinder) FindProcessByPID(pid process.ProcessID) (*process.ProcessInfo, error) {
	procPath := fmt.Sprintf("/proc/%d", pid)

	// Check if the process exists
	if _, err := os.Stat(procPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("process with PID %d does not exist", pid)
	}

	return getProcessInfo(pid)
}

// FindProcessByName finds processes by their name (exact match)
func (f *LinuxProcessFinder) FindProcessByName(name string) ([]process.ProcessInfo, error) {
	return findProcessesByNamePattern("^" + regexp.QuoteMeta(name) + "$")
}

// FindProcessByNamePattern finds processes by their name (pattern match)
func (f *LinuxProcessFinder) FindProcessByNamePattern(pattern string) ([]process.ProcessInfo, error) {
	return findProcessesByNamePattern(pattern)
}

// FindAllProcesses returns information about all running processes
func (f *LinuxProcessFinder) FindAllProcesses() ([]process.ProcessInfo, error) {
	return findProcessesByNamePattern(".*")
}

// Helper function to find processes by name pattern
func findProcessesByNamePattern(pattern string) ([]process.ProcessInfo, error) {
	// Compile the regex pattern
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}

	// List all directories in /proc that are numbers (PIDs)
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc: %w", err)
	}

	var results []process.ProcessInfo

	for _, entry := range entries {
		// Check if the entry is a directory and its name is a number (PID)
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			// Not a PID directory
			continue
		}

		procID := process.ProcessID(pid)

		// Get process info
		info, err := getProcessInfo(procID)
		if err != nil {
			// Process may have terminated while we were reading
			continue
		}

		// Check if the process name matches the pattern
		if re.MatchString(info.Name) {
			results = append(results, *info)
		}
	}

	return results, nil
}

// Helper function to get process information
func getProcessInfo(pid process.ProcessID) (*process.ProcessInfo, error) {
	procPath := fmt.Sprintf("/proc/%d", pid)

	// Read process name from /proc/<pid>/comm
	nameBytes, err := os.ReadFile(filepath.Join(procPath, "comm"))
	if err != nil {
		return nil, fmt.Errorf("failed to read process name: %w", err)
	}
	name := strings.TrimSpace(string(nameBytes))

	// Read executable path from /proc/<pid>/exe symlink
	exe, err := os.Readlink(filepath.Join(procPath, "exe"))
	if err != nil {
		// Some processes don't have an exe (e.g., kernel threads)
		exe = ""
	}

	// Read the command line from /proc/<pid>/cmdline
	cmdlineBytes, err := os.ReadFile(filepath.Join(procPath, "cmdline"))
	if err != nil {
		return nil, fmt.Errorf("failed to read process cmdline: %w", err)
	}

	// Split the command line on NULL bytes
	var cmdline []string
	if len(cmdlineBytes) > 0 {
		// Remove the trailing NULL byte
		if cmdlineBytes[len(cmdlineBytes)-1] == 0 {
			cmdlineBytes = cmdlineBytes[:len(cmdlineBytes)-1]
		}

		// Split by NULL bytes
		for _, arg := range bytes.Split(cmdlineBytes, []byte{0}) {
			cmdline = append(cmdline, string(arg))
		}
	}

	// Get process status from /proc/<pid>/status
	var (
		ppid    process.ProcessID    = 0
		state   process.ProcessState = ""
		user    string               = ""
		threads int                  = 0
		memory  uint64               = 0
	)

	// Read the status file
	statusBytes, err := os.ReadFile(filepath.Join(procPath, "status"))
	if err == nil {
		// Parse status information
		statusLines := strings.Split(string(statusBytes), "\n")
		for _, line := range statusLines {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			case "PPid":
				if ppidVal, err := strconv.Atoi(value); err == nil {
					ppid = process.ProcessID(ppidVal)
				}
			case "State":
				if len(value) > 0 {
					state = process.ProcessState(value[0:1]) // First character is the state code
				}
			case "Uid":
				// Extract the effective UID
				uidParts := strings.Fields(value)
				if len(uidParts) >= 2 {
					user = uidParts[1] // Effective UID
				}
			case "Threads":
				if threadsVal, err := strconv.Atoi(value); err == nil {
					threads = threadsVal
				}
			case "VmRSS":
				// Extract memory usage (format: "1234 kB")
				memParts := strings.Fields(value)
				if len(memParts) >= 1 {
					if memVal, err := strconv.ParseUint(memParts[0], 10, 64); err == nil {
						if len(memParts) > 1 && memParts[1] == "kB" {
							memory = memVal * 1024 // Convert kB to bytes
						} else {
							memory = memVal
						}
					}
				}
			}
		}
	}

	// Get username from UID
	if user != "" {
		// This is simplified - in a real implementation, you would look up
		// the username from /etc/passwd or use a syscall
		user = "uid_" + user // Placeholder
	}

	return &process.ProcessInfo{
		PID:     pid,
		PPID:    ppid,
		Name:    name,
		Exe:     exe,
		Cmdline: cmdline,
		State:   state,
		User:    user,
		Threads: threads,
		Memory:  memory,
	}, nil
}

// FindProcessByCommandLine finds processes that have a specific argument in their command line
func (f *LinuxProcessFinder) FindProcessByCommandLine(arg string) ([]process.ProcessInfo, error) {
	return findProcessesByCommandLinePattern(regexp.QuoteMeta(arg))
}

// FindProcessByCommandLinePattern finds processes with command line arguments matching a pattern
func (f *LinuxProcessFinder) FindProcessByCommandLinePattern(pattern string) ([]process.ProcessInfo, error) {
	return findProcessesByCommandLinePattern(pattern)
}

// Helper function to find processes by command line pattern
func findProcessesByCommandLinePattern(pattern string) ([]process.ProcessInfo, error) {
	// Compile the regex pattern
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}

	finder := NewProcessFinder()
	// Get all processes
	allProcesses, err := finder.FindAllProcesses()
	if err != nil {
		return nil, err
	}

	var results []process.ProcessInfo

	for _, proc := range allProcesses {
		// Check if any command line argument matches the pattern
		for _, arg := range proc.Cmdline {
			if re.MatchString(arg) {
				results = append(results, proc)
				break // Found a match in this process, no need to check other args
			}
		}
	}

	return results, nil
}

// FindChildProcesses finds all child processes of a given PID
func (f *LinuxProcessFinder) FindChildProcesses(parentPID process.ProcessID) ([]process.ProcessInfo, error) {
	// Get all processes
	allProcesses, err := f.FindAllProcesses()
	if err != nil {
		return nil, err
	}

	var children []process.ProcessInfo

	// Filter processes with the specified parent PID
	for _, proc := range allProcesses {
		if proc.PPID == parentPID {
			children = append(children, proc)
		}
	}

	return children, nil
}

// FindDescendantProcesses finds all descendant processes (children, grandchildren, etc.) of a given PID
func (f *LinuxProcessFinder) FindDescendantProcesses(rootPID process.ProcessID) ([]process.ProcessInfo, error) {
	// Get all processes
	allProcesses, err := f.FindAllProcesses()
	if err != nil {
		return nil, err
	}

	// Build a map of parent-to-children relationships
	childrenMap := make(map[process.ProcessID][]process.ProcessID)
	processMap := make(map[process.ProcessID]process.ProcessInfo)

	for _, proc := range allProcesses {
		// Add to process map for easy lookup
		processMap[proc.PID] = proc

		// Add to parent-child relationship map
		if _, exists := childrenMap[proc.PPID]; !exists {
			childrenMap[proc.PPID] = []process.ProcessID{}
		}
		childrenMap[proc.PPID] = append(childrenMap[proc.PPID], proc.PID)
	}

	// Collect all descendants using BFS (breadth-first search)
	var descendants []process.ProcessInfo
	var queue []process.ProcessID
	visited := make(map[process.ProcessID]bool)

	// Start with the children of the root process
	if directChildren, exists := childrenMap[rootPID]; exists {
		queue = append(queue, directChildren...)
	}

	// Process the queue
	for len(queue) > 0 {
		// Dequeue a process
		pid := queue[0]
		queue = queue[1:]

		// Skip if already visited
		if visited[pid] {
			continue
		}

		// Mark as visited
		visited[pid] = true

		// Add to descendants
		if proc, exists := processMap[pid]; exists {
			descendants = append(descendants, proc)

			// Add its children to the queue
			if children, exists := childrenMap[pid]; exists {
				queue = append(queue, children...)
			}
		}
	}

	return descendants, nil
}

// GetProcessTree returns a tree-like representation of processes starting from a root PID
func (f *LinuxProcessFinder) GetProcessTree(rootPID process.ProcessID) (*process.ProcessTreeNode, error) {
	// Check if the root process exists
	rootProcess, err := f.FindProcessByPID(rootPID)
	if err != nil {
		return nil, err
	}

	// Get all processes
	allProcesses, err := f.FindAllProcesses()
	if err != nil {
		return nil, err
	}

	// Build a map of parent-to-children relationships
	childrenMap := make(map[process.ProcessID][]process.ProcessID)
	processMap := make(map[process.ProcessID]process.ProcessInfo)

	for _, proc := range allProcesses {
		// Add to process map for easy lookup
		processMap[proc.PID] = proc

		// Add to parent-child relationship map
		if _, exists := childrenMap[proc.PPID]; !exists {
			childrenMap[proc.PPID] = []process.ProcessID{}
		}
		childrenMap[proc.PPID] = append(childrenMap[proc.PPID], proc.PID)
	}

	// Build the tree recursively
	tree := buildProcessTree(*rootProcess, childrenMap, processMap)

	return tree, nil
}

// Helper function to build a process tree recursively
func buildProcessTree(procInfo process.ProcessInfo, childrenMap map[process.ProcessID][]process.ProcessID, processMap map[process.ProcessID]process.ProcessInfo) *process.ProcessTreeNode {
	node := &process.ProcessTreeNode{
		Process:  procInfo,
		Children: []*process.ProcessTreeNode{},
	}

	// Add children nodes
	if childPIDs, exists := childrenMap[procInfo.PID]; exists {
		for _, childPID := range childPIDs {
			if childProc, exists := processMap[childPID]; exists {
				childNode := buildProcessTree(childProc, childrenMap, processMap)
				node.Children = append(node.Children, childNode)
			}
		}
	}

	return node
}
