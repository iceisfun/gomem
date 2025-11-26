package process_blob

import (
	"fmt"

	"gomem/process"
)

// Implement ReadPointerChain for ProcessBlob (was missing)
func (p *ProcessBlob) ReadPointerChain(base process.ProcessMemoryAddress, size process.ProcessMemorySize, offsets ...process.ProcessMemorySize) (process.ProcessReadOffset, error) {
	currentAddr := base
	for i, offset := range offsets {
		// Read pointer at current address
		ptr, err := p.ReadPOINTER(currentAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to read pointer at level %d (addr %x): %w", i, currentAddr, err)
		}
		currentAddr = ptr + process.ProcessMemoryAddress(offset)
	}

	// Read final blob
	return p.ReadBlob(currentAddr, size)
}

func (p *ProcessBlob) ReadPointerChainDebug(base process.ProcessMemoryAddress, size process.ProcessMemorySize, offsets ...process.ProcessMemorySize) (process.ProcessReadOffset, error) {
	// Same as ReadPointerChain for now
	return p.ReadPointerChain(base, size, offsets...)
}
