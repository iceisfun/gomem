//go:build linux

package process_linux

import (
	"encoding/hex"
	"fmt"
	"gomem/process"
)

// ReadPointerChain walks pointer fields at all offsets except the last,
// which is treated as a raw byte offset into the final struct, and then
// reads `size` bytes starting there.
//
// Example:
//
//	// base -> [ +0 ]ptrA -> [ +24 ]ptrB -> [ +144 ]ptrC
//	// final read at (ptrC + 504), length 0x10
//	data, err := proc.ReadPointerChain(process.ProcessMemoryAddress(room1Ptr),
//	                                   0x10, 0, 24, 144, 504)
func (p *LinuxProcess) ReadPointerChain(
	base process.ProcessMemoryAddress,
	size process.ProcessMemorySize,
	offsets ...process.ProcessMemorySize,
) (process.ProcessReadOffset, error) {

	// No offsets: read size bytes directly at base
	if len(offsets) == 0 {
		return p.ReadBlob(base, size)
	}

	current := base

	// Deref each offset except the last
	for i := 0; i < len(offsets)-1; i++ {
		off := offsets[i]
		addr := current + process.ProcessMemoryAddress(off)

		ptr := p.ReadPOINTER2(addr)
		if ptr == 0 {
			return nil, fmt.Errorf("ReadPointerChain: NULL pointer at step %d (addr=%#x + off=%#x)", i, uint64(current), uint64(off))
		}
		if !p.IsValidAddress(ptr) {
			return nil, fmt.Errorf("ReadPointerChain: invalid pointer %#x at step %d (addr=%#x + off=%#x)", uint64(ptr), i, uint64(current), uint64(off))
		}
		current = ptr
	}

	// Last offset is a raw byte offset into `current` (no deref)
	finalOff := offsets[len(offsets)-1]
	start := current + process.ProcessMemoryAddress(finalOff)

	blob, err := p.ReadBlob(start, size)
	if err != nil {
		return nil, fmt.Errorf("ReadPointerChain: read blob at %#x (size=%#x) failed: %w", uint64(start), uint64(size), err)
	}
	return blob, nil
}

// ReadPointerChainDebug does the same as ReadPointerChain but prints the hop trace.
func (p *LinuxProcess) ReadPointerChainDebug(
	base process.ProcessMemoryAddress,
	size process.ProcessMemorySize,
	offsets ...process.ProcessMemorySize,
) (process.ProcessReadOffset, error) {

	if len(offsets) == 0 {
		fmt.Printf("[chain] base=%#x read size=%#x\n", uint64(base), uint64(size))
		return p.ReadBlob(base, size)
	}

	current := base
	fmt.Printf("[chain] base=%#x\n", uint64(current))

	for i := 0; i < len(offsets)-1; i++ {
		off := offsets[i]
		addr := current + process.ProcessMemoryAddress(off)
		ptr := p.ReadPOINTER2(addr)
		fmt.Printf("[chain] step %d: *(%#x + %#x) => %#x\n", i, uint64(current), uint64(off), uint64(ptr))
		if ptr == 0 {
			return nil, fmt.Errorf("ReadPointerChainDebug: NULL pointer at step %d", i)
		}
		if !p.IsValidAddress(ptr) {
			return nil, fmt.Errorf("ReadPointerChainDebug: invalid pointer %#x at step %d", uint64(ptr), i)
		}
		current = ptr
	}

	finalOff := offsets[len(offsets)-1]
	start := current + process.ProcessMemoryAddress(finalOff)
	fmt.Printf("[chain] final: read size=%#x at (%#x + %#x) => %#x\n",
		uint64(size), uint64(current), uint64(finalOff), uint64(start))

	blob, err := p.ReadBlob(start, size)
	if err != nil {
		return nil, fmt.Errorf("ReadPointerChainDebug: read blob at %#x failed: %w", uint64(start), err)
	}

	fmt.Println(hex.Dump(blob.Data()))

	return blob, nil
}
