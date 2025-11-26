package process

import (
	"fmt"
)

// ProcessMemoryAddress represents a memory address within a process
type ProcessMemoryAddress uint64

func (pma ProcessMemoryAddress) ToString() string {
	return fmt.Sprintf("0x%X", uint64(pma))
}

// ProcessMemorySize represents a size of memory region
type ProcessMemorySize uint

func (pms ProcessMemorySize) ToString() string {
	return fmt.Sprintf("%d bytes", uint(pms))
}

// AOB (Array of Bytes) represents a pattern to search for in memory
type AOB struct {
	Pattern []byte // The byte pattern to search for
	Mask    []byte // Optional mask where 0xFF means exact match and 0x00 means wildcard
}

// IsValid checks if the AOB pattern is valid
func (aob AOB) IsValid() bool {
	return len(aob.Pattern) == len(aob.Mask)
}

func NewAOB(pattern, mask []byte) (AOB, error) {
	if len(pattern) != len(mask) {
		return AOB{}, fmt.Errorf("pattern and mask must be of the same length")
	}
	return AOB{Pattern: pattern, Mask: mask}, nil
}
