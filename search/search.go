package search

import (
	"fmt"
	"gomem/process"
	"unsafe"
)

// Searcher holds configuration for the search
type Searcher struct {
	MaxStructSize uint
	MaxDepth      int
	MinAlignment  uint
	SearchFor     func([]byte) bool
}

// Option is a function that configures a Searcher
type Option func(*Searcher)

func WithMaxStructSize(size uint) Option {
	return func(s *Searcher) {
		s.MaxStructSize = size
	}
}

func WithMaxDepth(depth int) Option {
	return func(s *Searcher) {
		s.MaxDepth = depth
	}
}

func WithMinAlignment(align uint) Option {
	return func(s *Searcher) {
		s.MinAlignment = align
	}
}

func WithSearchForType[T any](val T) Option {
	return func(s *Searcher) {
		s.SearchFor = func(data []byte) bool {
			if len(data) < int(unsafe.Sizeof(val)) {
				return false
			}
			// Compare bytes
			// This assumes POD and little endian
			valBytes := unsafe.Slice((*byte)(unsafe.Pointer(&val)), int(unsafe.Sizeof(val)))
			for i := 0; i < len(valBytes); i++ {
				if data[i] != valBytes[i] {
					return false
				}
			}
			return true
		}
	}
}

// SearchResult represents a found path to the target
type SearchResult struct {
	Path  []process.ProcessMemorySize // Offsets from base
	Value interface{}
}

// Search performs a recursive search for the target value
func Search(proc process.Process, base process.ProcessMemoryAddress, options ...Option) ([]SearchResult, error) {
	s := &Searcher{
		MaxStructSize: 256, // Default
		MaxDepth:      3,   // Default
		MinAlignment:  4,   // Default
	}

	for _, opt := range options {
		opt(s)
	}

	if s.SearchFor == nil {
		return nil, fmt.Errorf("no search target specified")
	}

	var results []SearchResult
	visited := make(map[process.ProcessMemoryAddress]bool)

	var searchRecursive func(addr process.ProcessMemoryAddress, depth int, path []process.ProcessMemorySize)
	searchRecursive = func(addr process.ProcessMemoryAddress, depth int, path []process.ProcessMemorySize) {
		if depth > s.MaxDepth {
			return
		}
		if visited[addr] {
			return
		}
		visited[addr] = true

		// Read the struct memory
		// We read MaxStructSize bytes
		data, err := proc.ReadMemory(addr, process.ProcessMemorySize(s.MaxStructSize))
		if err != nil {
			// If we can't read the full size, maybe try reading smaller chunks?
			// For now, just return/skip
			return
		}

		// Iterate over the memory with alignment
		for offset := uint(0); offset < s.MaxStructSize; offset += s.MinAlignment {
			if offset+s.MinAlignment > uint(len(data)) {
				break
			}

			// Check if this offset matches the target
			// We pass the slice starting at offset
			if s.SearchFor(data[offset:]) {
				// Found a match!
				// Copy path and append offset
				newPath := make([]process.ProcessMemorySize, len(path))
				copy(newPath, path)
				newPath = append(newPath, process.ProcessMemorySize(offset))

				results = append(results, SearchResult{
					Path: newPath,
				})
			}

			// Check if this offset is a pointer (only if 8-byte aligned)
			if offset%8 == 0 && depth < s.MaxDepth {
				// Read uint64 at this offset
				if offset+8 <= uint(len(data)) {
					ptrVal := *(*uint64)(unsafe.Pointer(&data[offset]))

					// Check if pointer is valid
					if ptrVal != 0 && proc.IsValidAddress(process.ProcessMemoryAddress(ptrVal)) {
						// Recurse
						newPath := make([]process.ProcessMemorySize, len(path))
						copy(newPath, path)
						newPath = append(newPath, process.ProcessMemorySize(offset))

						searchRecursive(process.ProcessMemoryAddress(ptrVal), depth+1, newPath)
					}
				}
			}
		}
	}

	searchRecursive(base, 0, []process.ProcessMemorySize{})

	return results, nil
}
