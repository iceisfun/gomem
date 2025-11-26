package pod

import (
	"fmt"
	"gomem/coloransi"
	"gomem/process"
	"io"
	"os"
	"reflect"
	"strings"
)

// add near the top of the file
type stringer interface{ String() string }

func expandFlagsRows(table *Table, fieldName string, fv reflect.Value) {
	switch fv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val := uint64(fv.Int())
		emitFlags(table, fieldName, val, fv.Type().Bits())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		val := fv.Uint()
		emitFlags(table, fieldName, val, fv.Type().Bits())
	}
}

func emitFlags(table *Table, fieldName string, val uint64, bitSize int) {
	if bitSize <= 0 || bitSize > 64 {
		bitSize = 64
	}
	// mask to the field width in case of signed values
	varWidthMask := uint64(^uint64(0))
	if bitSize < 64 {
		varWidthMask = (uint64(1) << bitSize) - 1
	}
	val &= varWidthMask

	if val == 0 {
		return
	}

	// hex width in nibbles (2/4/8/16)
	nibbles := (bitSize + 3) / 4

	for b := 0; b < bitSize; b++ {
		if (val>>b)&1 == 1 {
			mask := uint64(1) << b
			offsetHex := fmt.Sprintf("0x%0*X", nibbles, mask)
			// Field/AsPtr blank; Value shows which bit is set
			table.AddRow("", offsetHex, fmt.Sprintf("bit %d True", b), "", "-")
			// If you prefer a labeled field and +bit in offset, use:
			// table.AddRow("  "+fieldName+".bit", fmt.Sprintf("+%d", b), fmt.Sprintf("True (mask %s)", offsetHex), "", "-")
		}
	}
}

func tryStringer(v reflect.Value) (string, bool) {
	// Work with concrete values; handle both value and pointer receivers.
	if !v.IsValid() {
		return "", false
	}

	// Prefer value receiver (e.g., type Thing uint32; func (t Thing) String() string)
	if v.CanInterface() {
		if s, ok := v.Interface().(fmt.Stringer); ok {
			return s.String(), true
		}
	}

	// Fall back to pointer receiver (e.g., func (*Thing) String() string)
	if v.CanAddr() {
		av := v.Addr()
		if av.CanInterface() {
			if s, ok := av.Interface().(fmt.Stringer); ok {
				return s.String(), true
			}
		}
	}
	return "", false
}

func formatScalarWithStringer(fv reflect.Value, hexIfUint bool) string {
	// Raw formatting
	var raw string
	switch fv.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		u := fv.Uint()
		if hexIfUint {
			if u != 0 {
				raw = fmt.Sprintf("%d (0x%X) %v", u, u, fv) // preserves your existing rich form
			} else {
				raw = fmt.Sprintf("%d (0x%X)", u, u)
			}
		} else {
			raw = fmt.Sprintf("%d", u)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i := fv.Int()
		if i != 0 {
			raw = fmt.Sprintf("%d (0x%X) %v", i, i, fv)
		} else {
			raw = fmt.Sprintf("%d (0x%X)", i, i)
		}
	case reflect.Bool:
		raw = fmt.Sprintf("%v", fv.Bool())
	default:
		raw = fmt.Sprintf("%v", fv.Interface())
	}

	// Optional String() annotation
	if s, ok := tryStringer(fv); ok && s != "" {
		return raw + " :: " + s
	}
	return raw
}

func asPtrString(isValidPtr func(uint64) bool, fv reflect.Value) string {
	switch fv.Kind() {
	case reflect.Uint, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		addr := fv.Uint()
		if addr == 0 {
			return ""
		}
		if isValidPtr(uint64(addr)) {
			return fmt.Sprintf("0x%X ✓", addr)
		}
		return fmt.Sprintf("0x%X ×", addr)
	case reflect.Pointer:
		if fv.IsNil() {
			return ""
		}
		addr := uint64(fv.Pointer())
		if isValidPtr(addr) {
			return fmt.Sprintf("0x%X ✓", addr)
		}
		return fmt.Sprintf("0x%X ×", addr)
	}
	return ""
}

