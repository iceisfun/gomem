package pod

import (
	"errors"
	"fmt"
	"gomem/process"
	"reflect"
	"strings"
	"unsafe"
)

func SizeOf[T any]() process.ProcessMemorySize {
	var t T
	return process.ProcessMemorySize(unsafe.Sizeof(t))
}

func ReadT[T any](proc process.Process, addr process.ProcessMemoryAddress) (T, error) {
	size := SizeOf[T]()
	if size == 0 {
		return *new(T), errors.New("ReadT: size of T is zero")
	}

	blob, blob_err := proc.ReadBlob(process.ProcessMemoryAddress(addr), size)
	if blob_err != nil {
		return *new(T), blob_err
	}

	return ReadBlob[T](proc, blob)
}

// WriteT serializes a POD struct T into a raw byte slice using the in-memory layout.
// T must be POD (no pointers or Go-managed references) for the bytes to be meaningful
// outside the process. This function uses unsafe to copy the raw bytes directly.
func WriteT[T any](v T) []byte {
	// Take the address of v and make a byte slice view of its memory.
	size := int(unsafe.Sizeof(v))
	if size == 0 {
		return []byte{}
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(&v)), size)
	out := make([]byte, size)
	copy(out, src)
	return out
}

func ReadSliceT[T any](proc process.Process, addr process.ProcessMemoryAddress, count int) ([]T, error) {
	if count < 0 {
		return nil, errors.New("ReadSliceT: count must be positive")
	}

	size := SizeOf[T]()
	if size == 0 {
		return []T{}, nil
	}

	// Calculate total size needed for all elements
	totalSize := size * process.ProcessMemorySize(count)

	// Read the entire blob at once for efficiency
	blob, blob_err := proc.ReadBlob(addr, totalSize)
	if blob_err != nil {
		return nil, blob_err
	}

	// Create result slice
	result := make([]T, count)

	// Parse each element from the blob
	elementSize := int(size)
	for i := range count {
		offset := i * elementSize
		if offset+elementSize > len(blob.Data()) {
			return nil, errors.New("ReadSliceT: unexpected end of data")
		}

		// Create a sub-blob for this element
		elementBlob, _ := blob.OffsetBlob(process.ProcessMemoryAddress(offset), size)

		// Parse the element
		element, err := ReadBlob[T](proc, elementBlob)
		if err != nil {
			return nil, fmt.Errorf("ReadSliceT: failed to parse element %d: %w", i, err)
		}

		result[i] = element
	}

	return result, nil
}

func ReadPointerList(proc process.Process, addr uint64, count int) (results []process.ProcessMemoryAddress, err error) {
	blob, blob_err := proc.ReadBlob(process.ProcessMemoryAddress(addr), process.ProcessMemorySize(count*8))
	if blob_err != nil {
		return nil, fmt.Errorf("ReadPointerList: failed to read blob at 0x%x: %w", addr, blob_err)
	}
	for i := range count {
		offset := i * 8
		ptr := blob.OffsetPOINTER2(process.ProcessMemoryAddress(offset))
		if proc.IsValidAddress(ptr) {
			results = append(results, ptr)
		}
	}

	return results, nil
}

// ReadBlob copies the first sizeof(T) bytes from data into a new T.
// T must be "POD": it and all of its fields/element types contain no pointers.
func ReadBlob[T any](proc process.Process, offset process.ProcessReadOffset) (T, error) {
	data := offset.Data()
	var zero T

	// Optional runtime guard: reject types that contain pointers.
	if hasPointers[T]() {
		return zero, errors.New("BytesInto: T contains pointers; not POD-safe")
	}

	// Size check
	var tmp T
	size := int(unsafe.Sizeof(tmp))
	if len(data) < size {
		return zero, errors.New("BytesInto: buffer too small")
	}

	// Copy into the stack-allocated tmp
	dst := unsafe.Slice((*byte)(unsafe.Pointer(&tmp)), size)
	copy(dst, data[:size])

	// Validate and clean pointer fields based on struct tags
	if err := validateAndCleanPointers(&tmp, proc); err != nil {
	}

	return tmp, nil
}

// hasPointers reports whether T (recursively) contains any pointer-like fields.
func hasPointers[T any]() bool {
	var t T
	return typeHasPointers(reflect.TypeOf(t))
}

