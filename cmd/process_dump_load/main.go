package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gomem/hexdump"
	"gomem/process"
	"gomem/process_blob"
)

func main() {
	fromFlag := flag.String("from", "", "Directory containing the dump")
	addrFlag := flag.String("addr", "", "Address to read from (hex)")
	sizeFlag := flag.Int("size", 256, "Number of bytes to hexdump")
	flag.Parse()

	if *fromFlag == "" {
		fmt.Println("Error: --from is required")
		flag.Usage()
		os.Exit(1)
	}

	// Load the dump
	dump := process_blob.NewProcessDump()
	if err := dump.Load(*fromFlag); err != nil {
		fmt.Printf("Error loading dump from %s: %v\n", *fromFlag, err)
		os.Exit(1)
	}

	fmt.Printf("Loaded dump from %s\n", *fromFlag)
	fmt.Printf("Process Name: %s\n", dump.Name)
	fmt.Printf("PID: %d\n", dump.PID)
	fmt.Printf("Memory Regions: %d\n", len(dump.MemoryMap))

	// If no address is specified, just print summary and exit
	if *addrFlag == "" {
		fmt.Println("\nMemory Map:")
		for _, region := range dump.MemoryMap {
			fmt.Printf("  %016x - %016x (%s) %d bytes\n",
				region.Address, region.Address+uint64(region.Size), region.Perms, region.Size)
		}
		return
	}

	// Parse address
	addrStr := *addrFlag
	if strings.HasPrefix(addrStr, "0x") {
		addrStr = addrStr[2:]
	}
	addrVal, err := strconv.ParseUint(addrStr, 16, 64)
	if err != nil {
		fmt.Printf("Error parsing address: %v\n", err)
		os.Exit(1)
	}
	addr := process.ProcessMemoryAddress(addrVal)

	// Read memory
	data, err := dump.ReadMemory(addr, process.ProcessMemorySize(*sizeFlag))
	if err != nil {
		fmt.Printf("Error reading memory at 0x%x: %v\n", addr, err)
		os.Exit(1)
	}

	// Hexdump
	fmt.Printf("\nHexdump at 0x%x (%d bytes):\n", addr, *sizeFlag)
	fmt.Println(hexdump.HexdumpBasic(data, uint64(addr), uint(*sizeFlag), dump.MemoryMap))
}
