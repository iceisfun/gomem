//go:build linux

package process_linux

import (
	"bytes"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"gomem/process"
)

// Scan searches for the given pattern in the process memory
// and returns all matching addresses
func (p *LinuxProcess) Scan(aob process.AOB) ([]process.ProcessMemoryAddress, error) {
	// Get the memory map to know which regions to scan
	memMap, err := p.GetMemoryMap()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory map: %w", err)
	}

	var results []process.ProcessMemoryAddress

	// Validate the AOB
	if len(aob.Pattern) == 0 {
		return nil, fmt.Errorf("empty pattern")
	}

	// If no mask is provided, create a mask of all 0xFF (exact match)
	if len(aob.Mask) == 0 {
		aob.Mask = bytes.Repeat([]byte{0xFF}, len(aob.Pattern))
	} else if len(aob.Mask) != len(aob.Pattern) {
		return nil, fmt.Errorf("mask length (%d) doesn't match pattern length (%d)",
			len(aob.Mask), len(aob.Pattern))
	}

	// Log that we're starting a scan
	p.log.Infoln("Starting memory scan for pattern of length", len(aob.Pattern))
	fmt.Printf("Pattern bytes: %x\n", aob.Pattern)

	// Scan each memory region
	for _, region := range memMap {
		// Skip non-readable regions
		if !isReadablePerms(region.Perms) {
			continue
		}

		// Read the memory region
		data, err := p.ReadMemory(process.ProcessMemoryAddress(region.Address), process.ProcessMemorySize(region.Size))

		if err != nil {
			if err == process.ErrAddressNotMapped {
				continue
			}

			// Some regions might fail to read due to permissions or other reasons
			// Just log and continue
			p.log.Debugln("Failed to read memory region at", fmt.Sprintf("%x", region.Address), err)
			continue
		}

		// Search for matches in this region
		matches := findPatternMatches(data, aob.Pattern, aob.Mask)

		// Convert relative offsets to absolute addresses
		for _, offset := range matches {
			addr := process.ProcessMemoryAddress(region.Address + uint64(offset))
			results = append(results, addr)
		}
	}

	p.log.Infoln("Scan complete, found", len(results), "matches")
	return results, nil
}

// ScanParallel searches for the given pattern in parallel
// maxdop controls the maximum degree of parallelism
func (p *LinuxProcess) ScanParallel(aob process.AOB, maxdop uint) ([]process.ProcessMemoryAddress, error) {
	// If maxdop is 0 or 1, use the regular scan
	if maxdop <= 1 {
		return p.Scan(aob)
	}

	// Get the memory map to know which regions to scan
	memMap, err := p.GetMemoryMap()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory map: %w", err)
	}

	// Validate the AOB
	if len(aob.Pattern) == 0 {
		return nil, fmt.Errorf("empty pattern")
	}

	// If no mask is provided, create a mask of all 0xFF (exact match)
	if len(aob.Mask) == 0 {
		aob.Mask = bytes.Repeat([]byte{0xFF}, len(aob.Pattern))
	} else if len(aob.Mask) != len(aob.Pattern) {
		return nil, fmt.Errorf("mask length (%d) doesn't match pattern length (%d)",
			len(aob.Mask), len(aob.Pattern))
	}

	// Log that we're starting a parallel scan
	p.log.Infoln("Starting parallel memory scan with maxdop=", maxdop)

	// Limit maxdop to number of CPUs if it's too large
	numCPU := uint(runtime.NumCPU())
	if maxdop > numCPU {
		maxdop = numCPU
		p.log.Debugln("Limiting maxdop to number of CPUs:", maxdop)
	}

	// Create a semaphore to limit concurrency
	sem := make(chan struct{}, maxdop)
	var wg sync.WaitGroup

	// Create a mutex for results
	var resultsMutex sync.Mutex
	var results []process.ProcessMemoryAddress

	// Filter out non-readable regions
	var readableRegions []struct {
		Address uint64
		Size    uint
	}

	var upperLimit = uint64(0x7d0000000000)
	for _, region := range memMap {
		if region.Address > upperLimit {
			continue
		}
		if isReadablePerms(region.Perms) {
			readableRegions = append(readableRegions, struct {
				Address uint64
				Size    uint
			}{
				Address: region.Address,
				Size:    region.Size,
			})
		}
	}

	// Scan each memory region in parallel
	for _, region := range readableRegions {
		wg.Add(1)

		// Acquire a semaphore slot
		sem <- struct{}{}

		go func(addr uint64, size uint) {
			defer func() {
				// Release the semaphore slot
				<-sem
				wg.Done()
			}()

			// Read the memory region
			data, err := p.ReadMemory(process.ProcessMemoryAddress(addr), process.ProcessMemorySize(size))
			if err != nil {
				if err == process.ErrAddressNotMapped {
					// If the address is not mapped, just skip this region
					return
				}

				// Some regions might fail to read due to permissions or other reasons
				p.log.Debugln("Failed to read memory region at", fmt.Sprintf("%x", addr), err)
				return
			}

			// Search for matches in this region
			matches := findPatternMatches(data, aob.Pattern, aob.Mask)

			// If there are matches, add them to the results
			if len(matches) > 0 {
				resultsMutex.Lock()
				for _, offset := range matches {
					results = append(results, process.ProcessMemoryAddress(addr+uint64(offset)))
				}
				resultsMutex.Unlock()
			}
		}(region.Address, region.Size)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	p.log.Infoln("Parallel scan complete, found", len(results), "matches")
	return results, nil
}