func typeHasPointers(rt reflect.Type) bool {
	switch rt.Kind() {
	case reflect.Ptr, reflect.UnsafePointer, reflect.Interface, reflect.Func, reflect.Map, reflect.Slice, reflect.String:
		return true
	case reflect.Array:
		return typeHasPointers(rt.Elem())
	case reflect.Struct:
		for i := 0; i < rt.NumField(); i++ {
			if typeHasPointers(rt.Field(i).Type) {
				return true
			}
		}
		return false
	default:
		// bool, ints, uints, floats, complex, etc.
		return false
	}
}

// validateAndCleanPointers validates pointers and cleans invalid ones
func validateAndCleanPointers(structPtr interface{}, proc process.Process) error {
	v := reflect.ValueOf(structPtr).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		tag := fieldType.Tag.Get("pod")

		if err := processField(field, fieldType, tag, proc, false); err != nil {
			// In non-strict mode, we clean the field instead of returning error
			cleanInvalidField(field, tag)
		}
	}

	return nil
}

// validatePointersStrict validates pointers and returns error on invalid ones
func validatePointersStrict(structPtr interface{}, proc process.Process) error {
	v := reflect.ValueOf(structPtr).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		tag := fieldType.Tag.Get("pod")

		if err := processField(field, fieldType, tag, proc, true); err != nil {
			return err
		}
	}

	return nil
}

// processField handles validation for a single field
func processField(field reflect.Value, fieldType reflect.StructField, tag string, proc process.Process, strict bool) error {
	if tag == "" {
		return nil
	}

	tags := parsePodTags(tag)

	// Handle valid_pointer tag
	if tags["type"] == "valid_pointer" {
		return validatePointerField(field, fieldType, tags, proc, strict)
	}

	// Handle other tag types
	switch tags["type"] {
	case "char_array":
		cleanCharArray(field)
	case "skip":
		// Do nothing
	}

	return nil
}

// validatePointerField validates a pointer field
func validatePointerField(field reflect.Value, fieldType reflect.StructField, tags map[string]string, proc process.Process, strict bool) error {
	// Only handle uint64 pointer fields
	if field.Kind() != reflect.Uint64 {
		return nil
	}

	ptr := field.Uint()

	// Check if it's a required pointer
	if tags["required"] == "true" && ptr == 0 {
		if strict {
			return errors.New("required pointer field " + fieldType.Name + " is NULL")
		}
		// In non-strict mode, we'll leave it as 0
		return nil
	}

	// NULL is valid unless required
	if ptr == 0 {
		return nil
	}

	// Validate the address using the process's IsValidAddress
	addr := process.ProcessMemoryAddress(ptr)
	if !proc.IsValidAddress(addr) {
		if strict {
			return errors.New("invalid pointer in field " + fieldType.Name + ": 0x" + string(ptr))
		}
		// In non-strict mode, clean the invalid pointer
		if field.CanSet() {
			field.SetUint(0)
		}
	}

	return nil
}

// parsePodTags parses pod tag string into a map
func parsePodTags(tagStr string) map[string]string {
	tags := make(map[string]string)
	if tagStr == "" {
		return tags
	}

	parts := strings.Split(tagStr, ",")
	if len(parts) > 0 {
		tags["type"] = parts[0]
	}

	// Parse additional options like "required"
	for i := 1; i < len(parts); i++ {
		if parts[i] == "required" {
			tags["required"] = "true"
		} else if strings.Contains(parts[i], "=") {
			kv := strings.SplitN(parts[i], "=", 2)
			if len(kv) == 2 {
				tags[kv[0]] = kv[1]
			}
		}
	}

	return tags
}

// cleanInvalidField sets invalid fields to safe values
func cleanInvalidField(field reflect.Value, tag string) {
	tags := parsePodTags(tag)

	switch tags["type"] {
	case "valid_pointer":
		if field.Kind() == reflect.Uint64 && field.CanSet() {
			field.SetUint(0) // Set invalid pointer to NULL
		}
	}
}

// cleanCharArray ensures proper null termination
func cleanCharArray(field reflect.Value) {
	if field.Kind() != reflect.Array || field.Type().Elem().Kind() != reflect.Uint8 {
		return
	}

	foundNull := false
	for i := 0; i < field.Len(); i++ {
		if foundNull {
			// Zero out everything after first null
			if field.Index(i).CanSet() {
				field.Index(i).SetUint(0)
			}
		} else if field.Index(i).Uint() == 0 {
			foundNull = true
		}
	}
}
