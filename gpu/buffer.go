package gpu

// BufferUsage describes how a buffer will be used.
type BufferUsage uint32

const (
	BufferUsageMapRead      BufferUsage = 1 << 0
	BufferUsageMapWrite     BufferUsage = 1 << 1
	BufferUsageCopySrc      BufferUsage = 1 << 2
	BufferUsageCopyDst      BufferUsage = 1 << 3
	BufferUsageIndex        BufferUsage = 1 << 4
	BufferUsageVertex       BufferUsage = 1 << 5
	BufferUsageUniform      BufferUsage = 1 << 6
	BufferUsageStorage      BufferUsage = 1 << 7
	BufferUsageIndirect     BufferUsage = 1 << 8
	BufferUsageQueryResolve BufferUsage = 1 << 9
)

// BufferDescriptor describes how to create a buffer.
type BufferDescriptor struct {
	Label            string
	Size             uint64
	Usage            BufferUsage
	MappedAtCreation bool
}

// Buffer represents a GPU buffer.
type Buffer struct {
	label string
	size  uint64
	usage BufferUsage
	// Internal handle will be added when integrating with webgpu
}

// Label returns the buffer label.
func (b *Buffer) Label() string {
	return b.label
}

// Size returns the buffer size in bytes.
func (b *Buffer) Size() uint64 {
	return b.size
}

// Usage returns the buffer usage flags.
func (b *Buffer) Usage() BufferUsage {
	return b.usage
}

// MapState represents buffer mapping state.
type MapState uint8

const (
	MapStateUnmapped MapState = iota
	MapStatePending
	MapStateMapped
)
