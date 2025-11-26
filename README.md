# gomem

`gomem` is a Go library for reading and writing process memory, designed for process memory analysis and reverse engineering. It provides a high-level, type-safe interface for interacting with memory, supporting both Linux and Windows.

## Features

- **Cross-Platform**: Supports Linux (via `ptrace` / `process_vm_readv`) and Windows (via `ReadProcessMemory`).
- **Type-Safe Memory Access**: Use Go structs and generics to read complex data structures directly from memory.
- **POD (Plain Old Data) Support**: Automatically handle struct layout, padding, and pointer chasing.
- **Memory Scanning**: Pattern scanning (AOB) and value searching.
- **Dump & Load**: Save process memory to disk for offline analysis and testing.
- **Path Reading**: Read values at the end of multi-level pointer chains.

## Concepts

### Process Abstraction
The `process.Process` interface abstracts the underlying OS-specific mechanisms. You can attach to a running process by PID or name, or load a memory dump.

### Custom Types
`gomem` uses custom types to ensure type safety and clarity:
- `ProcessMemoryAddress` (uint64): Represents a virtual memory address.
- `ProcessMemorySize` (uint64): Represents the size of a memory region.
- `ProcessReadOffset`: Represents a chunk of read memory, allowing safe slicing and offset calculations.

### POD (Plain Old Data)
The `pod` package allows you to map Go structs to process memory. It supports:
- **`pod` Tags**: Control how fields are read (e.g., `type=int32`, `char_array`, `valid_pointer`).
- **Pointers**: Automatically follow pointers defined in structs (e.g., `*FlagData`).
- **Generics**: Use generic structs to model complex memory layouts.

## Installation

```bash
go get github.com/iceisfun/gomem
```

## Usage Examples

### 1. Basic Memory Reading

```go
package main

import (
	"fmt"
	"gomem/process"
	"gomem/process_linux" // or process_windows
)

func main() {
	// Attach to a process
	proc, err := process_linux.NewProcessFromPID(1234)
	if err != nil {
		panic(err)
	}
	defer proc.Close()

	// Read an int32 at a specific address
	var val int32
	val, err = process.Read[int32](proc, 0x12345678)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Value: %d\n", val)
}
```

### 2. Reading Complex Structs (POD)

Define your structs with `pod` tags to match the target process's memory layout.

```go
type Player struct {
	Health    int32     `pod:"type=int32"`
	Armor     int32     `pod:"type=int32"`
	Name      [32]byte  `pod:"char_array"`
	WeaponPtr *Weapon   `pod:"valid_pointer"`
}

type Weapon struct {
	Ammo      int32     `pod:"type=int32"`
	Damage    float32   `pod:"type=float32"`
}

func readPlayer(proc process.Process, addr process.ProcessMemoryAddress) {
	player, err := pod.ReadT[Player](proc, addr)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Player: %s, Health: %d, Ammo: %d\n", 
		string(player.Name[:]), player.Health, player.WeaponPtr.Ammo)
}
```

### 3. Path Reading

Read a value at the end of a pointer chain (e.g., `Base -> Ptr1 -> Ptr2 -> Value`).

```go
// Read a float32 at Base + 0x10 -> + 0x20 -> + 0x04
val, err := process.ReadPath[float32](proc, baseAddr, 0x10, 0x20, 0x04)
```

### 4. Searching

Search for values or patterns in memory.

```go
import "gomem/search"

// Search for a specific float value
results, err := search.Search(proc, baseAddr,
	search.WithMaxDepth(2),
	search.WithSearchForType(float32(3.14)),
)

for _, res := range results {
	fmt.Printf("Found at path: %v\n", res.Path)
}
```

## Platforms

- **Linux**: Requires `ptrace` permissions. Ensure `/proc/sys/kernel/yama/ptrace_scope` is 0 or the target process allows tracing.
- **Windows**: Requires Administrator privileges to open processes with `PROCESS_ALL_ACCESS`.

## CLI Tools

`gomem` includes several CLI tools for quick analysis:
- `process_dump_save`: Save process memory to disk.
- `process_dump_load`: Load and inspect a memory dump.
- `process_aob`: Scan for Array of Bytes (AOB) patterns.
- `process_test_pod`: Example tool demonstrating POD reading and searching.
