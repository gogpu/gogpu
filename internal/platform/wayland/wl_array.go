//go:build linux

package wayland

import "unsafe"

// wlArrayContainsUint32 checks if a C wl_array contains the given uint32 value.
// arrayPtr is a uintptr to a C wl_array struct from a goffi callback.
func wlArrayContainsUint32(arrayPtr uintptr, target uint32) bool {
	if arrayPtr == 0 {
		return false
	}

	// struct wl_array { size_t size; size_t alloc; void *data; }
	// On 64-bit Linux: offsets 0, 8, 16. Total 24 bytes.
	//
	// We use the purego/ebitengine pattern to convert uintptr to unsafe.Pointer:
	// take address of uintptr, cast to *unsafe.Pointer, dereference.
	// This avoids go vet "possible misuse of unsafe.Pointer" for C pointers
	// received from goffi callbacks (non-Go-managed memory).
	arrayBase := *(*unsafe.Pointer)(unsafe.Pointer(&arrayPtr))
	sizeField := *(*uint64)(arrayBase)
	dataField := *(*uintptr)(unsafe.Add(arrayBase, 16))

	if sizeField == 0 || dataField == 0 {
		return false
	}

	dataBase := *(*unsafe.Pointer)(unsafe.Pointer(&dataField))
	count := int(sizeField / 4)
	for i := range count {
		val := *(*uint32)(unsafe.Add(dataBase, uintptr(i)*4))
		if val == target {
			return true
		}
	}
	return false
}
