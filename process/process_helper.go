package process

// ProcessHelper provides utility functions for working with processes
type ProcessHelper interface {
	// NewProcess creates a new Process instance
	New() Process

	// NewWithPID creates a new Process instance and opens it with the given PID
	NewWithPID(pid ProcessID) (Process, error)

	// Process opening operations
	ProcessOpener
}

// ProcessOpener defines operations for opening processes with various search criteria
type ProcessOpener interface {
	// OpenProcessByName opens a process by its name (returns the first match)
	OpenProcessByName(name string) (Process, error)

	// OpenProcessByPattern opens a process by its name pattern (returns the first match)
	OpenProcessByPattern(pattern string) (Process, error)

	// OpenProcessByCommandLine opens a process by searching for a command line argument
	OpenProcessByCommandLine(arg string) (Process, error)

	// OpenProcessByCommandLinePattern opens a process by matching command line arguments with a pattern
	OpenProcessByCommandLinePattern(pattern string) (Process, error)
}
