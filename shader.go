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
