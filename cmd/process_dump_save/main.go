package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	pidFlag := flag.Int("pid", 0, "Process ID to attach to")
	outputFlag := flag.String("output", "", "Output directory for the dump")
	allFlag := flag.Bool("all", false, "Save all memory regions (including mmapped files)")
	flag.Parse()

	if *pidFlag == 0 {
		fmt.Println("Error: --pid is required")
		flag.Usage()
		os.Exit(1)
	}

	if *outputFlag == "" {
		fmt.Println("Error: --output is required")
		flag.Usage()
		os.Exit(1)
	}

	// Create output directory
	if err := os.MkdirAll(*outputFlag, 0755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	proc, err := getProcess(*pidFlag)

	if err != nil {
		fmt.Printf("Error attaching to process %d: %v\n", *pidFlag, err)
		os.Exit(1)
	}
	defer proc.Close()

	fmt.Printf("Attached to process %d\n", *pidFlag)

	// In a real implementation, we would pass the 'all' flag to the Save method
	// or filter the regions here before saving.
	// For now, the Save method in process_linux/process_save.go saves everything
	// except non-readable and very large regions.
	// We might need to enhance the Save method to support the 'all' flag.

	// However, the current interface doesn't support passing options to Save.
	// We will use the existing Save method for now.
	// TODO: Enhance Process.Save to accept options or implement custom saving logic here.

	if *allFlag {
		fmt.Println("Note: --all flag is currently not fully implemented in the backend saving logic.")
	}

	fmt.Printf("Saving dump to %s...\n", *outputFlag)
	if err := proc.Save(*outputFlag); err != nil {
		fmt.Printf("Error saving dump: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Dump saved successfully.")
}
