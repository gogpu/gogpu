package gogpu

// damageOverlayShaderSource is the WGSL shader for rendering flat-color quads
// in the damage debug overlay. Uses the same pixel-to-NDC conversion pattern
// as positionedQuadShaderSource in shader.go but without texture sampling —
// output is a solid color from the uniform buffer.
//
// Uniform layout (48 bytes, 16-byte aligned):
//
//	rect:   vec4<f32>  — x, y, width, height in physical pixels
//	screen: vec2<f32>  — surface width, height in physical pixels
//	_pad:   vec2<f32>  — alignment padding (vec4 boundary)
//	color:  vec4<f32>  — RGBA with pre-multiplied fade alpha
const damageOverlayShaderSource = `
struct RectUniforms {
    rect:   vec4<f32>,
    screen: vec2<f32>,
    _pad:   vec2<f32>,
    color:  vec4<f32>,
}

@group(0) @binding(0) var<uniform> uniforms: RectUniforms;

@vertex
fn vs_main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    var corners = array<vec2<f32>, 6>(
        vec2<f32>(0.0, 0.0),
        vec2<f32>(0.0, 1.0),
        vec2<f32>(1.0, 1.0),
        vec2<f32>(0.0, 0.0),
        vec2<f32>(1.0, 1.0),
        vec2<f32>(1.0, 0.0)
    );

    let corner = corners[idx];
    let pixelX = uniforms.rect.x + corner.x * uniforms.rect.z;
    let pixelY = uniforms.rect.y + corner.y * uniforms.rect.w;
    let ndcX = (pixelX / uniforms.screen.x) * 2.0 - 1.0;
    let ndcY = 1.0 - (pixelY / uniforms.screen.y) * 2.0;
    return vec4<f32>(ndcX, ndcY, 0.0, 1.0);
}

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return uniforms.color;
}
`
