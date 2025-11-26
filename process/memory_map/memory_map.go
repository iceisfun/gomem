package memory_map

import (
	"fmt"
	"sort"
)

// MemoryMapItem represents a memory region in a process's address space
type MemoryMapItem struct {
	Address uint64 // The starting address of the memory region
	Size    uint   // The size of the memory region in bytes
	Perms   string // Permissions (e.g., "r-xp" for read, execute, private)
}

// String returns a string representation of the memory map item
func (mmItem MemoryMapItem) String() string {
	return fmt.Sprintf("Address: %x, Size: %d, Perms: %s", mmItem.Address, mmItem.Size, mmItem.Perms)
}

func (mmItem MemoryMapItem) IsReadable() bool {
	return mmItem.Perms[0] == 'r'
}

func (mmItem MemoryMapItem) IsWritable() bool {
	return mmItem.Perms[1] == 'w'
}

// MemoryMap defines the interface for operations related to a process's memory map
type MemoryMap interface {
	// ReadMemoryMap reads and parses the memory map for a process
	ReadMemoryMap(pid int) ([]MemoryMapItem, error)

	// IsReadablePerms checks if a memory region has read permissions
	IsReadablePerms(perms string) bool

	// IsWritablePerms checks if a memory region has write permissions
	IsWritablePerms(perms string) bool

	// IsExecutablePerms checks if a memory region has execute permissions
	IsExecutablePerms(perms string) bool
}

// Helper functions for working with memory maps

// IsValidAddress checks if an address is within a valid, readable memory region
func IsValidAddress(addr uint64, memoryMap []MemoryMapItem) bool {
	for _, item := range memoryMap {
		end := item.Address + uint64(item.Size)
		if addr >= item.Address && addr < end {
			return true
		}
	}
	return false
}

func IsValidAddress2(addr uint64, memoryMap []MemoryMapItem) *MemoryMapItem {
	i := sort.Search(len(memoryMap), func(i int) bool {
		return memoryMap[i].Address+uint64(memoryMap[i].Size) > addr
	})
	if i < len(memoryMap) && memoryMap[i].Address <= addr {
		return &memoryMap[i]
	}

	return nil
}

// GetMemoryRegionForAddress returns the memory region containing an address
func GetMemoryRegionForAddress(addr uint64, memoryMap []MemoryMapItem) *MemoryMapItem {
	for _, item := range memoryMap {
		end := item.Address + uint64(item.Size)
		if addr >= item.Address && addr < end {
			return &item
		}
	}
	return nil
}
