package process

import (
	"gomem/process/memory_map"
)

var BASEADDRESS = ProcessMemoryAddress(0x140000000)

// Process is the interface that defines operations for interacting with a system process
type Process interface {
	// Open opens a process with the given PID for memory operations
	Open(pid ProcessID) error

	// Close closes the process and releases resources
	Close() error

	// GetPID returns the process ID
	GetPID() ProcessID

	// UpdateMemoryMap refreshes the memory map for the process
	UpdateMemoryMap() error

	// IsValidAddress checks if the given memory address is valid and readable
	IsValidAddress(addr ProcessMemoryAddress) bool

	// GetMemoryMap returns a copy of the current memory map
	GetMemoryMap() ([]memory_map.MemoryMapItem, error)

	// ReadMemory reads memory from the process at the specified address
	ReadMemory(addr ProcessMemoryAddress, size ProcessMemorySize) ([]byte, error)

	// WriteMemory writes data to the process memory at the specified address
	WriteMemory(addr ProcessMemoryAddress, data []byte) error

	// Save saves the process memory and metadata to a directory
	Save(dirname string) error

	// Load loads the process memory and metadata from a directory
	Load(dirname string) error

	// Memory scanning operations
	MemoryScanner

	// Typed memory reading operations
	ProcessRead
}

// ProcessRead defines typed read operations for process memory
type ProcessRead interface {
	// ReadUINT8 reads an unsigned 8-bit integer from the specified address
	ReadUINT8(addr ProcessMemoryAddress) (uint8, error)

	// ReadUINT16 reads an unsigned 16-bit integer from the specified address
	ReadUINT16(addr ProcessMemoryAddress) (uint16, error)

	// ReadUINT32 reads an unsigned 32-bit integer from the specified address
	ReadUINT32(addr ProcessMemoryAddress) (uint32, error)

	// ReadUINT64 reads an unsigned 64-bit integer from the specified address
	ReadUINT64(addr ProcessMemoryAddress) (uint64, error)

	// ReadINT8 reads a signed 8-bit integer from the specified address
	ReadINT8(addr ProcessMemoryAddress) (int8, error)

	// ReadINT16 reads a signed 16-bit integer from the specified address
	ReadINT16(addr ProcessMemoryAddress) (int16, error)

	// ReadINT32 reads a signed 32-bit integer from the specified address
	ReadINT32(addr ProcessMemoryAddress) (int32, error)

	// ReadINT64 reads a signed 64-bit integer from the specified address
	ReadINT64(addr ProcessMemoryAddress) (int64, error)

	// ReadFLOAT32 reads a 32-bit floating point number from the specified address
	ReadFLOAT32(addr ProcessMemoryAddress) (float32, error)

	// ReadFLOAT64 reads a 64-bit floating point number from the specified address
	ReadFLOAT64(addr ProcessMemoryAddress) (float64, error)

	// ReadNTS reads a null-terminated string from the specified address with a maximum length
	ReadNTS(addr ProcessMemoryAddress, maxLength ProcessMemorySize) (string, error)

	// ReadPOINTER reads a pointer value from the specified address
	ReadPOINTER(addr ProcessMemoryAddress) (ProcessMemoryAddress, error)

	// ReadPOINTER reads a pointer value from the specified address, zero on error
	ReadPOINTER2(addr ProcessMemoryAddress) ProcessMemoryAddress

	// ReadPointers reads a list of pointers from the specified address
	ReadPointers(base ProcessMemoryAddress, count int) (results []ProcessMemoryAddress, err error)

	// ReadBlob reads a blob of memory from the specified address with the given size
	ReadBlob(addr ProcessMemoryAddress, size ProcessMemorySize) (ProcessReadOffset, error)

	// ReadBlobs reads multiple blobs of memory from the specified addresses with the given size
	ReadBlobs(list []ProcessMemoryAddress, size ProcessMemorySize) []ReadBlobsResult

	ReadPointerChain(base ProcessMemoryAddress, size ProcessMemorySize, offsets ...ProcessMemorySize) (ProcessReadOffset, error)
	ReadPointerChainDebug(base ProcessMemoryAddress, size ProcessMemorySize, offsets ...ProcessMemorySize) (ProcessReadOffset, error)
}

