package process

// ProcessFinder defines operations for discovering processes and their relationships
type ProcessFinder interface {
	// FindProcessByPID finds a process by its PID
	FindProcessByPID(pid ProcessID) (*ProcessInfo, error)

	// FindProcessByName finds processes by their name (exact match)
	FindProcessByName(name string) ([]ProcessInfo, error)

	// FindProcessByNamePattern finds processes by their name (pattern match)
	FindProcessByNamePattern(pattern string) ([]ProcessInfo, error)

	// FindAllProcesses returns information about all running processes
	FindAllProcesses() ([]ProcessInfo, error)

	// FindProcessByCommandLine finds processes that have a specific argument in their command line
	FindProcessByCommandLine(arg string) ([]ProcessInfo, error)

	// FindProcessByCommandLinePattern finds processes with command line arguments matching a pattern
	FindProcessByCommandLinePattern(pattern string) ([]ProcessInfo, error)

	// Process hierarchy operations
	ProcessHierarchy
}

// ProcessHierarchy defines operations for working with process relationships
type ProcessHierarchy interface {
	// FindChildProcesses finds all child processes of a given PID
	FindChildProcesses(parentPID ProcessID) ([]ProcessInfo, error)

	// FindDescendantProcesses finds all descendant processes (children, grandchildren, etc.) of a given PID
	FindDescendantProcesses(rootPID ProcessID) ([]ProcessInfo, error)

	// GetProcessTree returns a tree-like representation of processes starting from a root PID
	GetProcessTree(rootPID ProcessID) (*ProcessTreeNode, error)
}
