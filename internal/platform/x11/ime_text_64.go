//go:build linux && (amd64 || arm64 || ppc64 || ppc64le || riscv64 || s390x || mips64 || mips64le || loong64)

package x11

// ximText is the LP64 layout of XIMText. XIMText uses either a wchar_t or
// multi-byte string union; XIM UTF-8 locales use the multi-byte arm.
//
// The explicit padding is part of the C ABI: unsigned short is followed by
// an eight-byte-aligned pointer, and Bool is followed by an aligned pointer
// union. Keeping this layout explicit avoids interpreting the callback's
// payload at the wrong offsets on 64-bit Linux.
type ximText struct {
	Length       uint16
	_            [6]byte
	Feedback     uintptr
	EncodingWide int32
	_            [4]byte
	String       uintptr
}
