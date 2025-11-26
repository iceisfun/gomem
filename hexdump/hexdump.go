package hexdump

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	"gomem/process/memory_map"

	"gomem/coloransi"
)

// HexDumpOptions defines options for customizing the hexdump output
type HexDumpOptions struct {
	// BytesPerLine defines the number of bytes to display per line
	BytesPerLine int

	// GroupSize defines the grouping of bytes (usually 1, 2, 4, or 8)
	GroupSize int

	// ShowASCII determines whether to show the ASCII representation
	ShowASCII bool

	// ShowOffset determines whether to show the offset/address column
	ShowOffset bool

	// StartOffset is the starting offset for the hexdump
	StartOffset uint64

	// OffsetWidth is the width of the offset column in hex digits
	OffsetWidth int

	// OffsetColor is the color for the offset/address column
	OffsetColor coloransi.ColorCode

	// HexColor is the color for the hex values
	HexColor coloransi.ColorCode

	// ASCIIColor is the color for the ASCII representation
	ASCIIColor coloransi.ColorCode

	// NonPrintableColor is the color for non-printable ASCII characters
	NonPrintableColor coloransi.ColorCode

	// HighlightPattern is a pattern to highlight in the dump
	HighlightPattern []byte

	// HighlightColor is the color for highlighting the pattern
	HighlightColor coloransi.ColorCode

	// HighlightBackgroundColor is the background color for highlighting
	HighlightBackgroundColor coloransi.ColorCode

	// ZeroColor is the color for zero bytes (0x00)
	ZeroColor coloransi.ColorCode

	// MaxLines is the maximum number of lines to show (0 for no limit)
	MaxLines int

	// ShowPointers determines whether to show potential pointers
	ShowPointers bool

	// MemoryMap is the memory map used for pointer validation
	MemoryMap []memory_map.MemoryMapItem
}

// DefaultOptions returns the default hexdump options
func DefaultOptions() HexDumpOptions {
	return HexDumpOptions{
		BytesPerLine:             16,
		GroupSize:                1,
		ShowASCII:                true,
		ShowOffset:               true,
		StartOffset:              0,
		OffsetWidth:              8,
		OffsetColor:              coloransi.Cyan,
		HexColor:                 coloransi.Green,
		ASCIIColor:               coloransi.White,
		NonPrintableColor:        coloransi.BrightBlack,
		HighlightColor:           coloransi.Yellow,
		HighlightBackgroundColor: coloransi.Black,
		ZeroColor:                coloransi.BrightBlack,
		MaxLines:                 0,
		ShowPointers:             false,
		MemoryMap:                nil,
	}
}

// Dump creates a hex dump of the given data with specified options
func Dump(data []byte, options HexDumpOptions) string {
	var buffer bytes.Buffer
	DumpToWriter(&buffer, data, options)
	return buffer.String()
}

// DumpToWriter writes a hex dump of the given data to the specified writer
func DumpToWriter(writer io.Writer, data []byte, options HexDumpOptions) {
	if options.BytesPerLine <= 0 {
		options.BytesPerLine = 16
	}
	if options.GroupSize <= 0 {
		options.GroupSize = 1
	}
	if options.OffsetWidth <= 0 {
		options.OffsetWidth = 8
	}

	lineCount := 0
	for offset := 0; offset < len(data); offset += options.BytesPerLine {
		if options.MaxLines > 0 && lineCount >= options.MaxLines {
			fmt.Fprintf(writer, "... %d more bytes\n", len(data)-offset)
			break
		}

		// Calculate the end of this line
		end := offset + options.BytesPerLine
		if end > len(data) {
			end = len(data)
		}

		lineData := data[offset:end]
		formatLine(writer, lineData, uint64(offset)+options.StartOffset, options)

		lineCount++
	}
}