// findPatternMatches finds all occurrences of the pattern in the data
// Returns the offsets where matches were found
func findPatternMatches(data, pattern, mask []byte) []uint {
	if len(data) < len(pattern) {
		fmt.Printf("Data length (%d) is less than pattern length (%d)\n", len(data), len(pattern))
		return nil
	}

	var matches []uint

	// Scan through the data byte by byte
	for i := 0; i <= len(data)-len(pattern); i++ {
		matched := true

		// Check if the pattern matches at this position
		for j := 0; j < len(pattern); j++ {
			// Apply the mask: if mask byte is 0, skip this byte (wildcard)
			if mask[j] == 0 {
				continue
			}

			// Only compare the masked bits
			maskedData := data[i+j] & mask[j]
			maskedPattern := pattern[j] & mask[j]

			if maskedData != maskedPattern {
				matched = false
				break
			}
		}

		if matched {
			fmt.Printf("Found match at offset %d (0x%x)\n", i, i)
			matches = append(matches, uint(i))
		}
	}

	return matches
}

// ScanFirst searches for the first occurrence of the pattern
func (p *LinuxProcess) ScanFirst(aob process.AOB) (process.ProcessMemoryAddress, error) {
	results, err := p.Scan(aob)
	if err != nil {
		return 0, err
	}

	if len(results) == 0 {
		return 0, fmt.Errorf("pattern not found")
	}

	return results[0], nil
}

// ScanFirstParallel searches for the first occurrence of the pattern in parallel
func (p *LinuxProcess) ScanFirstParallel(aob process.AOB, maxdop uint) (process.ProcessMemoryAddress, error) {
	results, err := p.ScanParallel(aob, maxdop)
	if err != nil {
		return 0, err
	}

	if len(results) == 0 {
		return 0, fmt.Errorf("pattern not found")
	}

	return results[0], nil
}

// ScanInteger searches for an integer value in memory
func (p *LinuxProcess) ScanInteger(value int64, size uint) ([]process.ProcessMemoryAddress, error) {
	var pattern []byte

	// Convert the integer to bytes in little-endian order
	switch size {
	case 1:
		pattern = []byte{byte(value)}
	case 2:
		pattern = []byte{
			byte(value),
			byte(value >> 8),
		}
	case 4:
		pattern = []byte{
			byte(value),
			byte(value >> 8),
			byte(value >> 16),
			byte(value >> 24),
		}
	case 8:
		pattern = []byte{
			byte(value),
			byte(value >> 8),
			byte(value >> 16),
			byte(value >> 24),
			byte(value >> 32),
			byte(value >> 40),
			byte(value >> 48),
			byte(value >> 56),
		}
	default:
		return nil, fmt.Errorf("invalid integer size: %d", size)
	}

	return p.Scan(process.AOB{Pattern: pattern})
}

// ScanFloat searches for a float value in memory
func (p *LinuxProcess) ScanFloat(value float64, isFloat32 bool) ([]process.ProcessMemoryAddress, error) {
	var int64Val int64

	if isFloat32 {
		// Convert float64 to float32 bits
		float32Val := float32(value)
		int32Val := *(*int32)(unsafe.Pointer(&float32Val))
		int64Val = int64(int32Val)
		return p.ScanInteger(int64Val, 4)
	} else {
		// Convert float64 to int64 bits
		int64Val = *(*int64)(unsafe.Pointer(&value))
		return p.ScanInteger(int64Val, 8)
	}
}

// ScanString searches for a string in memory
func (p *LinuxProcess) ScanString(value string, isUTF16 bool) ([]process.ProcessMemoryAddress, error) {
	if !isUTF16 {
		// ASCII/UTF-8 string
		return p.Scan(process.AOB{Pattern: []byte(value)})
	} else {
		// UTF-16 string (LE)
		pattern := make([]byte, len(value)*2)
		for i, c := range value {
			pattern[i*2] = byte(c)
			pattern[i*2+1] = byte(c >> 8)
		}
		return p.Scan(process.AOB{Pattern: pattern})
	}
}
