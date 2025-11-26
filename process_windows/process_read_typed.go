//go:build windows

package process_windows

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"gomem/process"
	"gomem/process/memory_map"
	"gomem/process_blob"
)

// ReadUINT8 reads an unsigned 8-bit integer from the specified address
func (p *WindowsProcess) ReadUINT8(addr process.ProcessMemoryAddress) (uint8, error) {
	data, err := p.ReadMemory(addr, 1)
	if err != nil {
		return 0, err
	}
	return data[0], nil
}

// ReadUINT16 reads an unsigned 16-bit integer from the specified address
func (p *WindowsProcess) ReadUINT16(addr process.ProcessMemoryAddress) (uint16, error) {
	data, err := p.ReadMemory(addr, 2)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(data), nil
}

// ReadUINT32 reads an unsigned 32-bit integer from the specified address
func (p *WindowsProcess) ReadUINT32(addr process.ProcessMemoryAddress) (uint32, error) {
	data, err := p.ReadMemory(addr, 4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(data), nil
}

// ReadUINT64 reads an unsigned 64-bit integer from the specified address
func (p *WindowsProcess) ReadUINT64(addr process.ProcessMemoryAddress) (uint64, error) {
	data, err := p.ReadMemory(addr, 8)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(data), nil
}

// ReadINT8 reads a signed 8-bit integer from the specified address
func (p *WindowsProcess) ReadINT8(addr process.ProcessMemoryAddress) (int8, error) {
	data, err := p.ReadMemory(addr, 1)
	if err != nil {
		return 0, err
	}
	return int8(data[0]), nil
}

// ReadINT16 reads a signed 16-bit integer from the specified address
func (p *WindowsProcess) ReadINT16(addr process.ProcessMemoryAddress) (int16, error) {
	data, err := p.ReadMemory(addr, 2)
	if err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(data)), nil
}

// ReadINT32 reads a signed 32-bit integer from the specified address
func (p *WindowsProcess) ReadINT32(addr process.ProcessMemoryAddress) (int32, error) {
	data, err := p.ReadMemory(addr, 4)
	if err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(data)), nil
}

// ReadINT64 reads a signed 64-bit integer from the specified address
func (p *WindowsProcess) ReadINT64(addr process.ProcessMemoryAddress) (int64, error) {
	data, err := p.ReadMemory(addr, 8)
	if err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(data)), nil
}

// ReadFLOAT32 reads a 32-bit floating point number from the specified address
func (p *WindowsProcess) ReadFLOAT32(addr process.ProcessMemoryAddress) (float32, error) {
	data, err := p.ReadMemory(addr, 4)
	if err != nil {
		return 0, err
	}
	bits := binary.LittleEndian.Uint32(data)
	return *(*float32)(unsafe.Pointer(&bits)), nil
}

// ReadFLOAT64 reads a 64-bit floating point number from the specified address
func (p *WindowsProcess) ReadFLOAT64(addr process.ProcessMemoryAddress) (float64, error) {
	data, err := p.ReadMemory(addr, 8)
	if err != nil {
		return 0, err
	}
	bits := binary.LittleEndian.Uint64(data)
	return *(*float64)(unsafe.Pointer(&bits)), nil
}

// ReadNTS reads a null-terminated string from the specified address with a maximum length
func (p *WindowsProcess) ReadNTS(addr process.ProcessMemoryAddress, maxLength process.ProcessMemorySize) (string, error) {
	if maxLength == 0 {
		return "", nil
	}

	// Read the maximum length
	data, err := p.ReadMemory(addr, maxLength)
	if err != nil {
		return "", err
	}

	// Find the null terminator
	for i, b := range data {
		if b == 0 {
			return string(data[:i]), nil
		}
	}

	// If no null terminator found, return the whole buffer as string
	return string(data), nil
}

// ReadPOINTER reads a pointer value from the specified address
func (p *WindowsProcess) ReadPOINTER(addr process.ProcessMemoryAddress) (process.ProcessMemoryAddress, error) {
	// On 64-bit systems, pointers are 8 bytes
	// On 32-bit systems, pointers are 4 bytes
	const ptrSize = 8 // Assuming 64-bit architecture

	data, err := p.ReadMemory(addr, ptrSize)
	if err != nil {
		return 0, err
	}

	// Read as uint64 for 64-bit pointers
	ptr := binary.LittleEndian.Uint64(data)
	return process.ProcessMemoryAddress(ptr), nil
}

func (p *WindowsProcess) ReadPOINTER2(addr process.ProcessMemoryAddress) process.ProcessMemoryAddress {
	ptr, err := p.ReadPOINTER(addr)
	if err != nil {
		return 0 // Return zero on error
	}
	return ptr
}

