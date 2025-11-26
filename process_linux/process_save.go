//go:build linux

package process_linux

import (
	"encoding/json"
	"fmt"
	"gomem/process"

	"os"
	"path/filepath"
	"strconv"
	"time"

	"gomem/process/memory_map"
)

// Save saves the process memory and metadata to a directory
func (p *LinuxProcess) Save(dirname string) error {
	fmt.Printf("Save: Starting with directory %s\n", dirname)

	// Create the output directory without holding the lock
	if err := os.MkdirAll(dirname, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// First get the necessary information under lock
	p.mu.Lock()
	fmt.Printf("Save: Acquired mutex for initial data\n")

	// Check if process is opened
	if p.pid == 0 {
		p.mu.Unlock() // Don't forget to unlock before returning
		return fmt.Errorf("process not opened")
	}

	// Get a copy of the PID
	pid := p.pid

	// Log under lock
	p.log.Infoln("Saving process to directory:", dirname)

	// Release the lock while doing external operations
	p.mu.Unlock()
	fmt.Printf("Save: Released mutex for external operations\n")

	// Get process name using ps command without holding the lock
	procInfo, err := findProcessByPID(pid)
	name := "unknown"
	if err == nil && procInfo != nil {
		name = procInfo.Name
	}

	// Save metadata (process name and PID)
	metadata := struct {
		PID  process.ProcessID `json:"pid"`
		Name string            `json:"name"`
	}{
		PID:  pid,
		Name: name,
	}

	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dirname, "metadata.json"), metadataJSON, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	// Update memory map without a long-held lock
	if err := p.UpdateMemoryMap(); err != nil {
		return fmt.Errorf("failed to update memory map: %w", err)
	}

	// Get a copy of the memory map under lock
	p.mu.Lock()
	mmCopy := make([]memory_map.MemoryMapItem, len(p.mm))
	copy(mmCopy, p.mm)
	p.mu.Unlock()

	// Serialize the memory map without holding the lock
	memoryMapJSON, err := json.MarshalIndent(mmCopy, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal memory map: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dirname, "process_memory_map.json"), memoryMapJSON, 0644); err != nil {
		return fmt.Errorf("failed to write memory map file: %w", err)
	}

	// Save memory regions
	savedCount := 0
	errorCount := 0

	fmt.Printf("Save: Total memory regions to process: %d\n", len(mmCopy))

	// Use a timeout channel to prevent hanging indefinitely
	timeoutChan := make(chan bool, 1)
	go func() {
		// Set a reasonable timeout (30 seconds)
		time.Sleep(30 * time.Second)
		timeoutChan <- true
	}()

	regionTypeStats := map[string]int{
		"skipped_non_readable": 0,
		"skipped_too_large":    0,
		"read_error":           0,
		"write_error":          0,
		"saved":                0,
		"timeout":              0,
	}

	for i, region := range mmCopy {
		// Check for timeout
		select {
		case <-timeoutChan:
			fmt.Printf("TIMEOUT: Save operation is taking too long, aborting\n")
			regionTypeStats["timeout"] = 1
			return fmt.Errorf("save operation timed out after 30 seconds")
		default:
			// Continue processing
		}

		fmt.Printf("Processing region %d/%d: Address 0x%x, Size %d, Perms %s\n",
			i+1, len(mmCopy), region.Address, region.Size, region.Perms)

		// Skip non-readable regions
		if !isReadablePerms(region.Perms) {
			fmt.Printf("  - Skipping non-readable region (perms: %s)\n", region.Perms)
			regionTypeStats["skipped_non_readable"]++
			continue
		}

		// Skip regions that are too large
		if region.Size > 100*1024*1024 { // 100 MB
			fmt.Printf("  - Skipping large region: %d MB\n", region.Size/1024/1024)
			p.log.Infoln("Skipping large region at", fmt.Sprintf("%x", region.Address),
				"(size:", region.Size/1024/1024, "MB)")
			regionTypeStats["skipped_too_large"]++
			continue
		}

		// We save all memory regions to ensure complete dumps
		// This allows for examining any valid memory address in the dump

		// Read memory with timeout channel
		fmt.Printf("  - Reading %d bytes from 0x%x\n", region.Size, region.Address)

		// Start a timer for this memory read
		readStart := time.Now()

		// Read memory - this is where it's likely hanging
		data, err := p.ReadMemory(process.ProcessMemoryAddress(region.Address), process.ProcessMemorySize(region.Size))

		readDuration := time.Since(readStart)
		fmt.Printf("  - Read operation took %v\n", readDuration)

		if err != nil {
			fmt.Printf("  - ERROR reading memory: %v\n", err)
			p.log.Infoln("Failed to read memory region at", fmt.Sprintf("%x", region.Address), ":", err)
			errorCount++
			regionTypeStats["read_error"]++
			continue
		}

		fmt.Printf("  - Successfully read %d bytes\n", len(data))

		// Save to file
		filename := filepath.Join(dirname, fmt.Sprintf("blob_0x%x_%d.bin", region.Address, region.Size))
		fmt.Printf("  - Writing to file: %s\n", filename)

		writeStart := time.Now()

		if err := os.WriteFile(filename, data, 0644); err != nil {
			fmt.Printf("  - ERROR writing file: %v\n", err)
			p.log.Infoln("Failed to write memory file for region at", fmt.Sprintf("%x", region.Address), ":", err)
			errorCount++
			regionTypeStats["write_error"]++
			continue
		}

		writeDuration := time.Since(writeStart)
		fmt.Printf("  - Write operation took %v\n", writeDuration)

		fmt.Printf("  - Successfully saved region to file\n")
		savedCount++
		regionTypeStats["saved"]++
	}

	fmt.Printf("Region statistics:\n")
	fmt.Printf("  - Skipped non-readable: %d\n", regionTypeStats["skipped_non_readable"])
	fmt.Printf("  - Skipped too large: %d\n", regionTypeStats["skipped_too_large"])
	fmt.Printf("  - Read errors: %d\n", regionTypeStats["read_error"])
	fmt.Printf("  - Write errors: %d\n", regionTypeStats["write_error"])
	fmt.Printf("  - Successfully saved: %d\n", regionTypeStats["saved"])

	// Acquire lock just for logging
	p.mu.Lock()
	p.log.Infoln("Process dump saved successfully:", savedCount, "regions saved,", errorCount, "errors")
	p.mu.Unlock()

	return nil
}

// Load always returns an error for LinuxProcess as loading is only supported by ProcessDump
func (p *LinuxProcess) Load(dirname string) error {
	return fmt.Errorf("loading from a dump is not supported by LinuxProcess, use ProcessDump instead")
}

// Helper function to find a process by PID
func findProcessByPID(pid process.ProcessID) (*process.ProcessInfo, error) {
	// Create the proc filesystem path for the process
	procPath := filepath.Join("/proc", strconv.Itoa(int(pid)))

	// Check if the process exists
	if _, err := os.Stat(procPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("process with PID %d does not exist", pid)
	}

	// Read the process name from /proc/[pid]/comm
	commPath := filepath.Join(procPath, "comm")
	commData, err := os.ReadFile(commPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read process name: %w", err)
	}

	// Remove trailing newline and create ProcessInfo
	name := string(commData)
	if len(name) > 0 && name[len(name)-1] == '\n' {
		name = name[:len(name)-1]
	}

	procInfo := &process.ProcessInfo{
		PID:  pid,
		Name: name,
	}

	return procInfo, nil
}
