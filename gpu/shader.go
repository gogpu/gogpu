package gpu

// ShaderStage represents shader stages.
type ShaderStage uint32

const (
	ShaderStageVertex   ShaderStage = 1 << 0
	ShaderStageFragment ShaderStage = 1 << 1
	ShaderStageCompute  ShaderStage = 1 << 2
)

// ShaderModuleDescriptor describes how to create a shader module.
type ShaderModuleDescriptor struct {
	Label string
	Code  string // WGSL source code
}

// ShaderModule represents a compiled shader.
type ShaderModule struct {
	label string
	// Internal handle will be added when integrating with webgpu
}

// Label returns the shader module label.
func (s *ShaderModule) Label() string {
	return s.label
}

// VertexState describes the vertex stage of a render pipeline.
type VertexState struct {
	Module     *ShaderModule
	EntryPoint string
	Buffers    []VertexBufferLayout
}

// FragmentState describes the fragment stage of a render pipeline.
type FragmentState struct {
	Module     *ShaderModule
	EntryPoint string
	Targets    []ColorTargetState
}

// ComputeState describes the compute stage of a compute pipeline.
type ComputeState struct {
	Module     *ShaderModule
	EntryPoint string
}
