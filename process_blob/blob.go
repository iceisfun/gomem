package process_blob

import (
	"encoding/binary"
	"errors"
	"gomem/process"
	"unsafe"
)

type ProcessBlob struct {
	baseaddress process.ProcessMemoryAddress
	data        []byte
}

var _ process.ProcessRead = (*ProcessBlob)(nil)
var _ process.ProcessOffset = (*ProcessBlob)(nil)
var _ process.ProcessReadOffset = (*ProcessBlob)(nil)

func NewProcessBlob(baseAddress process.ProcessMemoryAddress, data []byte) *ProcessBlob {
	return &ProcessBlob{
		baseaddress: baseAddress,
		data:        data,
	}
}

func (p *ProcessBlob) Data() []byte {
	return p.data
}

func (p *ProcessBlob) ReadMemory(addr process.ProcessMemoryAddress, size process.ProcessMemorySize) ([]byte, error) {
	if addr < p.baseaddress || process.ProcessMemorySize(addr)+size > process.ProcessMemorySize(p.baseaddress)+process.ProcessMemorySize(len(p.data)) {
		return nil, errors.New("address out of bounds")
	}
	offset := addr - p.baseaddress
	return p.data[offset : uint64(offset)+uint64(size)], nil
}

// ReadUINT8 reads an unsigned 8-bit integer from the specified address
func (p *ProcessBlob) ReadUINT8(addr process.ProcessMemoryAddress) (uint8, error) {
	data, err := p.ReadMemory(addr, 1)
	if err != nil {
		return 0, err
	}
	return data[0], nil
}

// ReadUINT16 reads an unsigned 16-bit integer from the specified address
func (p *ProcessBlob) ReadUINT16(addr process.ProcessMemoryAddress) (uint16, error) {
	data, err := p.ReadMemory(addr, 2)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(data), nil
}

// ReadUINT32 reads an unsigned 32-bit integer from the specified address
func (p *ProcessBlob) ReadUINT32(addr process.ProcessMemoryAddress) (uint32, error) {
	data, err := p.ReadMemory(addr, 4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(data), nil
}

// ReadUINT64 reads an unsigned 64-bit integer from the specified address
func (p *ProcessBlob) ReadUINT64(addr process.ProcessMemoryAddress) (uint64, error) {
	data, err := p.ReadMemory(addr, 8)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(data), nil
}

// ReadINT8 reads a signed 8-bit integer from the specified address
func (p *ProcessBlob) ReadINT8(addr process.ProcessMemoryAddress) (int8, error) {
	data, err := p.ReadMemory(addr, 1)
	if err != nil {
		return 0, err
	}
	return int8(data[0]), nil
}

// ReadINT16 reads a signed 16-bit integer from the specified address
func (p *ProcessBlob) ReadINT16(addr process.ProcessMemoryAddress) (int16, error) {
	data, err := p.ReadMemory(addr, 2)
	if err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(data)), nil
}

// ReadINT32 reads a signed 32-bit integer from the specified address
func (p *ProcessBlob) ReadINT32(addr process.ProcessMemoryAddress) (int32, error) {
	data, err := p.ReadMemory(addr, 4)
	if err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(data)), nil
}

// ReadINT64 reads a signed 64-bit integer from the specified address
func (p *ProcessBlob) ReadINT64(addr process.ProcessMemoryAddress) (int64, error) {
	data, err := p.ReadMemory(addr, 8)
	if err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(data)), nil
}

// ReadFLOAT32 reads a 32-bit floating point number from the specified address
func (p *ProcessBlob) ReadFLOAT32(addr process.ProcessMemoryAddress) (float32, error) {
	data, err := p.ReadMemory(addr, 4)
	if err != nil {
		return 0, err
	}
	bits := binary.LittleEndian.Uint32(data)
	return *(*float32)(unsafe.Pointer(&bits)), nil
}

// ReadFLOAT64 reads a 64-bit floating point number from the specified address
func (p *ProcessBlob) ReadFLOAT64(addr process.ProcessMemoryAddress) (float64, error) {
	data, err := p.ReadMemory(addr, 8)
	if err != nil {
		return 0, err
	}
	bits := binary.LittleEndian.Uint64(data)
	return *(*float64)(unsafe.Pointer(&bits)), nil
}

