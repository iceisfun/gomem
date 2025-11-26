//go:build linux

package memory_map

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// LinuxMemoryMap implements MemoryMap for Linux
type LinuxMemoryMap struct{}

// NewLinuxMemoryMap creates a new LinuxMemoryMap instance
func NewLinuxMemoryMap() *LinuxMemoryMap {
	return &LinuxMemoryMap{}
}

// ReadMemoryMap reads and parses the memory map for a process from /proc/[pid]/maps
func (l *LinuxMemoryMap) ReadMemoryMap(pid int) ([]MemoryMapItem, error) {
	file, err := os.Open(fmt.Sprintf("/proc/%d/maps", pid))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var memoryMap []MemoryMapItem
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}

		// Parse address range (e.g., "00400000-0040b000")
		addrRange := strings.Split(fields[0], "-")
		if len(addrRange) != 2 {
			continue
		}

		startAddr, err := strconv.ParseUint(addrRange[0], 16, 64)
		if err != nil {
			continue
		}

		endAddr, err := strconv.ParseUint(addrRange[1], 16, 64)
		if err != nil {
			continue
		}

		size := uint(endAddr - startAddr)
		perms := fields[1]

		memoryMap = append(memoryMap, MemoryMapItem{
			Address: startAddr,
			Size:    size,
			Perms:   perms,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return memoryMap, nil
}

func (l *LinuxMemoryMap) IsReadablePerms(perms string) bool {
	return len(perms) > 0 && perms[0] == 'r'
}

func (l *LinuxMemoryMap) IsWritablePerms(perms string) bool {
	return len(perms) > 1 && perms[1] == 'w'
}

func (l *LinuxMemoryMap) IsExecutablePerms(perms string) bool {
	return len(perms) > 2 && perms[2] == 'x'
}
