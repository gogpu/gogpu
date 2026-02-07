package gogpu

// coloredTriangleShaderSource is the WGSL shader for a simple red triangle.
// Uses the same pattern as vulkan-triangle example for maximum compatibility.
const coloredTriangleShaderSource = `
@vertex
fn vs_main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    var positions = array<vec2<f32>, 3>(
        vec2<f32>(0.0, 0.5),    // Top center
        vec2<f32>(-0.5, -0.5),  // Bottom left
        vec2<f32>(0.5, -0.5)    // Bottom right
    );
    return vec4<f32>(positions[idx], 0.0, 1.0);
}

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(1.0, 0.0, 0.0, 1.0);  // Red
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
    transform: mat4x4<f32>,
    color: vec4<f32>,
}

@group(0) @binding(0) var<uniform> uniforms: Uniforms;
@group(1) @binding(0) var texSampler: sampler;
@group(1) @binding(1) var tex: texture_2d<f32>;

struct VertexInput {
    @location(0) position: vec2<f32>,
    @location(1) uv: vec2<f32>,
}

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    output.position = uniforms.transform * vec4<f32>(input.position, 0.0, 1.0);
    output.uv = input.uv;
    return output;
}

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
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
    @builtin(position) position: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vertexIndex: u32) -> VertexOutput {
    // Full-screen quad vertices (2 triangles)
    var positions = array<vec2<f32>, 6>(
        vec2<f32>(-1.0,  1.0),  // top-left
        vec2<f32>(-1.0, -1.0),  // bottom-left
        vec2<f32>( 1.0, -1.0),  // bottom-right
        vec2<f32>(-1.0,  1.0),  // top-left
        vec2<f32>( 1.0, -1.0),  // bottom-right
        vec2<f32>( 1.0,  1.0)   // top-right
    );

    var uvs = array<vec2<f32>, 6>(
        vec2<f32>(0.0, 0.0),  // top-left
        vec2<f32>(0.0, 1.0),  // bottom-left
        vec2<f32>(1.0, 1.0),  // bottom-right
        vec2<f32>(0.0, 0.0),  // top-left
        vec2<f32>(1.0, 1.0),  // bottom-right
        vec2<f32>(1.0, 0.0)   // top-right
    );

    var output: VertexOutput;
    output.position = vec4<f32>(positions[vertexIndex], 0.0, 1.0);
    output.uv = uvs[vertexIndex];
    return output;
}

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    return textureSample(tex, texSampler, input.uv);
}
`

// positionedQuadShaderSource is the WGSL shader for positioned textured quads.
// Uses a uniform buffer for position/size and vertex-less rendering.
// Handles both premultiplied and straight alpha via uniform flag.
// All output is premultiplied — uses BlendFactorOne / OneMinusSrcAlpha.
// Bind group 0: uniforms (transform)
// Bind group 1: sampler + texture
const positionedQuadShaderSource = `
// Uniform buffer for quad positioning
// Layout: [x, y, width, height, screenWidth, screenHeight, alpha, premultiplied]
struct QuadUniforms {
    rect: vec4<f32>,          // x, y, width, height in pixels
    screen: vec2<f32>,        // screen width, height
    alpha: f32,               // opacity (0.0 - 1.0)
    premultiplied: f32,       // >0.5 = premultiplied alpha, <=0.5 = straight alpha
}

@group(0) @binding(0) var<uniform> uniforms: QuadUniforms;
@group(1) @binding(0) var texSampler: sampler;
@group(1) @binding(1) var tex: texture_2d<f32>;

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vertexIndex: u32) -> VertexOutput {
    // Quad corners in normalized [0,1] space
    var corners = array<vec2<f32>, 6>(
        vec2<f32>(0.0, 0.0),  // top-left
        vec2<f32>(0.0, 1.0),  // bottom-left
        vec2<f32>(1.0, 1.0),  // bottom-right
        vec2<f32>(0.0, 0.0),  // top-left
        vec2<f32>(1.0, 1.0),  // bottom-right
        vec2<f32>(1.0, 0.0)   // top-right
    );

    let corner = corners[vertexIndex];

    // Calculate pixel position
    let pixelX = uniforms.rect.x + corner.x * uniforms.rect.z;
    let pixelY = uniforms.rect.y + corner.y * uniforms.rect.w;

    // Convert to NDC: [-1, 1]
    // X: (pixel / screenWidth) * 2 - 1
    // Y: 1 - (pixel / screenHeight) * 2 (Y is flipped)
    let ndcX = (pixelX / uniforms.screen.x) * 2.0 - 1.0;
    let ndcY = 1.0 - (pixelY / uniforms.screen.y) * 2.0;

    var output: VertexOutput;
    output.position = vec4<f32>(ndcX, ndcY, 0.0, 1.0);
    output.uv = corner;
    return output;
}

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    let texColor = textureSample(tex, texSampler, input.uv);
    // Output is always premultiplied (pipeline uses BlendFactorOne).
    if (uniforms.premultiplied > 0.5) {
        // Already premultiplied: scale all channels uniformly by opacity.
        return texColor * uniforms.alpha;
    } else {
        // Straight alpha: premultiply RGB by alpha in shader.
        let a = texColor.a * uniforms.alpha;
        return vec4<f32>(texColor.rgb * a, a);
    }
}
`

// PositionedQuadShader returns the WGSL shader for positioned textured quads.
// This shader uses vertex-less rendering with uniforms for position and size.
func PositionedQuadShader() string {
	return positionedQuadShaderSource
}
