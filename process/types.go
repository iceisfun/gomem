package process

// ProcessID represents a unique identifier for a process
type ProcessID int

// ProcessInfo contains basic information about a process
type ProcessInfo struct {
	PID     ProcessID    // Process ID
	PPID    ProcessID    // Parent Process ID
	Name    string       // Process name from /proc/[pid]/comm
	Exe     string       // Path to the executable
	Cmdline []string     // Command line arguments
	State   ProcessState // Process state (R, S, D, Z, etc.)
	User    string       // User running the process
	Threads int          // Number of threads
	Memory  uint64       // Resident Set Size (memory usage in bytes)
}

// ProcessTreeNode represents a node in a process tree
type ProcessTreeNode struct {
	Process  ProcessInfo
	Children []*ProcessTreeNode
}