func (p *WindowsProcess) ReadBlob(addr process.ProcessMemoryAddress, size process.ProcessMemorySize) (process.ProcessReadOffset, error) {
	if size == 0 {
		return nil, nil // Return nil for zero size
	}

	data, err := p.ReadMemory(addr, size)
	if err != nil {
		return nil, err
	}

	if len(data) < int(size) {
		return nil, errors.New("read less data than requested")
	}

	return process_blob.NewProcessBlob(addr, data[:size]), nil
}

func (p *WindowsProcess) ReadPointers(base process.ProcessMemoryAddress, count int) (results []process.ProcessMemoryAddress, err error) {
	size := uint64(count) * 8

	if size <= 0 {
		return nil, errors.New("invalid count for pointers")
	}

	data, err := p.ReadMemory(base, process.ProcessMemorySize(size))
	if err != nil {
		return nil, err
	}
	for i := range count {
		offset := i * 8
		if offset+8 > len(data) {
			return nil, errors.New("not enough data read for pointers")
		}
		ptr := binary.LittleEndian.Uint64(data[offset : offset+8])

		if memory_map.IsValidAddress2(ptr, p.mm) != nil {
			results = append(results, process.ProcessMemoryAddress(ptr))
		}
	}
	return results, nil
}

// fixme
const defaultReadBlobsMDOP = 8 // Maximum Degree Of Parallelism

// ReadBlobs reads multiple blobs of a specified size from a list of addresses concurrently.
// It returns a slice of ReadBlobsResult, one for each input address, preserving the order.
// If an individual read fails, the corresponding ReadBlobsResult will contain the error.
func (p *WindowsProcess) ReadBlobsX(list []process.ProcessMemoryAddress, size process.ProcessMemorySize) []process.ReadBlobsResult {
	if len(list) == 0 {
		return []process.ReadBlobsResult{}
	}

	// Initialize results slice to the same length as the input list to preserve order.
	// We will populate this slice by index.
	results := make([]process.ReadBlobsResult, len(list))

	mdop := defaultReadBlobsMDOP

	semaphore := make(chan struct{}, mdop)
	var wg sync.WaitGroup

	for i, addr := range list {
		wg.Add(1)
		go func(index int, addressToRead process.ProcessMemoryAddress) {
			defer wg.Done()

			semaphore <- struct{}{}        // Acquire a slot
			defer func() { <-semaphore }() // Release the slot

			blob, err := p.ReadBlob(addressToRead, size)

			// safe to write to unique index, no race
			results[index] = process.ReadBlobsResult{
				Address: addressToRead,
				Blob:    blob,
				Err:     err,
			}
		}(i, addr) // Pass index and address as arguments to the goroutine
	}

	wg.Wait() // Wait for all goroutines to complete

	return results
}

var (
	ErrAddressNotInAnyValidRegion = errors.New("address not found in any valid mapped region")
	ErrRequestExceedsRegionBounds = errors.New("requested read size exceeds its mapped region's boundaries")
	ErrBlobReadSizeIsZero         = errors.New("blobReadSize cannot be zero")
	ErrGroupReadFailed            = errors.New("failed to read combined blob for group")
	ErrSliceOutOfBounds           = errors.New("error slicing data for sub-request")
	ErrRequestAddrOutOfGroup      = errors.New("request address outside of group's read range")
	ErrAddressCalculationOverflow = errors.New("address calculation resulted in overflow")
)

// OriginalRequest stores information about an individual read request before grouping.
type OriginalRequest struct {
	Index   int // Original index in the input 'list' to place the result
	Address process.ProcessMemoryAddress
	Size    process.ProcessMemorySize
}

// GroupedReadOp defines a single large read operation that covers multiple original requests
// that fall within the same memory_map.MemoryMapItem.
type GroupedReadOp struct {
	Region            memory_map.MemoryMapItem     // The memory map item this group belongs to
	CombinedReadStart process.ProcessMemoryAddress // The absolute starting address for the combined read for this group
	CombinedReadEnd   process.ProcessMemoryAddress // The absolute *inclusive* ending address for the combined read
	Requests          []OriginalRequest            // List of original requests covered by this combined read
}

