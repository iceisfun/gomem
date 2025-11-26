//go:build linux

package process_linux

import (
	"fmt"
	"unsafe"

	"gomem/process"

	"golang.org/x/sys/unix"
)

// process_vm_writev uses the process_vm_writev syscall to write memory to another process
func process_vm_writev(
	pid process.ProcessID,
	localBuf []byte,
	localBufSize process.ProcessMemorySize,
	remoteAddr process.ProcessMemoryAddress,
	bytesToWrite process.ProcessMemorySize,
) (int, error) {
	// Create iovec for local buffer
	localIov := unix.Iovec{
		Base: &localBuf[0],
		Len:  uint64(localBufSize),
	}

	// Create iovec for remote buffer
	remoteIov := unix.RemoteIovec{
		Base: uintptr(remoteAddr),
		Len:  int(bytesToWrite),
	}

	// Call process_vm_writev
	n, _, errno := unix.Syscall6(
		unix.SYS_PROCESS_VM_WRITEV,
		uintptr(pid),                      // Remote process PID
		uintptr(unsafe.Pointer(&localIov)), // Local iovec
		uintptr(1),                        // Number of local iovecs
		uintptr(unsafe.Pointer(&remoteIov)), // Remote iovec
		uintptr(1),                        // Number of remote iovecs
		uintptr(0),                        // Flags (reserved for future use)
	)

	// Check for errors
	if errno != 0 {
		return 0, fmt.Errorf("process_vm_writev failed: %s (errno: %d)", errno.Error(), errno)
	}

	return int(n), nil
}

// WriteMemory writes data to the process memory at the specified address
func (p *LinuxProcess) WriteMemory(addr process.ProcessMemoryAddress, data []byte) error {
	// Acquire the lock for checking state and permissions
	p.mu.Lock()
	
	if p.pid == 0 {
		p.mu.Unlock()
		return fmt.Errorf("process not opened")
	}

	// Make a copy of the PID to avoid race conditions
	pid := p.pid

	// Validate the address
	if !p.isValidAddressInternal(addr) {
		p.mu.Unlock()
		return fmt.Errorf("invalid memory address %x", addr)
	}

	// Check permissions for writing (must be writeable)
	region, isWritable := p.getMemoryRegionForAddress(addr)
	
	// Release the lock before the system call
	p.mu.Unlock()
	
	if region == nil {
		return fmt.Errorf("memory region not found for address %x", addr)
	}

	if !isWritable {
		return fmt.Errorf("memory region at %x is not writable", addr)
	}

	size := process.ProcessMemorySize(len(data))

	// Create a copy of the data to avoid potential modification during the write
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	// Use process_vm_writev to write memory (without holding the lock)
	written, err := process_vm_writev(
		pid,
		dataCopy,
		size,
		addr,
		size,
	)

	if err != nil {
		return fmt.Errorf("failed to write process memory: %w", err)
	}

	if written != len(data) {
		return fmt.Errorf("only wrote %d of %d bytes", written, len(data))
	}

	return nil
}