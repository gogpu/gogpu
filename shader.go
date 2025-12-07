package gogpu

// coloredTriangleShaderSource is the WGSL shader for a vertex-colored triangle.
const coloredTriangleShaderSource = `
struct VertexOutput {
    @builtin(position) position: vec4f,
    @location(0) color: vec3f,
}

@vertex
fn vs_main(@builtin(vertex_index) vertexIndex: u32) -> VertexOutput {
    // Triangle vertices in clip space
    var positions = array<vec2f, 3>(
        vec2f( 0.0,  0.5),  // top
        vec2f(-0.5, -0.5),  // bottom left
        vec2f( 0.5, -0.5)   // bottom right
    );

    // Vertex colors (RGB)
    var colors = array<vec3f, 3>(
        vec3f(1.0, 0.0, 0.0),  // red
        vec3f(0.0, 1.0, 0.0),  // green
        vec3f(0.0, 0.0, 1.0)   // blue
    );

    var output: VertexOutput;
    output.position = vec4f(positions[vertexIndex], 0.0, 1.0);
    output.color = colors[vertexIndex];
    return output;
}

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4f {
    return vec4f(input.color, 1.0);
}
`

// TexturedQuadShader returns the WGSL shader for rendering textured quads.
// Exported for use in examples and advanced rendering scenarios.
func TexturedQuadShader() string {
	return texturedQuadShaderSource
}

// SimpleTextureShader returns the WGSL shader for full-screen textured quads.
func SimpleTextureShader() string {
	return simpleTextureShaderSource
}

// texturedQuadShaderSource is the WGSL shader for rendering textured quads.
const texturedQuadShaderSource = `
// Uniform buffer for transforms
struct Uniforms {
    transform: mat4x4f,
    color: vec4f,
}

@group(0) @binding(0) var<uniform> uniforms: Uniforms;
@group(1) @binding(0) var texSampler: sampler;
@group(1) @binding(1) var tex: texture_2d<f32>;

struct VertexInput {
    @location(0) position: vec2f,
    @location(1) uv: vec2f,
}

struct VertexOutput {
    @builtin(position) position: vec4f,
    @location(0) uv: vec2f,
}

@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    output.position = uniforms.transform * vec4f(input.position, 0.0, 1.0);
    output.uv = input.uv;
    return output;
}

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4f {
    let texColor = textureSample(tex, texSampler, input.uv);
    return texColor * uniforms.color;
}
`

// simpleTextureShaderSource is a simpler WGSL shader for full-screen textured quads
// without transforms (useful for basic image display).
const simpleTextureShaderSource = `
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var tex: texture_2d<f32>;

struct VertexOutput {
    @builtin(position) position: vec4f,
    @location(0) uv: vec2f,
}

@vertex
fn vs_main(@builtin(vertex_index) vertexIndex: u32) -> VertexOutput {
    // Full-screen quad vertices (2 triangles)
    var positions = array<vec2f, 6>(
        vec2f(-1.0,  1.0),  // top-left
        vec2f(-1.0, -1.0),  // bottom-left
        vec2f( 1.0, -1.0),  // bottom-right
        vec2f(-1.0,  1.0),  // top-left
        vec2f( 1.0, -1.0),  // bottom-right
        vec2f( 1.0,  1.0)   // top-right
    );

    var uvs = array<vec2f, 6>(
        vec2f(0.0, 0.0),  // top-left
        vec2f(0.0, 1.0),  // bottom-left
        vec2f(1.0, 1.0),  // bottom-right
        vec2f(0.0, 0.0),  // top-left
        vec2f(1.0, 1.0),  // bottom-right
        vec2f(1.0, 0.0)   // top-right
    );

    var output: VertexOutput;
    output.position = vec4f(positions[vertexIndex], 0.0, 1.0);
    output.uv = uvs[vertexIndex];
    return output;
}

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4f {
    return textureSample(tex, texSampler, input.uv);
}
`