func PrintPodStruct[T any](proc process.Process, v T, w io.Writer) {

	isValidPtr := func(addr uint64) bool {
		if proc == nil || addr < 0x100000 || addr > 0xff00000000000000 {
			return false
		}
		return proc.IsValidAddress(process.ProcessMemoryAddress(addr))
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			fmt.Fprintln(w, "<nil pointer>")
			return
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		fmt.Fprintf(w, "PrintPodStruct: expected struct or *struct, got %s\n", rv.Kind())
		return
	}

	rt := rv.Type()

	// Header
	fmt.Fprintf(w, "=== %s ===\n", rt.Name())
	fmt.Fprintf(w, "Size: 0x%X (%d bytes)\n\n", rt.Size(), rt.Size())

	// Create table with column specs
	table := NewTable(
		ColumnSpec{Header: "Field", MinWidth: 8},
		ColumnSpec{Header: "Offset", MinWidth: 10},
		ColumnSpec{
			Header:   "Value",
			MinWidth: 6,
			FormatFunc: func(s string) string {
				if s == "0 (0x0)" {
					return coloransi.Foreground(coloransi.CreateRGB(64, 64, 64), s)
				}
				return coloransi.Foreground(coloransi.ColorLimeGreen, s)
			},
		},
		ColumnSpec{
			Header:     "AsPtr",
			MinWidth:   6,
			BlankValue: "-",
			FormatFunc: func(s string) string {
				// Optionally colorize pointer validity
				if s == "-" || s == "0x0" {
					return coloransi.Foreground(coloransi.White, s)
				}
				if strings.Contains(s, "✓") {
					return coloransi.Foreground(coloransi.ColorLimeGreen, s)
				}
				if strings.Contains(s, "×") {
					return coloransi.Foreground(coloransi.BrightRed, s)
				}
				return s
			},
		},
		ColumnSpec{Header: "Tags", MinWidth: 6, BlankValue: "-"},
	)

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		fv := rv.Field(i)
		offset := field.Offset

		// Format primary value
		var valueStr string
		switch fv.Kind() {
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			if tag := field.Tag.Get("pod"); strings.Contains(tag, "pointer") {
				valueStr = fmt.Sprintf("0x%016X", fv.Uint())
			} else {
				if fv.Uint() != 0 {
					valueStr = fmt.Sprintf("%d (0x%X) %v", fv.Uint(), fv.Uint(), fv)
				} else {
					valueStr = fmt.Sprintf("%d (0x%X)", fv.Uint(), fv.Uint())
				}
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if fv.Int() != 0 {
				valueStr = fmt.Sprintf("%d (0x%X) %v", fv.Int(), fv.Int(), fv)
			} else {
				valueStr = fmt.Sprintf("%d (0x%X)", fv.Int(), fv.Int())
			}
		case reflect.Array:
			elemT := fv.Type().Elem()
			elemSize := elemT.Size()
			_ = elemSize

			// Special-case: [N]byte with pod:"char_array"
			if elemT.Kind() == reflect.Uint8 && strings.Contains(field.Tag.Get("pod"), "char_array") {
				b := make([]byte, fv.Len())
				for j := 0; j < fv.Len(); j++ {
					b[j] = byte(fv.Index(j).Uint())
				}
				// C-string up to first NUL
				n := len(b)
				for j, x := range b {
					if x == 0 {
						n = j
						break
					}
				}
				if n > 0 {
					valueStr = fmt.Sprintf("%q", string(b[:n]))
				} else {
					valueStr = fmt.Sprintf("[%d]byte{...}", fv.Len())
				}

				// Parent summary row
				table.AddRow(field.Name, fmt.Sprintf("0x%04X", offset), valueStr, "", field.Tag.Get("pod"))

				// Expanded element rows (bytes)
				for j := 0; j < fv.Len(); j++ {
					elem := fv.Index(j)
					elemVal := fmt.Sprintf("0x%02X", elem.Uint())
					elemPtr := asPtrString(isValidPtr, elem) // mostly empty for bytes
					table.AddRow(
						fmt.Sprintf("  %s[%d]", field.Name, j),
						fmt.Sprintf("+%d", j),
						elemVal,
						elemPtr,
						"-",
					)
				}
				continue
			}

			// Non-byte arrays: show a parent summary then each element on its own row.
			// Parent summary
			{
				// Try to detect "all zero" quickly
				allZero := true
				for j := 0; j < fv.Len(); j++ {
					if !fv.Index(j).IsZero() {
						allZero = false
						break
					}
				}
				if allZero {
					valueStr = fmt.Sprintf("[%d]%s{0...}", fv.Len(), elemT)
				} else {
					// brief preview of first 3
					maxShow := fv.Len()
					if maxShow > 3 {
						maxShow = 3
					}
					sb := &strings.Builder{}
					fmt.Fprintf(sb, "[%d]%s{", fv.Len(), elemT)
					for j := 0; j < maxShow; j++ {
						if j > 0 {
							sb.WriteString(",")
						}
						ev := fv.Index(j)
						switch ev.Kind() {
						case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
							fmt.Fprintf(sb, "0x%X", ev.Uint())
						case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
							fmt.Fprintf(sb, "%d", ev.Int())
						default:
							fmt.Fprintf(sb, "%v", ev.Interface())
						}
					}
					if fv.Len() > maxShow {
						sb.WriteString("...")
					}
					sb.WriteString("}")
					valueStr = sb.String()
				}
			}
			table.AddRow(field.Name, fmt.Sprintf("0x%04X", offset), valueStr, "", field.Tag.Get("pod"))

			// Element rows
			for j := 0; j < fv.Len(); j++ {
				elem := fv.Index(j)

				// Value string w/ String() awareness
				elemVal := formatScalarWithStringer(elem, true /*hexIfUint*/)

				// If element is a struct, don't attempt to fully render; show brief form
				if elem.Kind() == reflect.Struct {
					elemVal = fmt.Sprintf("{%s}", elem.Type().Name())
				}

				// AsPtr check per element (works for integral/pointer elements)
				elemPtr := asPtrString(isValidPtr, elem)

				// Offset column shows +idx (not bytes) as requested
				table.AddRow(
					fmt.Sprintf("  %s[%d]", field.Name, j),
					fmt.Sprintf("+%d", j),
					elemVal,
					elemPtr,
					"-",
				)
			}
			continue
		case reflect.Bool:
			valueStr = fmt.Sprintf("%v", fv.Bool())
		case reflect.Pointer:
			if fv.IsNil() {
				valueStr = "nil"
			} else {
				valueStr = fmt.Sprintf("0x%016X", fv.Pointer())
			}
		default:
			valueStr = fmt.Sprintf("%v", fv.Interface())
		}

		// Format offset
		offsetStr := fmt.Sprintf("0x%04X", offset)

		// Determine AsPtr column value
		asPtr := ""
		switch fv.Kind() {
		case reflect.Uint, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			addr := fv.Uint()
			if addr != 0 {
				if isValidPtr(uint64(addr)) {
					asPtr = fmt.Sprintf("0x%X ✓", addr)
				} else {
					asPtr = fmt.Sprintf("0x%X ×", addr)
				}
			}
		case reflect.Pointer:
			if !fv.IsNil() {
				addr := uint64(fv.Pointer())
				if isValidPtr(addr) {
					asPtr = fmt.Sprintf("0x%X ✓", addr)
				} else {
					asPtr = fmt.Sprintf("0x%X ×", addr)
				}
			}
		}

		// Get tags
		tag := field.Tag.Get("pod")

		// Add row to table
		table.AddRow(field.Name, offsetStr, valueStr, asPtr, tag)

		// flags expansion
		if strings.Contains(strings.ToLower(field.Name), "flags") {
			expandFlagsRows(table, field.Name, fv)
		}
	}

	// Render the table
	table.Render(w)
	fmt.Fprintln(w)
}

