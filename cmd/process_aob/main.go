package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gomem/hexdump"
	"gomem/process"
)

// AOBPart represents a part of the AOB pattern
type AOBPart struct {
	Value byte
	Mask  byte // 0xFF for exact match, 0x00 for wildcard
}

func main() {
	pidFlag := flag.Int("pid", 0, "Process ID to attach to")
	aobFlag := flag.String("aob", "", "Array of bytes to scan for (e.g., '00,ba,ad,??,f0')")
	flag.Parse()

	if *pidFlag == 0 {
		fmt.Println("Error: --pid is required")
		flag.Usage()
		os.Exit(1)
	}

	if *aobFlag == "" {
		fmt.Println("Error: --aob is required")
		flag.Usage()
		os.Exit(1)
	}

	// Parse AOB string
	pattern, err := parseAOB(*aobFlag)
	if err != nil {
		fmt.Printf("Error parsing AOB: %v\n", err)
		os.Exit(1)
	}

	proc, err := getProcess(*pidFlag)

	if err != nil {
		fmt.Printf("Error attaching to process %d: %v\n", *pidFlag, err)
		os.Exit(1)
	}
	defer proc.Close()

	fmt.Printf("Attached to process %d\n", *pidFlag)
	fmt.Printf("Scanning for pattern: %s\n", formatPattern(pattern))

	// Scan memory
	// Since the Process interface doesn't have a generic Scan method yet,
	// we'll implement a basic scanner here using ReadMemoryMap and ReadMemory.
	// In a real implementation, this should be part of the Process interface.

	// Update memory map
	if err := proc.UpdateMemoryMap(); err != nil {
		fmt.Printf("Error updating memory map: %v\n", err)
		os.Exit(1)
	}

	matches, err := scanMemory(proc, pattern)
	if err != nil {
		fmt.Printf("Error scanning memory: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Found %d matches:\n", len(matches))

	for _, match := range matches {
		fmt.Printf("Match at 0x%x:\n", match)

		// Read context (16 bytes before and 32 bytes after)
		start := match - 16
		size := process.ProcessMemorySize(48) // 16 + len(pattern) + padding
		if len(pattern) > 16 {
			size = process.ProcessMemorySize(32 + len(pattern))
		}

		data, err := proc.ReadMemory(start, size)
		if err == nil {
			// Highlight the match
			hlPattern := make([]byte, len(pattern))
			for i, p := range pattern {
				hlPattern[i] = p.Value
			}

			// Use hexdump with highlighting
			// Note: Highlighting with wildcards is tricky with simple byte matching
			// For now, we just dump the memory
			fmt.Println(hexdump.HexdumpBasic(data, uint64(start), uint(size), nil))
		}
	}
}

func parseAOB(aob string) ([]AOBPart, error) {
	// Split by comma or space
	parts := strings.FieldsFunc(aob, func(r rune) bool {
		return r == ',' || r == ' '
	})

	var pattern []AOBPart

	for _, part := range parts {
		if part == "??" || part == "?" {
			pattern = append(pattern, AOBPart{Value: 0, Mask: 0})
			continue
		}

		// Handle type:value expansion (e.g., uint32:1234)
		if strings.Contains(part, ":") {
			// TODO: Implement type expansion
			return nil, fmt.Errorf("type expansion not yet implemented: %s", part)
		}

		// Parse hex
		val, err := strconv.ParseUint(part, 16, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid hex byte: %s", part)
		}
		pattern = append(pattern, AOBPart{Value: byte(val), Mask: 0xFF})
	}

	return pattern, nil
}

func formatPattern(pattern []AOBPart) string {
	var sb strings.Builder
	for i, p := range pattern {
		if i > 0 {
			sb.WriteString(" ")
		}
		if p.Mask == 0 {
			sb.WriteString("??")
		} else {
			sb.WriteString(hex.EncodeToString([]byte{p.Value}))
		}
	}
	return sb.String()
}

func scanMemory(proc process.Process, pattern []AOBPart) ([]process.ProcessMemoryAddress, error) {
	// Create AOB object
	aobObj, err := process.NewAOB(
		func() []byte {
			p := make([]byte, len(pattern))
			for i, part := range pattern {
				p[i] = part.Value
			}
			return p
		}(),
		func() []byte {
			m := make([]byte, len(pattern))
			for i, part := range pattern {
				m[i] = part.Mask
			}
			return m
		}(),
	)
	if err != nil {
		return nil, fmt.Errorf("Error creating AOB: %v", err)
	}

	matches, err := proc.Scan(aobObj)
	if err != nil {
		return nil, fmt.Errorf("Scan error: %v", err)
	}

	return matches, nil
}