// ReadNTS reads a null-terminated string from the specified address with a maximum length
func (p *ProcessBlob) ReadNTS(addr process.ProcessMemoryAddress, maxLength process.ProcessMemorySize) (string, error) {
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
func (p *ProcessBlob) ReadPOINTER(addr process.ProcessMemoryAddress) (process.ProcessMemoryAddress, error) {
	const ptrSize = 8 // Assuming 64-bit architecture

	if addr == 0 {
		return 0, errors.New("invalid address: 0x0")
	}

	data, err := p.ReadMemory(addr, ptrSize)
	if err != nil {
		return 0, err
	}

	// Read as uint64 for 64-bit pointers
	ptr := binary.LittleEndian.Uint64(data)
	return process.ProcessMemoryAddress(ptr), nil
}

func (p *ProcessBlob) ReadPOINTER2(addr process.ProcessMemoryAddress) process.ProcessMemoryAddress {
	if addr == 0 {
		return 0 // Return zero if the address is invalid
	}

	if addr > process.ProcessMemoryAddress(0x7FFFFFFFFFFF) {
		return 0 // Return zero if the address is out of bounds
	}

	ptr, err := p.ReadPOINTER(addr)
	if err != nil {
		return 0 // Return zero on error
	}
	return ptr
}

// ReadBlob reads a blob of memory from the specified address with the given size
func (p *ProcessBlob) ReadBlob(addr process.ProcessMemoryAddress, size process.ProcessMemorySize) (process.ProcessReadOffset, error) {
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

	return NewProcessBlob(addr, data[:size]), nil
}

func (p *ProcessBlob) ReadPointers(base process.ProcessMemoryAddress, count int) (results []process.ProcessMemoryAddress, err error) {
	panic("not implemented")
}

func (p *ProcessBlob) ReadBlobs(list []process.ProcessMemoryAddress, size process.ProcessMemorySize) []process.ReadBlobsResult {
	panic("not implemented")
}

// Offset methods for ProcessOffset interface

// OffsetUINT8 returns an unsigned 8-bit integer with offset from the specified address
func (p *ProcessBlob) OffsetUINT8(offset process.ProcessMemoryAddress) (uint8, error) {
	addr := p.baseaddress + offset
	value, err := p.ReadUINT8(addr)
	return value, err
}

// OffsetUINT16 returns an unsigned 16-bit integer with offset from the specified address
func (p *ProcessBlob) OffsetUINT16(offset process.ProcessMemoryAddress) (uint16, error) {
	addr := p.baseaddress + offset
	value, err := p.ReadUINT16(addr)
	return value, err
}

// OffsetUINT32 returns an unsigned 32-bit integer with offset from the specified address
func (p *ProcessBlob) OffsetUINT32(offset process.ProcessMemoryAddress) (uint32, error) {
	addr := p.baseaddress + offset
	value, err := p.ReadUINT32(addr)
	return value, err
}

// OffsetUINT64 returns an unsigned 64-bit integer with offset from the specified address
func (p *ProcessBlob) OffsetUINT64(offset process.ProcessMemoryAddress) (uint64, error) {
	addr := p.baseaddress + offset
	value, err := p.ReadUINT64(addr)
	return value, err
}

// OffsetINT8 returns a signed 8-bit integer with offset from the specified address
func (p *ProcessBlob) OffsetINT8(offset process.ProcessMemoryAddress) (int8, error) {
	addr := p.baseaddress + offset
	value, err := p.ReadINT8(addr)
	return value, err
}

// OffsetINT16 returns a signed 16-bit integer with offset from the specified address
func (p *ProcessBlob) OffsetINT16(offset process.ProcessMemoryAddress) (int16, error) {
	addr := p.baseaddress + offset
	value, err := p.ReadINT16(addr)
	return value, err
}

// OffsetINT32 returns a signed 32-bit integer with offset from the specified address
func (p *ProcessBlob) OffsetINT32(offset process.ProcessMemoryAddress) (int32, error) {
	addr := p.baseaddress + offset
	value, err := p.ReadINT32(addr)
	return value, err
}

// OffsetINT64 returns a signed 64-bit integer with offset from the specified address
func (p *ProcessBlob) OffsetINT64(offset process.ProcessMemoryAddress) (int64, error) {
	addr := p.baseaddress + offset
	value, err := p.ReadINT64(addr)
	return value, err
}

// OffsetFLOAT32 returns a 32-bit floating point number with offset from the specified address
func (p *ProcessBlob) OffsetFLOAT32(offset process.ProcessMemoryAddress) (float32, error) {
	addr := p.baseaddress + offset
	value, err := p.ReadFLOAT32(addr)
	return value, err
}

// OffsetFLOAT64 returns a 64-bit floating point number with offset from the specified address
func (p *ProcessBlob) OffsetFLOAT64(offset process.ProcessMemoryAddress) (float64, error) {
	addr := p.baseaddress + offset
	value, err := p.ReadFLOAT64(addr)
	return value, err
}

// OffsetNTS returns a null-terminated string with offset from the specified address with a maximum length
func (p *ProcessBlob) OffsetNTS(offset process.ProcessMemoryAddress, maxLength process.ProcessMemorySize) (string, error) {
	addr := p.baseaddress + offset
	value, err := p.ReadNTS(addr, maxLength)
	return value, err
}

// OffsetPOINTER returns a pointer value with offset from the specified address
func (p *ProcessBlob) OffsetPOINTER(offset process.ProcessMemoryAddress) (process.ProcessMemoryAddress, error) {
	addr := p.baseaddress + offset
	value, err := p.ReadPOINTER(addr)
	return value, err
}

// OffsetPOINTER2 returns a pointer value with offset from the specified address, zero on error
func (p *ProcessBlob) OffsetPOINTER2(offset process.ProcessMemoryAddress) process.ProcessMemoryAddress {
	addr := p.baseaddress + offset
	value := p.ReadPOINTER2(addr)
	return value
}

// OffsetBlob returns a blob of memory with offset from the specified address with the given size
func (p *ProcessBlob) OffsetBlob(offset process.ProcessMemoryAddress, size process.ProcessMemorySize) (process.ProcessReadOffset, error) {
	addr := p.baseaddress + offset
	blob, err := p.ReadBlob(addr, size)
	if err != nil {
		return nil, err
	}
	return blob, nil
}