func PrintPodStructCompact[T any](v T, w io.Writer) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			fmt.Fprintln(w, "<nil pointer>")
			return
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		fmt.Fprintf(w, "PrintPodStructCompact: expected struct or *struct, got %s\n", rv.Kind())
		return
	}
	rt := rv.Type()

	fmt.Fprintf(w, "%s {", rt.Name())
	first := true
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		if !first {
			fmt.Fprint(w, ", ")
		}
		first = false

		fv := rv.Field(i)
		tag := f.Tag.Get("pod")
		if strings.Contains(tag, "pointer") && (fv.Kind() == reflect.Uint || fv.Kind() == reflect.Uint64 || fv.Kind() == reflect.Uintptr) {
			fmt.Fprintf(w, "%s:0x%X", f.Name, fv.Uint())
		} else {
			fmt.Fprintf(w, "%s:%v", f.Name, fv.Interface())
		}
	}
	fmt.Fprintln(w, "}")
}

func PrintPodStructStdout[T any](proc process.Process, v T) {
	PrintPodStruct(proc, v, os.Stdout)
}

// PrintPodStructWithColors creates a colored version for terminal output
func PrintPodStructWithColors[T any](proc process.Process, v T, w io.Writer) {

	isValidPtr := func(addr uint64) bool {
		if proc == nil || addr < 0x100000 || addr > 0xff00000000000000 {
			return false
		}
		return proc.IsValidAddress(process.ProcessMemoryAddress(addr))
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			fmt.Fprintln(w, "<nil pointer>")
			return
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		fmt.Fprintf(w, "PrintPodStruct: expected struct or *struct, got %s\n", rv.Kind())
		return
	}

	rt := rv.Type()

	// Header with color
	fmt.Fprintf(w, "\033[1m=== %s ===\033[0m\n", rt.Name())
	fmt.Fprintf(w, "Size: \033[36m0x%X\033[0m (%d bytes)\n\n", rt.Size(), rt.Size())

	// Create table with colored column specs
	table := NewTable(
		ColumnSpec{Header: "Field", MinWidth: 20},
		ColumnSpec{Header: "Offset", MinWidth: 10},
		ColumnSpec{Header: "Value", MinWidth: 20},
		ColumnSpec{
			Header:     "AsPtr",
			MinWidth:   20,
			BlankValue: "", // Empty for AsPtr
			FormatFunc: func(s string) string {
				if s == "" {
					return s // Keep empty
				}
				if strings.Contains(s, "✓") {
					return ColorGreen(s)
				}
				if strings.Contains(s, "×") {
					return ColorRed(s)
				}
				return s
			},
		},
		ColumnSpec{
			Header:     "Tags",
			MinWidth:   15,
			BlankValue: "-",
			FormatFunc: func(s string) string {
				if strings.Contains(s, "valid_pointer") {
					return ColorBlue(s)
				}
				if strings.Contains(s, "follow") {
					return ColorYellow(s)
				}
				if s == "-" || s == "" {
					return ColorGray(s)
				}
				return s
			},
		},
	)

	// Process fields (same as before but with table)
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}

		fv := rv.Field(i)
		offset := field.Offset

		// Format value (same logic as before)
		var valueStr string
		// ... (same switch statement as in PrintPodStruct)
		switch fv.Kind() {
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			if tag := field.Tag.Get("pod"); strings.Contains(tag, "pointer") {
				valueStr = fmt.Sprintf("0x%016X", fv.Uint())
			} else {
				valueStr = fmt.Sprintf("%d (0x%X)", fv.Uint(), fv.Uint())
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			valueStr = fmt.Sprintf("%d (0x%X)", fv.Int(), fv.Int())
		case reflect.Bool:
			valueStr = fmt.Sprintf("%v", fv.Bool())
		case reflect.Pointer:
			if fv.IsNil() {
				valueStr = "nil"
			} else {
				valueStr = fmt.Sprintf("0x%016X", fv.Pointer())
			}
		default:
			valueStr = fmt.Sprintf("%v", fv.Interface())
		}

		offsetStr := fmt.Sprintf("0x%04X", offset)

		// Determine AsPtr
		asPtr := ""
		switch fv.Kind() {
		case reflect.Uint, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			addr := fv.Uint()
			if addr != 0 {
				if isValidPtr(uint64(addr)) {
					asPtr = fmt.Sprintf("0x%X ✓", addr)
				} else {
					asPtr = fmt.Sprintf("0x%X ×", addr)
				}
			}
		case reflect.Pointer:
			if !fv.IsNil() {
				addr := uint64(fv.Pointer())
				if isValidPtr(addr) {
					asPtr = fmt.Sprintf("0x%X ✓", addr)
				} else {
					asPtr = fmt.Sprintf("0x%X ×", addr)
				}
			}
		}

		tag := field.Tag.Get("pod")
		table.AddRow(field.Name, offsetStr, valueStr, asPtr, tag)
	}

	table.Render(w)
	fmt.Fprintln(w)
}
