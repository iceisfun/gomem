//go:build linux

package process_linux

import (
	"fmt"
	"unsafe"

	"gomem/process"

	"golang.org/x/sys/unix"
)

// process_vm_readv uses the process_vm_readv syscall to read memory from another process
func process_vm_readv(
	pid process.ProcessID,
	localBuf []byte,
	localBufSize process.ProcessMemorySize,
	remoteAddr process.ProcessMemoryAddress,
	bytesToRead process.ProcessMemorySize,
) ([]byte, error) {
	// Allocate a buffer if one wasn't provided
	if localBuf == nil || len(localBuf) != int(bytesToRead) {
		localBuf = make([]byte, bytesToRead)
	}

	// Create iovec for local buffer
	localIov := unix.Iovec{
		Base: &localBuf[0],
		Len:  uint64(bytesToRead),
	}

	// Create iovec for remote buffer
	remoteIov := unix.RemoteIovec{
		Base: uintptr(remoteAddr),
		Len:  int(bytesToRead),
	}

	// Call process_vm_readv
	n, _, errno := unix.Syscall6(
		unix.SYS_PROCESS_VM_READV,
		uintptr(pid),                        // Remote process PID
		uintptr(unsafe.Pointer(&localIov)),  // Local iovec
		uintptr(1),                          // Number of local iovecs
		uintptr(unsafe.Pointer(&remoteIov)), // Remote iovec
		uintptr(1),                          // Number of remote iovecs
		uintptr(0),                          // Flags (reserved for future use)
	)

	// Check for errors
	if errno != 0 {
		return nil, fmt.Errorf("process_vm_readv failed: %s (errno: %d)", errno.Error(), errno)
	}

	// Check if we read the expected number of bytes
	if int(n) != int(bytesToRead) {
		return localBuf[:n], fmt.Errorf("partial read: %d of %d bytes", n, bytesToRead)
	}

	return localBuf, nil
}

// ReadMemory reads memory from the process at the specified address
func (p *LinuxProcess) ReadMemory(addr process.ProcessMemoryAddress, size process.ProcessMemorySize) ([]byte, error) {
	// First, acquire the lock to check pid and validate the address
	pid := p.pid
	if pid == 0 {
		return nil, process.ErrProcessNotOpen
	}

	// Make a copy of the PID and check address validity
	p.mu.Lock()
	valid := p.isValidAddressInternal(addr)
	// Release the lock before the system call
	p.mu.Unlock()

	if !valid {
		return nil, process.ErrAddressNotMapped
	}

	// Use process_vm_readv to read memory without holding the lock
	data, err := process_vm_readv(
		pid,
		nil, // Local buffer will be allocated in the function
		size,
		addr,
		size,
	)

	if err != nil {
		return nil, fmt.Errorf("process_vm_readv: failed to read process memory: %w", err)
	}

	return data, nil
}
