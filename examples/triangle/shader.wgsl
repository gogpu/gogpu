// Triangle shader
//
// This shader renders a simple colored triangle.

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) color: vec3<f32>,
};

@vertex
fn vs_main(@builtin(vertex_index) vertex_index: u32) -> VertexOutput {
    // Triangle vertices (hardcoded for simplicity)
    var positions = array<vec2<f32>, 3>(
        vec2<f32>(0.0, 0.5),    // top
        vec2<f32>(-0.5, -0.5),  // bottom left
        vec2<f32>(0.5, -0.5),   // bottom right
    );

    var colors = array<vec3<f32>, 3>(
        vec3<f32>(1.0, 0.0, 0.0),  // red
        vec3<f32>(0.0, 1.0, 0.0),  // green
        vec3<f32>(0.0, 0.0, 1.0),  // blue
    );

    var output: VertexOutput;
    output.position = vec4<f32>(positions[vertex_index], 0.0, 1.0);
    output.color = colors[vertex_index];
    return output;
}

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    return vec4<f32>(input.color, 1.0);
}
