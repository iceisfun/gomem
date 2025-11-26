package process_blob

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"unsafe"

	"gomem/process"
	"gomem/process/memory_map"
)

// ProcessDump implements process.Process for a loaded process dump
type ProcessDump struct {
	PID       process.ProcessID
	Name      string
	MemoryMap []memory_map.MemoryMapItem
	Blobs     map[uint64][]byte // Address -> Data
}

// NewProcessDump creates a new ProcessDump instance
func NewProcessDump() *ProcessDump {
	return &ProcessDump{
		Blobs: make(map[uint64][]byte),
	}
}

func (p *ProcessDump) Open(pid process.ProcessID) error {
	return fmt.Errorf("Open not supported for ProcessDump, use Load")
}

func (p *ProcessDump) Close() error {
	p.Blobs = nil
	p.MemoryMap = nil
	return nil
}

func (p *ProcessDump) GetPID() process.ProcessID {
	return p.PID
}

func (p *ProcessDump) UpdateMemoryMap() error {
	return nil // Memory map is static in a dump
}

func (p *ProcessDump) IsValidAddress(addr process.ProcessMemoryAddress) bool {
	return memory_map.IsValidAddress(uint64(addr), p.MemoryMap)
}

func (p *ProcessDump) GetMemoryMap() ([]memory_map.MemoryMapItem, error) {
	result := make([]memory_map.MemoryMapItem, len(p.MemoryMap))
	copy(result, p.MemoryMap)
	return result, nil
}

func (p *ProcessDump) ReadMemory(addr process.ProcessMemoryAddress, size process.ProcessMemorySize) ([]byte, error) {
	// Find the region containing the address
	region := memory_map.GetMemoryRegionForAddress(uint64(addr), p.MemoryMap)
	if region == nil {
		return nil, process.ErrAddressNotMapped
	}

	// Check if we have data for this region
	data, ok := p.Blobs[region.Address]
	if !ok {
		return nil, fmt.Errorf("no data for region 0x%x", region.Address)
	}

	offset := uint64(addr) - region.Address
	if offset >= uint64(len(data)) {
		return nil, fmt.Errorf("address 0x%x out of bounds of region data", addr)
	}

	if offset+uint64(size) > uint64(len(data)) {
		return nil, fmt.Errorf("read size %d exceeds region data bounds", size)
	}

	result := make([]byte, size)
	copy(result, data[offset:offset+uint64(size)])
	return result, nil
}

func (p *ProcessDump) WriteMemory(addr process.ProcessMemoryAddress, data []byte) error {
	return fmt.Errorf("WriteMemory not supported for ProcessDump")
}

func (p *ProcessDump) Save(dirname string) error {
	return fmt.Errorf("Save not supported for ProcessDump (already saved)")
}

func (p *ProcessDump) Load(dirname string) error {
	// Read metadata
	metadataPath := filepath.Join(dirname, "metadata.json")
	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to read metadata: %w", err)
	}

	var metadata struct {
		PID  process.ProcessID `json:"pid"`
		Name string            `json:"name"`
	}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return fmt.Errorf("failed to unmarshal metadata: %w", err)
	}
	p.PID = metadata.PID
	p.Name = metadata.Name

	// Read memory map
	mmPath := filepath.Join(dirname, "process_memory_map.json")
	mmBytes, err := os.ReadFile(mmPath)
	if err != nil {
		return fmt.Errorf("failed to read memory map: %w", err)
	}

	if err := json.Unmarshal(mmBytes, &p.MemoryMap); err != nil {
		return fmt.Errorf("failed to unmarshal memory map: %w", err)
	}

	// Sort memory map
	sort.Slice(p.MemoryMap, func(i, j int) bool {
		return p.MemoryMap[i].Address < p.MemoryMap[j].Address
	})

	// Load blobs
	for _, region := range p.MemoryMap {
		// Skip if not readable (logic from Save)
		// But we should check if file exists
		filename := filepath.Join(dirname, fmt.Sprintf("blob_0x%x_%d.bin", region.Address, region.Size))
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			continue // Blob not saved (e.g. too large or not readable)
		}

		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read blob %s: %w", filename, err)
		}

		p.Blobs[region.Address] = data
	}

	return nil
}

// Implement ProcessRead interface methods by delegating to ReadMemory or using helpers
// Since ProcessDump struct doesn't embed a helper, we implement them manually or copy.
// Or we can create a helper struct that implements ProcessRead given a ReadMemory func.
// For now, I'll implement a few key ones or just leave them as "not implemented" if the user didn't strictly ask for full interface on Dump.
// But Process interface requires them.
// I should probably use the same code as in process_linux/process_read_typed.go but adapted.
// Or better, make `process_linux` code generic.
// I'll copy the implementation for now to be safe and complete.

func (p *ProcessDump) ReadUINT8(addr process.ProcessMemoryAddress) (uint8, error) {
	data, err := p.ReadMemory(addr, 1)
	if err != nil {
		return 0, err
	}
	return data[0], nil
}

func (p *ProcessDump) ReadUINT16(addr process.ProcessMemoryAddress) (uint16, error) {
	data, err := p.ReadMemory(addr, 2)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(data), nil
}

