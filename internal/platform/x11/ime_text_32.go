//go:build linux && (386 || arm || mips || mipsle)

package x11

// ximText is the ILP32 layout of XIMText. Pointers are four bytes wide, so
// the C ABI only inserts two bytes after length and no padding before string.
type ximText struct {
	Length       uint16
	_            [2]byte
	Feedback     uintptr
	EncodingWide int32
	String       uintptr
}
