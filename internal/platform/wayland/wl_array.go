//go:build linux

package wayland

import "unsafe"

// wlArrayHeader represents the C wl_array struct layout on 64-bit Linux.
// struct wl_array { size_t size; size_t alloc; void *data; }
type wlArrayHeader struct {
	Size  uint64
	Alloc uint64
	Data  uintptr
}

// wlArrayContainsUint32Impl reads a C wl_array and checks if it contains the
// given uint32 value. The arrayPtr is a valid C pointer from a goffi callback.
//
// go vet flags uintptr→unsafe.Pointer conversions as "possible misuse" but this
// is correct: the pointer comes directly from libwayland's wl_array passed via
// goffi callback trampoline. The C memory is valid for the duration of the callback.
//
//go:nocheckptr
func wlArrayContainsUint32Impl(arrayPtr uintptr, target uint32) bool {
	if arrayPtr == 0 {
		return false
	}
	arr := *(*wlArrayHeader)(unsafe.Pointer(arrayPtr)) //nolint:govet // C pointer from goffi callback
	if arr.Size == 0 || arr.Data == 0 {
		return false
	}
	count := int(arr.Size / 4) // each element is uint32 (4 bytes)
	for i := range count {
		val := *(*uint32)(unsafe.Pointer(arr.Data + uintptr(i)*4)) //nolint:govet // C array element access
		if val == target {
			return true
		}
	}
	return false
}
