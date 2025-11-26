package main

import (
	"flag"
	"fmt"
	"os"

	"gomem/pod"
	"gomem/process"
	"gomem/process_blob"
)

// FlagData matches the C++ struct
type FlagData struct {
	ID    int32    `pod:"type=int32"`
	Name  [32]byte `pod:"char_array"`
	Value float32  `pod:"type=float32"`
}

// InnerData matches the C++ struct
type InnerData[T any] struct {
	SomeInteger int32    `pod:"type=int32"`
	_           uint32   // Padding for alignment (4 bytes)
	FlagPtr     *T       `pod:"valid_pointer"`
	Description [64]byte `pod:"char_array"`
}

// GameState matches the C++ struct
type GameState[T any] struct {
	Seed         [4]byte      `pod:"char_array"`
	_            uint32       // Padding for alignment (4 bytes)
	UniqueID     uint64       `pod:"type=uint64"`
	Inner        InnerData[T] // Embedded by value to match C++ layout
	OtherFlagPtr *T           `pod:"valid_pointer"`
}

func main() {
	fromFlag := flag.String("from", "", "Directory containing the dump")
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

	// Scan for "SEED" pattern (53 45 45 44)
	pattern := []byte{0x53, 0x45, 0x45, 0x44}
	mask := []byte{0xFF, 0xFF, 0xFF, 0xFF}
	aob, _ := process.NewAOB(pattern, mask)

	matches, err := dump.Scan(aob)
	if err != nil {
		fmt.Printf("Error scanning for SEED: %v\n", err)
		os.Exit(1)
	}

	if len(matches) == 0 {
		fmt.Println("SEED pattern not found")
		os.Exit(1)
	}

	fmt.Printf("Found %d matches for SEED\n", len(matches))

	for _, match := range matches {
		fmt.Printf("Reading GameState at 0x%x\n", match)

		// Read GameState
		state, err := pod.ReadT[GameState[FlagData]](dump, match)
		if err != nil {
			fmt.Printf("Error reading GameState: %v\n", err)
			continue
		}

		// Print the struct
		pod.PrintPodStruct(dump, state, os.Stdout)

		if state.Inner.FlagPtr != nil {
			fmt.Printf("\nInner.FlagPtr:\n")
			pod.PrintPodStruct(dump, *state.Inner.FlagPtr, os.Stdout)
		}

		if state.OtherFlagPtr != nil {
			fmt.Printf("\nOtherFlagPtr:\n")
			pod.PrintPodStruct(dump, *state.OtherFlagPtr, os.Stdout)
		}
	}
}
