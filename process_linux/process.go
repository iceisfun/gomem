//go:build linux

package process_linux

import (
	"fmt"
	"os"
	"sort"
	"sync"

	"gomem/process"
	"gomem/process/memory_map"

	"github.com/Moonlight-Companies/gologger/coloransi"
	"github.com/Moonlight-Companies/gologger/logger"
)

var lastOpenProcess *LinuxProcess = nil

func LastOpenProcess() process.Process {
	return lastOpenProcess
}

// LinuxProcess implements the process.Process interface for Linux systems
type LinuxProcess struct {
	pid process.ProcessID
	log *logger.Logger
	mm  []memory_map.MemoryMapItem
	mu  sync.Mutex
}

// New creates a new LinuxProcess instance
func New() process.Process {
	result := &LinuxProcess{
		log: logger.NewLogger(coloransi.Color(coloransi.Red, coloransi.ColorOrange, "process-not-open")),
	}

	lastOpenProcess = result

	return result
}

// NewWithPID creates a new LinuxProcess instance and opens it with the given PID
func NewWithPID(pid process.ProcessID) (process.Process, error) {
	p := &LinuxProcess{}
	err := p.Open(pid)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *LinuxProcess) Open(pid process.ProcessID) error {
	// Check if process exists
	procPath := fmt.Sprintf("/proc/%d", pid)
	if _, err := os.Stat(procPath); os.IsNotExist(err) {
		return fmt.Errorf("process with PID %d does not exist", pid)
	}

	p.mu.Lock()
	p.pid = pid
	p.log = logger.NewLogger(coloransi.Color(coloransi.ColorPurple, coloransi.ColorOrange, fmt.Sprintf("process-%d", pid)))
	p.mu.Unlock()

	// Initialize memory map - call without holding the lock to avoid deadlock
	if err := p.UpdateMemoryMap(); err != nil {
		return fmt.Errorf("failed to initialize memory map: %w", err)
	}

	p.log.Infoln("Process opened")

	return nil
}

func (p *LinuxProcess) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.log.Infoln("Closing process")

	// Reset process state
	p.pid = 0
	p.mm = nil

	p.log = logger.NewLogger(coloransi.Color(coloransi.Red, coloransi.ColorOrange, "process-not-open"))

	p.log.Infoln("Process closed")

	return nil
}

// GetPID returns the process ID
func (p *LinuxProcess) GetPID() process.ProcessID {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pid
}

func (p *LinuxProcess) UpdateMemoryMap() error {
	// First get the pid value without holding the lock for long
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pid == 0 {
		return fmt.Errorf("process not opened")
	}
	pid := p.pid

	// Read memory map without holding the lock
	linuxMemMap := memory_map.NewLinuxMemoryMap()
	mm, err := linuxMemMap.ReadMemoryMap(int(pid))
	if err != nil {
		return fmt.Errorf("failed to read memory map: %w", err)
	}

	// IsAddressValid2 requires the memory map to be sorted by address
	sort.Slice(mm, func(i, j int) bool {
		return mm[i].Address < mm[j].Address
	})

	// Now update the memory map with the lock
	p.mm = mm
	return nil
}

func (p *LinuxProcess) IsValidAddress(addr process.ProcessMemoryAddress) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.isValidAddressInternal(addr)
}

// Internal helper function that assumes the mutex is already locked
func (p *LinuxProcess) isValidAddressInternal(addr process.ProcessMemoryAddress) bool {
	// Check if address is within any mapped memory region

	if addr <= 0x10000 {
		return false
	}

	if addr > 0x700000000000 {
		return false
	}
	// if addr > 0x7FFFFFFFFFFF {
	// 	return false
	// }

	if item := memory_map.IsValidAddress2(uint64(addr), p.mm); item != nil {
		// Check if memory region is readable
		if isReadablePerms(item.Perms) {
			return true
		}
	}

	return false
}

// Internal helper function that assumes the mutex is already locked
// Returns the memory region containing the address and whether it's writable
func (p *LinuxProcess) getMemoryRegionForAddress(addr process.ProcessMemoryAddress) (*memory_map.MemoryMapItem, bool) {
	for _, item := range p.mm {
		end := item.Address + uint64(item.Size)
		if uint64(addr) >= item.Address && uint64(addr) < end {
			return &item, isWritablePerms(item.Perms)
		}
	}
	return nil, false
}

func (p *LinuxProcess) GetMemoryMap() ([]memory_map.MemoryMapItem, error) {
	p.mu.Lock()

	if p.pid == 0 {
		p.mu.Unlock()
		return nil, fmt.Errorf("process not opened")
	}

	// Make a copy of the memory map to prevent external modification
	result := make([]memory_map.MemoryMapItem, len(p.mm))
	copy(result, p.mm)

	p.mu.Unlock()

	return result, nil
}

// Helper functions for checking permissions using the Linux memory map
var memoryMapHelper = memory_map.NewLinuxMemoryMap()

// Helper function to check if memory region has read permissions
func isReadablePerms(perms string) bool {
	return memoryMapHelper.IsReadablePerms(perms)
}

// Helper function to check if memory region has write permissions
func isWritablePerms(perms string) bool {
	return memoryMapHelper.IsWritablePerms(perms)
}

// Helper function to check if memory region has execute permissions
func isExecutablePerms(perms string) bool {
	return memoryMapHelper.IsExecutablePerms(perms)
}