// ReadBlobsCluster reads multiple blobs of a specified size from a list of addresses concurrently.
// It attempts to optimize reads by grouping requests that fall within the same memory regions.
func (p *WindowsProcess) ReadBlobs(list []process.ProcessMemoryAddress, blobReadSize process.ProcessMemorySize) []process.ReadBlobsResult {
	if len(list) == 0 {
		return []process.ReadBlobsResult{}
	}
	if blobReadSize == 0 {
		results := make([]process.ReadBlobsResult, len(list))
		for i, addr := range list {
			results[i] = process.ReadBlobsResult{Address: addr, Err: ErrBlobReadSizeIsZero}
		}
		return results
	}

	results := make([]process.ReadBlobsResult, len(list))

	// --- Phase 1: Grouping Requests ---
	// Key: Start address of the memory_map.MemoryMapItem (Region)
	// Value: Pointer to the GroupedReadOp for that region
	groups := make(map[uint64]*GroupedReadOp)

	for i, currentReqAddr := range list {
		// 1. Find the memory region for the start of the current request
		// IsValidAddress2 should ideally return the region containing 'currentReqAddr'.
		// We assume p.mm is the sorted list of MemoryMapItems for the process.
		regionItem := memory_map.IsValidAddress2(uint64(currentReqAddr), p.mm)

		if regionItem == nil {
			results[i] = process.ReadBlobsResult{Address: currentReqAddr, Err: ErrAddressNotInAnyValidRegion}
			continue
		}

		// 2. Validate that the entire request [currentReqAddr, currentReqAddr + blobReadSize - 1]
		//    fits within this specific regionItem.
		regionStartAddr := process.ProcessMemoryAddress(regionItem.Address)
		// regionItem.Size is uint64, ensure no underflow if regionItem.Size is 0
		var regionEndAddrInclusive process.ProcessMemoryAddress
		if regionItem.Size == 0 {
			regionEndAddrInclusive = regionStartAddr // Region of size 0, only valid if addr == regionStartAddr and blobReadSize == 0 or 1
		} else {
			regionEndAddrInclusive = process.ProcessMemoryAddress(regionStartAddr + process.ProcessMemoryAddress(regionItem.Size) - 1)
		}

		// Basic sanity check: currentReqAddr should be within the region we just found for it.
		if currentReqAddr < regionStartAddr || currentReqAddr > regionEndAddrInclusive {
			results[i] = process.ReadBlobsResult{Address: currentReqAddr, Err: fmt.Errorf("address 0x%X inconsistent with its determined region [0x%X-0x%X]", currentReqAddr, regionStartAddr, regionEndAddrInclusive)}
			continue
		}

		currentReqEndAddrInclusive := currentReqAddr + process.ProcessMemoryAddress(blobReadSize) - 1
		// Check for overflow in end address calculation
		if currentReqEndAddrInclusive < currentReqAddr && blobReadSize > 0 {
			results[i] = process.ReadBlobsResult{Address: currentReqAddr, Err: fmt.Errorf("%w: for address 0x%X, size %d", ErrAddressCalculationOverflow, currentReqAddr, blobReadSize)}
			continue
		}

		if currentReqEndAddrInclusive > regionEndAddrInclusive {
			results[i] = process.ReadBlobsResult{
				Address: currentReqAddr,
				Err:     fmt.Errorf("%w: request for 0x%X (size %d) ends at 0x%X, but region [0x%X-0x%X] ends at 0x%X", ErrRequestExceedsRegionBounds, currentReqAddr, blobReadSize, currentReqEndAddrInclusive, regionStartAddr, regionEndAddrInclusive, regionEndAddrInclusive),
			}
			continue
		}

		// 3. Add or update the group for this regionItem
		group, exists := groups[regionItem.Address] // Use regionItem.Address as the key
		if !exists {
			group = &GroupedReadOp{
				Region:            *regionItem,
				CombinedReadStart: currentReqAddr, // Initialize with the first valid request's bounds
				CombinedReadEnd:   currentReqEndAddrInclusive,
				Requests:          make([]OriginalRequest, 0, 1), // Small initial capacity
			}
			groups[regionItem.Address] = group
		}

		// Add current request to the group
		group.Requests = append(group.Requests, OriginalRequest{
			Index:   i,
			Address: currentReqAddr,
			Size:    blobReadSize, // Store the original requested size
		})

		// Update the combined read boundaries for the group based on this new request
		if currentReqAddr < group.CombinedReadStart {
			group.CombinedReadStart = currentReqAddr
		}
		if currentReqEndAddrInclusive > group.CombinedReadEnd {
			group.CombinedReadEnd = currentReqEndAddrInclusive
		}
	}

	// --- Phase 2: Reading Grouped Blobs Concurrently ---
	mdop := defaultReadBlobsMDOP
	semaphore := make(chan struct{}, mdop)
	var wg sync.WaitGroup

	for _, groupPtr := range groups { // groupPtr is *GroupedReadOp
		// Capture loop variable correctly by making a copy of the struct for the goroutine.
		// This ensures each goroutine works on its intended group's data.
		groupToProcess := *groupPtr

		wg.Add(1)
		go func(g GroupedReadOp) { // g is now a copy of GroupedReadOp for this goroutine
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Sanity check: CombinedReadEnd should not be less than CombinedReadStart.
			// This should be guaranteed by the grouping logic if group.Requests is not empty.
			if g.CombinedReadEnd < g.CombinedReadStart {
				err := fmt.Errorf("internal logic error: group CombinedReadEnd (0x%X) < CombinedReadStart (0x%X) for region starting at 0x%X", g.CombinedReadEnd, g.CombinedReadStart, g.Region.Address)
				for _, req := range g.Requests {
					results[req.Index] = process.ReadBlobsResult{Address: req.Address, Err: err}
				}
				return
			}

			sizeForCombinedRead := process.ProcessMemorySize(g.CombinedReadEnd - g.CombinedReadStart + 1)

			// If all requests in a group happen to result in a 0-byte combined read (e.g. single request of size 1, start==end),
			// and blobReadSize was 1, then sizeForCombinedRead will be 1. This is fine.
			// An issue might be if sizeForCombinedRead becomes 0 due to an empty request list or logic error,
			// but groups map should only contain groups with at least one request.

			// Assuming p.ReadBlob returns (data []byte, err error)
			combinedData, err := p.ReadBlob(g.CombinedReadStart, sizeForCombinedRead)

			if err != nil {
				wrappedErr := fmt.Errorf("%w for addresses in range [0x%X-0x%X]: %v", ErrGroupReadFailed, g.CombinedReadStart, g.CombinedReadEnd, err)
				for _, req := range g.Requests {
					results[req.Index] = process.ReadBlobsResult{
						Address: req.Address,
						Blob:    nil, // No data if group read failed
						Err:     wrappedErr,
					}
				}
				return
			}

			data := combinedData.Data()

			// If reading the combined blob succeeded, extract data for each original request
			for _, req := range g.Requests {
				// req.Address must be >= g.CombinedReadStart (guaranteed by grouping logic)
				// req.Address + req.Size - 1 must be <= g.CombinedReadEnd (also guaranteed)
				if req.Address < g.CombinedReadStart || (req.Address+process.ProcessMemoryAddress(req.Size)-1) > g.CombinedReadEnd {
					results[req.Index] = process.ReadBlobsResult{
						Address: req.Address,
						Blob:    nil,
						Err:     fmt.Errorf("%w: request 0x%X (size %d) somehow outside group's effective read range [0x%X-0x%X]", ErrRequestAddrOutOfGroup, req.Address, req.Size, g.CombinedReadStart, g.CombinedReadEnd),
					}
					continue
				}

				offsetInCombinedData := uint64(req.Address - g.CombinedReadStart)
				requestedSizeUint64 := uint64(req.Size)

				// Defensive boundary check for slicing combinedData
				if offsetInCombinedData+requestedSizeUint64 > uint64(len(data)) {
					results[req.Index] = process.ReadBlobsResult{
						Address: req.Address,
						Blob:    nil,
						Err:     fmt.Errorf("%w: request for 0x%X (size %d) at offset %d (len %d) exceeds bounds of successfully read group data (len %d from 0x%X)", ErrSliceOutOfBounds, req.Address, req.Size, offsetInCombinedData, requestedSizeUint64, len(data), g.CombinedReadStart),
					}
					continue
				}

				// Extract the specific blob. Create a copy to ensure each result owns its data.
				dataSlice := data[offsetInCombinedData : offsetInCombinedData+requestedSizeUint64]
				blobForRequest := make([]byte, len(dataSlice))
				copy(blobForRequest, dataSlice)

				results[req.Index] = process.ReadBlobsResult{
					Address: req.Address,
					Blob:    process_blob.NewProcessBlob(req.Address, blobForRequest),
					Err:     nil,
				}
			}
		}(groupToProcess) // Pass the copied struct to the goroutine
	}

	wg.Wait() // Wait for all goroutines to complete
	return results
}

func (p *WindowsProcess) ReadPointerChain(base process.ProcessMemoryAddress, size process.ProcessMemorySize, offsets ...process.ProcessMemorySize) (process.ProcessReadOffset, error) {
	return nil, fmt.Errorf("ReadPointerChain not implemented for WindowsProcess")
}

func (p *WindowsProcess) ReadPointerChainDebug(base process.ProcessMemoryAddress, size process.ProcessMemorySize, offsets ...process.ProcessMemorySize) (process.ProcessReadOffset, error) {
	return nil, fmt.Errorf("ReadPointerChainDebug not implemented for WindowsProcess")
}
