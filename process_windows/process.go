//go:build windows

package process_windows

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"

	"gomem/process"
	"gomem/process/memory_map"

	"gomem/coloransi"

	"github.com/Moonlight-Companies/gologger/logger"
)

var (
	modkernel32           = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess       = modkernel32.NewProc("OpenProcess")
	procReadProcessMemory = modkernel32.NewProc("ReadProcessMemory")
	procCloseHandle       = modkernel32.NewProc("CloseHandle")
	procVirtualQueryEx    = modkernel32.NewProc("VirtualQueryEx")
)

const (
	PROCESS_ALL_ACCESS        = 0x1F0FFF
	PROCESS_VM_READ           = 0x0010
	PROCESS_QUERY_INFORMATION = 0x0400
)

// WindowsProcess implements the process.Process interface for Windows systems
type WindowsProcess struct {
	pid    process.ProcessID
	handle syscall.Handle
	log    *logger.Logger
	mm     []memory_map.MemoryMapItem
	mu     sync.Mutex
}

// New creates a new WindowsProcess instance
func New() process.Process {
	return &WindowsProcess{
		log: logger.NewLogger(coloransi.Color(coloransi.Red, coloransi.ColorOrange, "process-not-open")),
	}
}

// NewWithPID creates a new WindowsProcess instance and opens it with the given PID
func NewWithPID(pid process.ProcessID) (process.Process, error) {
	p := &WindowsProcess{}
	err := p.Open(pid)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *WindowsProcess) Open(pid process.ProcessID) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	handle, _, err := procOpenProcess.Call(uintptr(PROCESS_ALL_ACCESS), 0, uintptr(pid))
	if handle == 0 {
		return fmt.Errorf("OpenProcess failed: %v", err)
	}

	p.pid = pid
	p.handle = syscall.Handle(handle)
	p.log = logger.NewLogger(coloransi.Color(coloransi.ColorPurple, coloransi.ColorOrange, fmt.Sprintf("process-%d", pid)))

	// Initialize memory map
	if err := p.updateMemoryMapInternal(); err != nil {
		p.log.Warn("Failed to initialize memory map: ", err)
	}

	p.log.Infoln("Process opened")
	return nil
}

func (p *WindowsProcess) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.handle != 0 {
		ret, _, err := procCloseHandle.Call(uintptr(p.handle))
		if ret == 0 {
			return fmt.Errorf("CloseHandle failed: %v", err)
		}
		p.handle = 0
	}

	p.pid = 0
	p.mm = nil
	p.log = logger.NewLogger(coloransi.Color(coloransi.Red, coloransi.ColorOrange, "process-not-open"))
	p.log.Infoln("Process closed")

	return nil
}

func (p *WindowsProcess) GetPID() process.ProcessID {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pid
}

func (p *WindowsProcess) UpdateMemoryMap() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.updateMemoryMapInternal()
}

func (p *WindowsProcess) updateMemoryMapInternal() error {
	if p.handle == 0 {
		return fmt.Errorf("process not opened")
	}

	// TODO: Implement VirtualQueryEx loop to populate p.mm
	// For now, we leave it empty or implement a basic version
	return nil
}

func (p *WindowsProcess) IsValidAddress(addr process.ProcessMemoryAddress) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Check against memory map
	return memory_map.IsValidAddress(uint64(addr), p.mm)
}

func (p *WindowsProcess) GetMemoryMap() ([]memory_map.MemoryMapItem, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == 0 {
		return nil, fmt.Errorf("process not opened")
	}
	result := make([]memory_map.MemoryMapItem, len(p.mm))
	copy(result, p.mm)
	return result, nil
}

func (p *WindowsProcess) ReadMemory(addr process.ProcessMemoryAddress, size process.ProcessMemorySize) ([]byte, error) {
	if size == 0 {
		return []byte{}, nil
	}

	p.mu.Lock()
	handle := p.handle
	p.mu.Unlock()

	if handle == 0 {
		return nil, fmt.Errorf("process not opened")
	}

	buf := make([]byte, size)
	var bytesRead uintptr
	ret, _, err := procReadProcessMemory.Call(
		uintptr(handle),
		uintptr(addr),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(size),
		uintptr(unsafe.Pointer(&bytesRead)),
	)

	if ret == 0 {
		return nil, fmt.Errorf("ReadProcessMemory failed: %v", err)
	}

	if bytesRead != uintptr(size) {
		return nil, fmt.Errorf("read incomplete: expected %d, got %d", size, bytesRead)
	}

	return buf, nil
}

func (p *WindowsProcess) WriteMemory(addr process.ProcessMemoryAddress, data []byte) error {
	return fmt.Errorf("WriteMemory not implemented")
}

func (p *WindowsProcess) Save(dirname string) error {
	return fmt.Errorf("Save not implemented")
}

func (p *WindowsProcess) Load(dirname string) error {
	return fmt.Errorf("Load not implemented")
}

// MemoryScanner implementation (placeholders)
func (p *WindowsProcess) Scan(aob process.AOB) ([]process.ProcessMemoryAddress, error) {
	return nil, fmt.Errorf("Scan not implemented")
}

func (p *WindowsProcess) ScanParallel(aob process.AOB, maxdop uint) ([]process.ProcessMemoryAddress, error) {
	return nil, fmt.Errorf("ScanParallel not implemented")
}

func (p *WindowsProcess) ScanFirst(aob process.AOB) (process.ProcessMemoryAddress, error) {
	return 0, fmt.Errorf("ScanFirst not implemented")
}

func (p *WindowsProcess) ScanFirstParallel(aob process.AOB, maxdop uint) (process.ProcessMemoryAddress, error) {
	return 0, fmt.Errorf("ScanFirstParallel not implemented")
}

func (p *WindowsProcess) ScanInteger(value int64, size uint) ([]process.ProcessMemoryAddress, error) {
	return nil, fmt.Errorf("ScanInteger not implemented")
}

func (p *WindowsProcess) ScanFloat(value float64, isFloat32 bool) ([]process.ProcessMemoryAddress, error) {
	return nil, fmt.Errorf("ScanFloat not implemented")
}

func (p *WindowsProcess) ScanString(value string, isUTF16 bool) ([]process.ProcessMemoryAddress, error) {
	return nil, fmt.Errorf("ScanString not implemented")
}