// formatLine formats a single line of the hex dump
func formatLine(writer io.Writer, data []byte, offset uint64, options HexDumpOptions) {
	// Offset column
	if options.ShowOffset {
		offsetStr := fmt.Sprintf("%0"+strconv.Itoa(options.OffsetWidth)+"x", offset)
		fmt.Fprint(writer, coloransi.Foreground(options.OffsetColor, offsetStr), "  ")
	}

	// Build hex groups
	hexParts := formatHexValues(data, options)

	// Decide if we show a mid-line divider.
	// Only show it once the line actually reaches past half of BytesPerLine.
	useSplit := options.BytesPerLine >= 8 && len(data) > (options.BytesPerLine/2)

	// Compute split index in *groups*, based on configured BytesPerLine and GroupSize.
	groupsPerLine := options.BytesPerLine / options.GroupSize
	if groupsPerLine == 0 {
		groupsPerLine = 1
	}
	leftGroups := groupsPerLine / 2
	if leftGroups > len(hexParts) {
		leftGroups = len(hexParts)
	}

	if useSplit && leftGroups > 0 && leftGroups < len(hexParts) {
		firstHalf := strings.Join(hexParts[:leftGroups], " ")
		secondHalf := strings.Join(hexParts[leftGroups:], " ")
		fmt.Fprint(writer, firstHalf, " | ", secondHalf)
	} else {
		fmt.Fprint(writer, strings.Join(hexParts, " "))
	}

	// ---- Padding to keep ASCII column aligned on short lines ----
	// Compute the difference in printed hex width between a full line and this line.
	if options.BytesPerLine > len(data) {
		fullGroups := (options.BytesPerLine + options.GroupSize - 1) / options.GroupSize // ceil
		curGroups := (len(data) + options.GroupSize - 1) / options.GroupSize             // ceil
		missingBytes := options.BytesPerLine - len(data)

		// Each missing byte removes two hex chars, and each missing group removes one inter-group space.
		deltaSpaces := (fullGroups - 1) - max(0, curGroups-1)

		// Full lines print the inner " | " when BytesPerLine>=8; short lines may or may not.
		pipeFull := 0
		if options.BytesPerLine >= 8 {
			pipeFull = 3
		}
		pipeCur := 0
		if useSplit {
			pipeCur = 3
		}

		paddingSize := missingBytes*2 + deltaSpaces + (pipeFull - pipeCur)
		if paddingSize > 0 {
			fmt.Fprint(writer, strings.Repeat(" ", paddingSize))
		}
	}

	// ASCII
	if options.ShowASCII {
		fmt.Fprint(writer, " | ")

		if options.BytesPerLine >= 8 && len(data) > options.BytesPerLine/2 {
			midPoint := options.BytesPerLine / 2
			if midPoint < len(data) {
				formatASCII(writer, data[:midPoint], 0, options)
				fmt.Fprint(writer, " ")
				formatASCII(writer, data[midPoint:], midPoint, options)
			} else {
				formatASCII(writer, data, 0, options)
			}
		} else {
			formatASCII(writer, data, 0, options)
		}
	}

	// Optional pointer preview (unchanged)
	if options.ShowPointers && len(data) >= 8 {
		fmt.Fprint(writer, " | ")
		ptr := binary.LittleEndian.Uint64(data[:8])
		if isValidPointer(ptr, options.MemoryMap) {
			fmt.Fprintf(writer, "%s ", coloransi.Foreground(coloransi.Yellow, fmt.Sprintf("0x%x", ptr)))
		}
		if len(data) >= 16 {
			ptr2 := binary.LittleEndian.Uint64(data[8:16])
			if isValidPointer(ptr2, options.MemoryMap) {
				fmt.Fprintf(writer, "%s", coloransi.Foreground(coloransi.Yellow, fmt.Sprintf("0x%x", ptr2)))
			}
		}
	}

	fmt.Fprintln(writer)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// formatASCII formats the ASCII part of a hex dump line
func formatASCII(writer io.Writer, data []byte, offset int, options HexDumpOptions) {
	for i, b := range data {
		c := rune(b)
		color := options.ASCIIColor

		// Check if this byte is part of the highlight pattern
		isHighlighted := false
		if len(options.HighlightPattern) > 0 {
			pos := offset + i
			if pos+len(options.HighlightPattern) <= len(data) {
				if bytes.Equal(data[pos:pos+len(options.HighlightPattern)], options.HighlightPattern) {
					isHighlighted = true
				}
			}
		}

		// Choose color based on byte value and highlighting
		if isHighlighted {
			fmt.Fprint(writer, coloransi.Color(options.HighlightColor, options.HighlightBackgroundColor, string(c)))
		} else if b == 0 {
			// Zero byte
			fmt.Fprint(writer, coloransi.Foreground(options.ZeroColor, "."))
		} else if !unicode.IsPrint(c) {
			// Non-printable character
			fmt.Fprint(writer, coloransi.Foreground(options.NonPrintableColor, "."))
		} else {
			// Regular printable character
			fmt.Fprint(writer, coloransi.Foreground(color, string(c)))
		}
	}
}

// formatHexValues formats the hex values part of the line with proper grouping and highlighting
func formatHexValues(data []byte, options HexDumpOptions) []string {
	var result []string
	var groupBuffer []string

	for i, b := range data {
		hexValue := fmt.Sprintf("%02x", b)
		color := options.HexColor

		// Special color for zero bytes
		if b == 0 {
			color = options.ZeroColor
		}

		// Check if this byte is part of the highlight pattern
		isHighlighted := false
		if len(options.HighlightPattern) > 0 {
			if i+len(options.HighlightPattern) <= len(data) {
				if bytes.Equal(data[i:i+len(options.HighlightPattern)], options.HighlightPattern) {
					isHighlighted = true
					color = options.HighlightColor
				}
			}
		}

		// Apply color formatting
		var coloredHex string
		if isHighlighted {
			coloredHex = coloransi.Color(color, options.HighlightBackgroundColor, hexValue)
		} else {
			coloredHex = coloransi.Foreground(color, hexValue)
		}

		groupBuffer = append(groupBuffer, coloredHex)

		// Add the group to the result when it's complete
		if (i+1)%options.GroupSize == 0 || i == len(data)-1 {
			result = append(result, strings.Join(groupBuffer, ""))
			groupBuffer = nil
		}
	}

	return result
}

// isValidPointer checks if a potential pointer is valid by checking the memory map
func isValidPointer(ptr uint64, memoryMap []memory_map.MemoryMapItem) bool {
	if memoryMap == nil || len(memoryMap) == 0 {
		return false
	}

	for _, item := range memoryMap {
		start := uint64(item.Address)
		end := start + uint64(item.Size)
		if ptr >= start && ptr < end {
			return true
		}
	}
	return false
}

// DumpBytes creates a simple hex dump with default options
func DumpBytes(data []byte) string {
	return Dump(data, DefaultOptions())
}

// DumpBytesWithHighlight creates a hex dump with the specified bytes highlighted
func DumpBytesWithHighlight(data []byte, highlight []byte) string {
	options := DefaultOptions()
	options.HighlightPattern = highlight
	return Dump(data, options)
}

// DumpWithOffset creates a hex dump starting at the specified offset
func DumpWithOffset(data []byte, startOffset uint64) string {
	options := DefaultOptions()
	options.StartOffset = startOffset
	return Dump(data, options)
}

// DumpCompact creates a more compact hex dump with smaller group sizes and width
func DumpCompact(data []byte) string {
	options := DefaultOptions()
	options.BytesPerLine = 8
	options.GroupSize = 1
	options.OffsetWidth = 4
	return Dump(data, options)
}

// DumpPretty creates a more visually appealing hex dump with custom colors
func DumpPretty(data []byte) string {
	options := DefaultOptions()
	options.BytesPerLine = 16
	options.GroupSize = 4
	options.OffsetColor = coloransi.ColorTeal
	options.HexColor = coloransi.ColorLimeGreen
	options.ASCIIColor = coloransi.ColorWhite
	options.NonPrintableColor = coloransi.BrightBlack
	options.ZeroColor = coloransi.BrightBlue
	return Dump(data, options)
}

// HexDump is a convenient wrapper around the Dump function with default options
type HexDump struct {
	Options HexDumpOptions
}

// NewHexDump creates a new HexDump with default options
func NewHexDump() *HexDump {
	return &HexDump{
		Options: DefaultOptions(),
	}
}

// SetBytesPerLine sets the number of bytes per line
func (h *HexDump) SetBytesPerLine(value int) *HexDump {
	h.Options.BytesPerLine = value
	return h
}

// SetGroupSize sets the grouping size for bytes
func (h *HexDump) SetGroupSize(value int) *HexDump {
	h.Options.GroupSize = value
	return h
}

// SetShowASCII sets whether to show ASCII representation
func (h *HexDump) SetShowASCII(value bool) *HexDump {
	h.Options.ShowASCII = value
	return h
}

// SetShowOffset sets whether to show offset column
func (h *HexDump) SetShowOffset(value bool) *HexDump {
	h.Options.ShowOffset = value
	return h
}

// SetStartOffset sets the starting offset
func (h *HexDump) SetStartOffset(value uint64) *HexDump {
	h.Options.StartOffset = value
	return h
}

// SetOffsetWidth sets the width of the offset column
func (h *HexDump) SetOffsetWidth(value int) *HexDump {
	h.Options.OffsetWidth = value
	return h
}

// SetColors sets all color options at once
func (h *HexDump) SetColors(offset, hex, ascii, nonPrintable, highlight, zero coloransi.ColorCode) *HexDump {
	h.Options.OffsetColor = offset
	h.Options.HexColor = hex
	h.Options.ASCIIColor = ascii
	h.Options.NonPrintableColor = nonPrintable
	h.Options.HighlightColor = highlight
	h.Options.ZeroColor = zero
	return h
}

// SetHighlight sets the pattern to highlight and its color
func (h *HexDump) SetHighlight(pattern []byte, foreground, background coloransi.ColorCode) *HexDump {
	h.Options.HighlightPattern = pattern
	h.Options.HighlightColor = foreground
	h.Options.HighlightBackgroundColor = background
	return h
}

// SetMaxLines sets the maximum number of lines to display
func (h *HexDump) SetMaxLines(value int) *HexDump {
	h.Options.MaxLines = value
	return h
}

// EnablePointerChecking enables checking for valid pointers
func (h *HexDump) EnablePointerChecking(memoryMap []memory_map.MemoryMapItem) *HexDump {
	h.Options.ShowPointers = true
	h.Options.MemoryMap = memoryMap
	return h
}

// Dump dumps the data with current options
func (h *HexDump) Dump(data []byte) string {
	return Dump(data, h.Options)
}

// DumpToWriter writes the hex dump to the specified writer
func (h *HexDump) DumpToWriter(writer io.Writer, data []byte) {
	DumpToWriter(writer, data, h.Options)
}

// format
//
// addr
//
// 00000000 00 01 02 03 04 05 06 07 | 08 09 0a 0b 0c 0d 0e 0f | aaaaaaaa aaaaaaaa | <pointer 0xaddress if valid> <second pointer if valid>
//
// data of 00's dark grey
// ascii of non-printable characters in red dots
// check if pointers at byte 0 and byte 8 are valid and put the pointer address to the right of the ascii
func HexdumpBasic(data []byte, offset uint64, size uint, mm []memory_map.MemoryMapItem) string {
	options := DefaultOptions()
	options.StartOffset = offset
	options.ShowPointers = true
	options.MemoryMap = mm
	options.BytesPerLine = 16
	options.GroupSize = 1
	options.ZeroColor = coloransi.BrightBlack
	options.NonPrintableColor = coloransi.Red

	return Dump(data, options)
}