func (p *ProcessDump) ReadUINT32(addr process.ProcessMemoryAddress) (uint32, error) {
	data, err := p.ReadMemory(addr, 4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(data), nil
}

func (p *ProcessDump) ReadUINT64(addr process.ProcessMemoryAddress) (uint64, error) {
	data, err := p.ReadMemory(addr, 8)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(data), nil
}

func (p *ProcessDump) ReadINT8(addr process.ProcessMemoryAddress) (int8, error) {
	data, err := p.ReadMemory(addr, 1)
	if err != nil {
		return 0, err
	}
	return int8(data[0]), nil
}

func (p *ProcessDump) ReadINT16(addr process.ProcessMemoryAddress) (int16, error) {
	data, err := p.ReadMemory(addr, 2)
	if err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(data)), nil
}

func (p *ProcessDump) ReadINT32(addr process.ProcessMemoryAddress) (int32, error) {
	data, err := p.ReadMemory(addr, 4)
	if err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(data)), nil
}

func (p *ProcessDump) ReadINT64(addr process.ProcessMemoryAddress) (int64, error) {
	data, err := p.ReadMemory(addr, 8)
	if err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(data)), nil
}

func (p *ProcessDump) ReadFLOAT32(addr process.ProcessMemoryAddress) (float32, error) {
	data, err := p.ReadMemory(addr, 4)
	if err != nil {
		return 0, err
	}
	bits := binary.LittleEndian.Uint32(data)
	return *(*float32)(unsafe.Pointer(&bits)), nil
}

func (p *ProcessDump) ReadFLOAT64(addr process.ProcessMemoryAddress) (float64, error) {
	data, err := p.ReadMemory(addr, 8)
	if err != nil {
		return 0, err
	}
	bits := binary.LittleEndian.Uint64(data)
	return *(*float64)(unsafe.Pointer(&bits)), nil
}

func (p *ProcessDump) ReadNTS(addr process.ProcessMemoryAddress, maxLength process.ProcessMemorySize) (string, error) {
	if maxLength == 0 {
		return "", nil
	}
	data, err := p.ReadMemory(addr, maxLength)
	if err != nil {
		return "", err
	}
	for i, b := range data {
		if b == 0 {
			return string(data[:i]), nil
		}
	}
	return string(data), nil
}

func (p *ProcessDump) ReadPOINTER(addr process.ProcessMemoryAddress) (process.ProcessMemoryAddress, error) {
	data, err := p.ReadMemory(addr, 8)
	if err != nil {
		return 0, err
	}
	return process.ProcessMemoryAddress(binary.LittleEndian.Uint64(data)), nil
}

func (p *ProcessDump) ReadPOINTER2(addr process.ProcessMemoryAddress) process.ProcessMemoryAddress {
	ptr, err := p.ReadPOINTER(addr)
	if err != nil {
		return 0
	}
	return ptr
}

func (p *ProcessDump) ReadPointers(base process.ProcessMemoryAddress, count int) (results []process.ProcessMemoryAddress, err error) {
	// Simplified implementation
	for i := 0; i < count; i++ {
		ptr, err := p.ReadPOINTER(base + process.ProcessMemoryAddress(i*8))
		if err != nil {
			return nil, err
		}
		results = append(results, ptr)
	}
	return results, nil
}

func (p *ProcessDump) ReadBlob(addr process.ProcessMemoryAddress, size process.ProcessMemorySize) (process.ProcessReadOffset, error) {
	data, err := p.ReadMemory(addr, size)
	if err != nil {
		return nil, err
	}
	return NewProcessBlob(addr, data), nil
}

func (p *ProcessDump) ReadBlobs(list []process.ProcessMemoryAddress, size process.ProcessMemorySize) []process.ReadBlobsResult {
	// Serial implementation
	results := make([]process.ReadBlobsResult, len(list))
	for i, addr := range list {
		blob, err := p.ReadBlob(addr, size)
		results[i] = process.ReadBlobsResult{Address: addr, Blob: blob, Err: err}
	}
	return results
}

func (p *ProcessDump) ReadPointerChain(base process.ProcessMemoryAddress, size process.ProcessMemorySize, offsets ...process.ProcessMemorySize) (process.ProcessReadOffset, error) {
	return nil, fmt.Errorf("not implemented")
}

func (p *ProcessDump) ReadPointerChainDebug(base process.ProcessMemoryAddress, size process.ProcessMemorySize, offsets ...process.ProcessMemorySize) (process.ProcessReadOffset, error) {
	return nil, fmt.Errorf("not implemented")
}

// MemoryScanner methods
func (p *ProcessDump) Scan(aob process.AOB) ([]process.ProcessMemoryAddress, error) {
	return nil, fmt.Errorf("Scan not implemented")
}

func (p *ProcessDump) ScanParallel(aob process.AOB, maxdop uint) ([]process.ProcessMemoryAddress, error) {
	return nil, fmt.Errorf("ScanParallel not implemented")
}

func (p *ProcessDump) ScanFirst(aob process.AOB) (process.ProcessMemoryAddress, error) {
	return 0, fmt.Errorf("ScanFirst not implemented")
}

func (p *ProcessDump) ScanFirstParallel(aob process.AOB, maxdop uint) (process.ProcessMemoryAddress, error) {
	return 0, fmt.Errorf("ScanFirstParallel not implemented")
}

func (p *ProcessDump) ScanInteger(value int64, size uint) ([]process.ProcessMemoryAddress, error) {
	return nil, fmt.Errorf("ScanInteger not implemented")
}

func (p *ProcessDump) ScanFloat(value float64, isFloat32 bool) ([]process.ProcessMemoryAddress, error) {
	return nil, fmt.Errorf("ScanFloat not implemented")
}

func (p *ProcessDump) ScanString(value string, isUTF16 bool) ([]process.ProcessMemoryAddress, error) {
	return nil, fmt.Errorf("ScanString not implemented")
}
