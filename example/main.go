package main

import (
	"fmt"
	"gomem/pod"
	"gomem/process"
)

// PlayerData represents the specific data for a player unit
type PlayerData struct {
	Name      [16]byte `pod:"char_array"`
	QuestPath uint64   `pod:"pointer,type=QuestPath"` // Example pointer
	// ... other fields
}

// UnitAny is a generic game unit structure
type UnitAny[T any] struct {
	Type      uint32
	TxtFileNo uint32
	UnitID    uint32
	Mode      uint32
	Data      *T `pod:"valid_pointer"` // This tells pod to recursively read T at this address
	Act       uint32
	ActPtr    uint64 `pod:"pointer"` // Just a pointer value, don't follow
	Seed      [2]uint32
	// ... other fields
}

func main() {
	// This example demonstrates how to read a UnitAny[PlayerData] from memory.
	// In a real scenario, you would obtain a process handle first.

	// 1. Get the process (e.g., by name)
	// finder := process_linux.NewProcessFinder()
	// pid, _ := finder.FindProcess("D2R.exe")
	// proc, _ := process_linux.NewWithPID(pid)

	// For demonstration, we'll use a mock or nil process (which will fail at runtime but shows the API)
	var proc process.Process = nil

	// 2. Assume we have the address of a UnitAny structure
	var unitAddr process.ProcessMemoryAddress = 0x12345678

	// 3. Read the structure
	// The pod.ReadT function will:
	// - Read the UnitAny struct
	// - Notice the 'Data' field has 'valid_pointer' tag
	// - Read the memory at the address stored in 'Data' into a new PlayerData struct
	// - Assign the pointer to the Data field
	unit, err := pod.ReadT[UnitAny[PlayerData]](proc, unitAddr)
	if err != nil {
		fmt.Printf("Failed to read unit: %v\n", err)
		return
	}

	// 4. Access the data
	fmt.Printf("Unit ID: %d\n", unit.UnitID)
	if unit.Data != nil {
		// Convert char array to string
		name := string(unit.Data.Name[:])
		fmt.Printf("Player Name: %s\n", name)
	}

	// 5. Print the structure with debug info
	pod.PrintPodStruct(proc, unit, nil)
}
