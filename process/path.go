package process

import (
	"fmt"
	"unsafe"
)

// ReadPath reads a value of type T at the end of a pointer path.
// It starts at base, adds the first offset, reads a pointer, adds the next offset, reads a pointer, etc.
// The last offset is added to the final pointer, and then T is read from that address.
// If offsets is empty, it reads T from base.
func ReadPath[T any](proc Process, base ProcessMemoryAddress, offsets ...ProcessMemorySize) (T, error) {
	currentAddr := base

	// Iterate over all offsets except the last one
	for i := 0; i < len(offsets)-1; i++ {
		// Calculate address of the pointer
		ptrAddr := currentAddr + ProcessMemoryAddress(offsets[i])

		// Read the pointer
		// We assume pointers are 8 bytes (uint64) for now.
		// TODO: Support 32-bit pointers if needed, maybe via Process interface?
		ptrVal, err := Read[uint64](proc, ptrAddr)
		if err != nil {
			var zero T
			return zero, fmt.Errorf("failed to read pointer at offset %d (addr 0x%x): %w", i, ptrAddr, err)
		}

		// Check if pointer is valid
		if ptrVal == 0 {
			var zero T
			return zero, fmt.Errorf("pointer at offset %d (addr 0x%x) is null", i, ptrAddr)
		}

		currentAddr = ProcessMemoryAddress(ptrVal)
	}

	// Apply the last offset (or the only offset if len == 1)
	finalOffset := ProcessMemorySize(0)
	if len(offsets) > 0 {
		finalOffset = offsets[len(offsets)-1]
	}

	finalAddr := currentAddr + ProcessMemoryAddress(finalOffset)

	// Read the final value
	val, err := Read[T](proc, finalAddr)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("failed to read final value at 0x%x: %w", finalAddr, err)
	}

	return val, nil
}

// Read is a helper to read a single value of type T from memory
func Read[T any](proc Process, addr ProcessMemoryAddress) (T, error) {
	// Use pod.ReadT if available, but we can't import pod here due to cycle (pod imports process).
	// So we implement a basic reader here using ReadMemory.
	// Actually, we can't easily do generics here without reflection or unsafe, similar to pod.
	// But wait, process package shouldn't depend on pod.
	// Maybe we should put ReadPath in pod package?
	// The user requested "process interface for ReadPath", but "process" package is low level.
	// "pod" is where the high level reading logic is.
	// Let's check imports.
	// process -> (no dependencies)
	// pod -> process
	// So we can't import pod in process.
	// We can implement a simple ReadT here using binary.Read or unsafe, similar to pod.ReadT but simpler.

	// However, ReadPath logic is: read pointer -> read pointer -> read value.
	// Reading pointers requires reading uint64.
	// Reading the final value requires reading T.

	// Let's defer the actual reading to a helper that uses unsafe/binary.
	// Or we can move ReadPath to a new package `gomem/path` or put it in `pod`.
	// The user said "Implement a Process Interface ReadPath[T]".
	// If it's in `process` package, it can't use `pod`.

	// Let's try to implement a basic ReadT here.
	return readT[T](proc, addr)
}

func readT[T any](proc Process, addr ProcessMemoryAddress) (T, error) {
	var t T
	size := ProcessMemorySize(unsafe.Sizeof(t))
	if size == 0 {
		return t, nil
	}

	data, err := proc.ReadMemory(addr, size)
	if err != nil {
		return t, err
	}

	// Copy data to t
	copyTo(&t, data)
	return t, nil
}

// copyTo copies bytes to *T
func copyTo[T any](dst *T, src []byte) {
	size := int(unsafe.Sizeof(*dst))
	if len(src) < size {
		return // Should not happen if ReadMemory succeeded with correct size
	}

	// Create a byte slice view of dst
	dstBytes := unsafe.Slice((*byte)(unsafe.Pointer(dst)), size)
	copy(dstBytes, src)
}
