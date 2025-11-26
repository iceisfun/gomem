package pod

import (
	"fmt"
	"io"
	"strings"
)

// FormatFunc is a callback to format/colorize cell values
type FormatFunc func(value string) string

// ColumnSpec defines a column's properties
type ColumnSpec struct {
	Header     string
	BlankValue string     // Value to show for empty cells (default: "-")
	FormatFunc FormatFunc // Optional formatter/colorizer
	MinWidth   int        // Minimum column width
}

// Table represents a formatted table
type Table struct {
	columns   []ColumnSpec
	rows      [][]string
	widths    []int
	separator string
}

// NewTable creates a new table with the given column specifications
func NewTable(cols ...ColumnSpec) *Table {
	t := &Table{
		columns:   cols,
		rows:      make([][]string, 0),
		widths:    make([]int, len(cols)),
		separator: "-",
	}

	// Initialize widths with header lengths or minimum widths
	for i, col := range cols {
		t.widths[i] = max(col.MinWidth, len(col.Header))
	}

	// Set default blank values
	for i := range t.columns {
		if t.columns[i].BlankValue == "" {
			t.columns[i].BlankValue = "-"
		}
	}

	return t
}

// AddRow adds a row of data to the table
func (t *Table) AddRow(data ...string) {
	// Ensure we have enough columns
	row := make([]string, len(t.columns))
	for i := range row {
		if i < len(data) {
			row[i] = data[i]
		} else {
			// If not enough data provided, use blank value
			row[i] = t.columns[i].BlankValue
		}
	}

	// Process each cell
	for i, val := range row {
		// Handle empty values - apply BlankValue if empty
		if val == "" {
			row[i] = t.columns[i].BlankValue
		}

		// Update width using visible length (accounts for ANSI codes)
		visLen := t.visibleLength(row[i])
		if visLen > t.widths[i] {
			t.widths[i] = visLen
		}
	}

	t.rows = append(t.rows, row)
}

// AddSeparator adds a separator line
func (t *Table) AddSeparator() {
	sep := make([]string, len(t.columns))
	for i := range sep {
		sep[i] = strings.Repeat(t.separator, t.widths[i])
	}
	t.rows = append(t.rows, sep)
}

// SetSeparatorChar sets the character used for separator lines
func (t *Table) SetSeparatorChar(char string) {
	t.separator = char
}

// Render writes the table to the given writer
func (t *Table) Render(w io.Writer) error {
	// Print header
	headers := make([]string, len(t.columns))
	for i, col := range t.columns {
		headers[i] = t.pad(col.Header, t.widths[i])
	}
	if _, err := fmt.Fprintln(w, strings.Join(headers, " ")); err != nil {
		return err
	}

	// Print header separator
	sep := make([]string, len(t.columns))
	for i := range sep {
		sep[i] = strings.Repeat("-", t.widths[i])
	}
	if _, err := fmt.Fprintln(w, strings.Join(sep, " ")); err != nil {
		return err
	}

	// Print rows
	for _, row := range t.rows {
		formatted := make([]string, len(row))
		for i, val := range row {
			// Check if this is a separator row
			if i < len(t.columns) && strings.TrimSpace(strings.Trim(val, t.separator)) == "" && val != "" && val != t.columns[i].BlankValue {
				formatted[i] = val
			} else if i < len(t.columns) {
				// Apply formatting if available
				displayVal := val
				if t.columns[i].FormatFunc != nil {
					displayVal = t.columns[i].FormatFunc(val)
				}
				formatted[i] = t.pad(displayVal, t.widths[i])
			} else {
				// Shouldn't happen, but handle gracefully
				formatted[i] = t.pad(val, t.widths[i])
			}
		}
		if _, err := fmt.Fprintln(w, strings.Join(formatted, " ")); err != nil {
			return err
		}
	}

	return nil
}

// pad pads a string to the given width
func (t *Table) pad(s string, width int) string {
	// Account for ANSI color codes if present
	visibleLen := t.visibleLength(s)
	if visibleLen >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visibleLen)
}

// visibleLength calculates the visible length of a string (excluding ANSI codes)
func (t *Table) visibleLength(s string) int {
	// Simple implementation - doesn't handle ANSI codes
	// For production, you'd want to strip ANSI escape sequences
	length := 0
	inEscape := false
	for _, r := range s {
		if r == '\033' {
			inEscape = true
		} else if inEscape {
			if r == 'm' {
				inEscape = false
			}
		} else {
			length++
		}
	}
	return length
}

// Example color functions for terminal output
func ColorRed(s string) string {
	return fmt.Sprintf("\033[31m%s\033[0m", s)
}

func ColorGreen(s string) string {
	return fmt.Sprintf("\033[32m%s\033[0m", s)
}

func ColorYellow(s string) string {
	return fmt.Sprintf("\033[33m%s\033[0m", s)
}

func ColorBlue(s string) string {
	return fmt.Sprintf("\033[34m%s\033[0m", s)
}

func ColorGray(s string) string {
	return fmt.Sprintf("\033[90m%s\033[0m", s)
}

// Example formatter for pointer validation
func PointerFormatter(s string) string {
	if s == "-" || s == "0x0" {
		return ColorGray(s)
	}
	if strings.Contains(s, "✓") {
		return ColorGreen(s)
	}
	if strings.Contains(s, "×") {
		return ColorRed(s)
	}
	return s
}

// Builder pattern methods for fluent interface
func (t *Table) WithSeparator(char string) *Table {
	t.separator = char
	return t
}

func (t *Table) WithRow(data ...string) *Table {
	t.AddRow(data...)
	return t
}

func (t *Table) WithSeparatorLine() *Table {
	t.AddSeparator()
	return t
}
