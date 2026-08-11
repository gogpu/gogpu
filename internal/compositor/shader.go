package compositor

// ShaderEntryVS is the vertex shader entry point name.
const ShaderEntryVS = "vs_main"

// ShaderEntryFS is the fragment shader entry point name.
const ShaderEntryFS = "fs_main"

// ModeOverlay is the debug mode string for visual overlay rendering.
const ModeOverlay = "overlay"

// ModeLog is the debug mode string for structured log output.
const ModeLog = "log"

// OverlayShaderSource is the WGSL shader for rendering flat-color quads
// via instanced draw. Each instance provides its own rect position, size, and
// color through instance-rate vertex attributes.
const OverlayShaderSource = `
struct ScreenUniforms {
    screen: vec2<f32>,
}

@group(0) @binding(0) var<uniform> uniforms: ScreenUniforms;

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) color: vec4<f32>,
}

@vertex
fn vs_main(
    @builtin(vertex_index) vertIdx: u32,
    @location(0) rectXY: vec2<f32>,
    @location(1) rectWH: vec2<f32>,
    @location(2) color: vec4<f32>,
) -> VertexOutput {
    var corners = array<vec2<f32>, 6>(
        vec2<f32>(0.0, 0.0),
        vec2<f32>(0.0, 1.0),
        vec2<f32>(1.0, 1.0),
        vec2<f32>(0.0, 0.0),
        vec2<f32>(1.0, 1.0),
        vec2<f32>(1.0, 0.0)
    );

    let c = corners[vertIdx];
    let px = rectXY.x + c.x * rectWH.x;
    let py = rectXY.y + c.y * rectWH.y;
    let ndcX = (px / uniforms.screen.x) * 2.0 - 1.0;
    let ndcY = 1.0 - (py / uniforms.screen.y) * 2.0;

    var out: VertexOutput;
    out.position = vec4<f32>(ndcX, ndcY, 0.0, 1.0);
    out.color = color;
    return out;
}

@fragment
fn fs_main(@location(0) color: vec4<f32>) -> @location(0) vec4<f32> {
    return color;
}
`
