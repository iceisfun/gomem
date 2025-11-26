package pod

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"strings"
	"unsafe"

	"gomem/process"
)

// ReadStruct reads a struct from process memory at the given address.
// It handles fields with "pod" tags.
func ReadStruct(proc process.Process, addr process.ProcessMemoryAddress, v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("v must be a non-nil pointer to a struct")
	}

	elem := rv.Elem()
	if elem.Kind() != reflect.Struct {
		return fmt.Errorf("v must point to a struct")
	}

	// 1. Read the raw bytes of the struct
	size := int(elem.Type().Size())
	data, err := proc.ReadMemory(addr, process.ProcessMemorySize(size))
	if err != nil {
		return fmt.Errorf("failed to read struct memory at %v: %w", addr, err)
	}

	// 2. Deserialize basic fields
	// We use a temporary struct or direct memory copy if possible.
	// However, because we need to handle pointers specially, we can't just copy bytes if the struct contains pointers.
	// But we can copy bytes to the struct first (assuming layout matches) and then fix up pointers.
	// WARNING: Copying raw bytes into a struct with pointers is dangerous if the GC sees invalid pointers.
	// But since we are constructing it, maybe it's okay if we fix them immediately?
	// Actually, if we copy random remote addresses into Go pointers, the GC might crash if it traces them.
	// So we should NOT copy raw bytes directly into fields that are pointers.

	// Safer approach: Iterate over fields.

	for i := 0; i < elem.NumField(); i++ {
		field := elem.Field(i)
		fieldType := elem.Type().Field(i)

		// Skip unexported fields? Or try to set them if possible (using unsafe).
		// For now assume exported.

		// Calculate offset of the field
		offset := fieldType.Offset
		fieldSize := fieldType.Type.Size()

		if offset+fieldSize > uintptr(len(data)) {
			return fmt.Errorf("field %s out of bounds", fieldType.Name)
		}

		fieldData := data[offset : offset+fieldSize]

		if !field.CanSet() {
			continue
		}

		if field.Kind() == reflect.Ptr {
			// It's a pointer. The data in memory is the address (uint64 on 64-bit).
			// We read the address.
			var ptrAddr uint64
			if len(fieldData) == 4 {
				ptrAddr = uint64(binary.LittleEndian.Uint32(fieldData))
			} else if len(fieldData) == 8 {
				ptrAddr = binary.LittleEndian.Uint64(fieldData)
			} else {
				// Unknown pointer size
				continue
			}

			// Check tags
			tag := fieldType.Tag.Get("pod")
			if strings.Contains(tag, "valid_pointer") {
				// Recursively read the object
				if ptrAddr == 0 {
					field.Set(reflect.Zero(field.Type()))
					continue
				}

				// Check if address is valid
				if !proc.IsValidAddress(process.ProcessMemoryAddress(ptrAddr)) {
					if strings.Contains(tag, "err_failure") {
						return fmt.Errorf("invalid pointer address %x for field %s", ptrAddr, fieldType.Name)
					}
					field.Set(reflect.Zero(field.Type()))
					continue
				}

				// Allocate new object of the pointed-to type
				newObj := reflect.New(fieldType.Type.Elem())

				// Recursively read
				err := ReadStruct(proc, process.ProcessMemoryAddress(ptrAddr), newObj.Interface())
				if err != nil {
					if strings.Contains(tag, "err_failure") {
						return fmt.Errorf("failed to read pointed struct for field %s: %w", fieldType.Name, err)
					}
					field.Set(reflect.Zero(field.Type()))
					continue
				}

				field.Set(newObj)
			} else {
				// Just a pointer, but we can't set a remote address to a Go pointer.
				// If the user didn't ask to read it (no valid_pointer tag), we probably should leave it nil
				// or we can't really do anything useful with it in a Go pointer field.
				// Unless the field type is uintptr or uint64, but here it is reflect.Ptr.
				// We leave it as nil (or whatever it was).
			}
		} else if field.Kind() == reflect.Struct {
			// Nested struct. Recursively read?
			// Since it's embedded (not a pointer), it's part of the memory block we just read.
			// We can just decode it from the data we already have.
			// But we need to handle its fields (which might have pointers).
			// So we call ReadStruct logic on the field, but we don't need to read from process memory again,
			// we just need to process the bytes we already have?
			// Actually, ReadStruct takes an address.
			// So we can call ReadStruct with (addr + offset).
			// This will re-read memory, which is slightly inefficient but correct.
			// Or we can implement a `readFromBytes` helper.
			// For simplicity, let's recurse with address.
			err := ReadStruct(proc, addr+process.ProcessMemoryAddress(offset), field.Addr().Interface())
			if err != nil {
				return err
			}
		} else {
			// POD type (int, uint, float, etc.)
			// We can use binary.Read or unsafe copy.
			// Since we have the bytes, we can use unsafe to set the value.
			// Or use binary.Read on the field address?
			// reflect.NewAt can create a pointer to the field.

			// Simple approach for common types:
			switch field.Kind() {
			case reflect.Uint8:
				field.SetUint(uint64(fieldData[0]))
			case reflect.Uint16:
				field.SetUint(uint64(binary.LittleEndian.Uint16(fieldData)))
			case reflect.Uint32:
				field.SetUint(uint64(binary.LittleEndian.Uint32(fieldData)))
			case reflect.Uint64:
				field.SetUint(binary.LittleEndian.Uint64(fieldData))
			case reflect.Int8:
				field.SetInt(int64(int8(fieldData[0])))
			case reflect.Int16:
				field.SetInt(int64(int16(binary.LittleEndian.Uint16(fieldData))))
			case reflect.Int32:
				field.SetInt(int64(int32(binary.LittleEndian.Uint32(fieldData))))
			case reflect.Int64:
				field.SetInt(int64(binary.LittleEndian.Uint64(fieldData)))
			case reflect.Float32:
				bits := binary.LittleEndian.Uint32(fieldData)
				field.SetFloat(float64(*(*float32)(unsafe.Pointer(&bits))))
			case reflect.Float64:
				bits := binary.LittleEndian.Uint64(fieldData)
				field.SetFloat(*(*float64)(unsafe.Pointer(&bits)))
			case reflect.Bool:
				field.SetBool(fieldData[0] != 0)
			// Add array/slice handling if needed
			default:
				// Try binary.Read for other types
				if err := binary.Read(bytes.NewReader(fieldData), binary.LittleEndian, field.Addr().Interface()); err != nil {
					// Ignore error or log?
				}
			}
		}
	}

	return nil
}