// ProcessReadOffset combines both ProcessRead and ProcessOffset interfaces
type ProcessReadOffset interface {
	ProcessRead
	ProcessOffset
}

// ProcessOffset defines typed Offset operations for process memory
type ProcessOffset interface {
	// Data returns the raw data read from the process memory
	Data() []byte

	// OffsetUINT8 Offsets an unsigned 8-bit integer from the specified address
	OffsetUINT8(offset ProcessMemoryAddress) (uint8, error)

	// OffsetUINT16 Offsets an unsigned 16-bit integer from the specified address
	OffsetUINT16(offset ProcessMemoryAddress) (uint16, error)

	// OffsetUINT32 Offsets an unsigned 32-bit integer from the specified address
	OffsetUINT32(offset ProcessMemoryAddress) (uint32, error)

	// OffsetUINT64 Offsets an unsigned 64-bit integer from the specified address
	OffsetUINT64(offset ProcessMemoryAddress) (uint64, error)

	// OffsetINT8 Offsets a signed 8-bit integer from the specified address
	OffsetINT8(offset ProcessMemoryAddress) (int8, error)

	// OffsetINT16 Offsets a signed 16-bit integer from the specified address
	OffsetINT16(offset ProcessMemoryAddress) (int16, error)

	// OffsetINT32 Offsets a signed 32-bit integer from the specified address
	OffsetINT32(offset ProcessMemoryAddress) (int32, error)

	// OffsetINT64 Offsets a signed 64-bit integer from the specified address
	OffsetINT64(offset ProcessMemoryAddress) (int64, error)

	// OffsetFLOAT32 Offsets a 32-bit floating point number from the specified address
	OffsetFLOAT32(offset ProcessMemoryAddress) (float32, error)

	// OffsetFLOAT64 Offsets a 64-bit floating point number from the specified address
	OffsetFLOAT64(offset ProcessMemoryAddress) (float64, error)

	// OffsetNTS Offsets a null-terminated string from the specified address with a maximum length
	OffsetNTS(offset ProcessMemoryAddress, maxLength ProcessMemorySize) (string, error)

	// OffsetPOINTER Offsets a pointer value from the specified address
	OffsetPOINTER(offset ProcessMemoryAddress) (ProcessMemoryAddress, error)

	// OffsetPOINTER2 Offsets a pointer value from the specified address, zero on error
	OffsetPOINTER2(offset ProcessMemoryAddress) ProcessMemoryAddress

	// OffsetBlob Offsets a blob of memory from the specified address with the given size
	OffsetBlob(offset ProcessMemoryAddress, size ProcessMemorySize) (ProcessReadOffset, error)
}

// MemoryScanner defines operations for searching patterns in process memory
type MemoryScanner interface {
	// Scan searches for a pattern in process memory
	Scan(aob AOB) ([]ProcessMemoryAddress, error)

	// ScanParallel searches for a pattern in process memory using parallel scanning
	ScanParallel(aob AOB, maxdop uint) ([]ProcessMemoryAddress, error)

	// ScanFirst searches for the first occurrence of a pattern in process memory
	ScanFirst(aob AOB) (ProcessMemoryAddress, error)

	// ScanFirstParallel searches for the first occurrence of a pattern using parallel scanning
	ScanFirstParallel(aob AOB, maxdop uint) (ProcessMemoryAddress, error)

	// ScanInteger searches for an integer value in memory
	ScanInteger(value int64, size uint) ([]ProcessMemoryAddress, error)

	// ScanFloat searches for a float value in memory
	ScanFloat(value float64, isFloat32 bool) ([]ProcessMemoryAddress, error)

	// ScanString searches for a string in memory
	ScanString(value string, isUTF16 bool) ([]ProcessMemoryAddress, error)
}
