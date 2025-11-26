// Package common provides interfaces and types for process manipulation
package process

import "errors"

// This file maintains the original API surface for backward compatibility
// It re-exports types and interfaces that have been moved to separate files

// All the original types and interfaces are now defined in:
// - types.go: ProcessID, ProcessInfo, ProcessTreeNode
// - process_state.go: ProcessState constants
// - memory_types.go: ProcessMemoryAddress, ProcessMemorySize, AOB
// - process_interface.go: Process interface
// - process_finder.go: ProcessFinder interface
// - process_helper.go: ProcessHelper interface

var (
	// ErrAddressNotMapped is returned when a memory address is not found within any mapped region of a process.
	ErrAddressNotMapped = errors.New("address not mapped")

	// ErrProcessNotOpen is returned when an operation requiring an open process is attempted
	// before the process has been successfully opened or after it has been closed.
	ErrProcessNotOpen = errors.New("process not open")

	ErrInvalidPointer = errors.New("invalid pointer read")
)

type ReadBlobsResult struct {
	Address ProcessMemoryAddress
	Blob    ProcessReadOffset
	Err     error
}
