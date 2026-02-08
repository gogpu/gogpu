package sdf

// circleShaderWGSL is the compute shader for SDF circle/ellipse rendering.
// It computes per-pixel coverage for filled or stroked circles/ellipses
// using signed distance fields and composites onto the target pixel buffer.
const circleShaderWGSL = `
struct SDFCircleParams {
    center_x: f32,
    center_y: f32,
    radius_x: f32,
    radius_y: f32,
    half_stroke_width: f32,
    is_stroked: u32,
    color_r: f32,
    color_g: f32,
    color_b: f32,
    color_a: f32,
    target_width: u32,
    target_height: u32,
}

@group(0) @binding(0) var<uniform> params: SDFCircleParams;
@group(0) @binding(1) var<storage, read_write> pixels: array<u32>;

@compute @workgroup_size(8, 8)
fn main(@builtin(global_invocation_id) global_id: vec3u) {
    let x = global_id.x;
    let y = global_id.y;
    if (x >= params.target_width || y >= params.target_height) {
        return;
    }

    let px = f32(x) + 0.5;
    let py = f32(y) + 0.5;

    var dist: f32;
    if (params.radius_x == params.radius_y) {
        let dx = px - params.center_x;
        let dy = py - params.center_y;
        dist = sqrt(dx * dx + dy * dy) - params.radius_x;
    } else {
        let dx = (px - params.center_x) / params.radius_x;
        let dy = (py - params.center_y) / params.radius_y;
        let min_r = min(params.radius_x, params.radius_y);
        dist = sqrt(dx * dx + dy * dy) * min_r - min_r;
    }

    var sdf: f32;
    if (params.is_stroked == 1u) {
        sdf = abs(dist) - params.half_stroke_width;
    } else {
        sdf = dist;
    }

    let afwidth = 0.7;
    var coverage: f32;
    if (sdf >= afwidth) {
        coverage = 0.0;
    } else if (sdf <= -afwidth) {
        coverage = 1.0;
    } else {
        let t = (sdf + afwidth) / (2.0 * afwidth);
        coverage = 1.0 - t * t * (3.0 - 2.0 * t);
    }

    if (coverage <= 0.0) {
        return;
    }

    let src_a = params.color_a * coverage;
    let src_r = params.color_r * src_a;
    let src_g = params.color_g * src_a;
    let src_b = params.color_b * src_a;

    let idx = y * params.target_width + x;
    let dst_packed = pixels[idx];
    let dst_r = f32((dst_packed >> 0u) & 0xFFu) / 255.0;
    let dst_g = f32((dst_packed >> 8u) & 0xFFu) / 255.0;
    let dst_b = f32((dst_packed >> 16u) & 0xFFu) / 255.0;
    let dst_a = f32((dst_packed >> 24u) & 0xFFu) / 255.0;

    let inv_src_a = 1.0 - src_a;
    let out_r = src_r + dst_r * inv_src_a;
    let out_g = src_g + dst_g * inv_src_a;
    let out_b = src_b + dst_b * inv_src_a;
    let out_a = src_a + dst_a * inv_src_a;

    let packed = (u32(clamp(out_r * 255.0, 0.0, 255.0)) << 0u) |
                 (u32(clamp(out_g * 255.0, 0.0, 255.0)) << 8u) |
                 (u32(clamp(out_b * 255.0, 0.0, 255.0)) << 16u) |
                 (u32(clamp(out_a * 255.0, 0.0, 255.0)) << 24u);
    pixels[idx] = packed;
}
`

// rrectShaderWGSL is the compute shader for SDF rounded rectangle rendering.
// It computes per-pixel coverage for filled or stroked rounded rectangles
// using signed distance fields and composites onto the target pixel buffer.
const rrectShaderWGSL = `
struct SDFRRectParams {
    center_x: f32,
    center_y: f32,
    half_width: f32,
    half_height: f32,
    corner_radius: f32,
    half_stroke_width: f32,
    is_stroked: u32,
    color_r: f32,
    color_g: f32,
    color_b: f32,
    color_a: f32,
    target_width: u32,
    target_height: u32,
    _padding: u32,
}

@group(0) @binding(0) var<uniform> params: SDFRRectParams;
@group(0) @binding(1) var<storage, read_write> pixels: array<u32>;

fn sdf_rrect(px: f32, py: f32) -> f32 {
    let dx = abs(px - params.center_x) - params.half_width + params.corner_radius;
    let dy = abs(py - params.center_y) - params.half_height + params.corner_radius;

    let outside = length(max(vec2f(dx, dy), vec2f(0.0)));
    let inside = min(max(dx, dy), 0.0);

    return outside + inside - params.corner_radius;
}

@compute @workgroup_size(8, 8)
fn main(@builtin(global_invocation_id) global_id: vec3u) {
    let x = global_id.x;
    let y = global_id.y;
    if (x >= params.target_width || y >= params.target_height) {
        return;
    }

    let px = f32(x) + 0.5;
    let py = f32(y) + 0.5;

    let dist = sdf_rrect(px, py);

    var sdf: f32;
    if (params.is_stroked == 1u) {
        sdf = abs(dist) - params.half_stroke_width;
    } else {
        sdf = dist;
    }

    let afwidth = 0.7;
    var coverage: f32;
    if (sdf >= afwidth) {
        coverage = 0.0;
    } else if (sdf <= -afwidth) {
        coverage = 1.0;
    } else {
        let t = (sdf + afwidth) / (2.0 * afwidth);
        coverage = 1.0 - t * t * (3.0 - 2.0 * t);
    }

    if (coverage <= 0.0) {
        return;
    }

    let src_a = params.color_a * coverage;
    let src_r = params.color_r * src_a;
    let src_g = params.color_g * src_a;
    let src_b = params.color_b * src_a;

    let idx = y * params.target_width + x;
    let dst_packed = pixels[idx];
    let dst_r = f32((dst_packed >> 0u) & 0xFFu) / 255.0;
    let dst_g = f32((dst_packed >> 8u) & 0xFFu) / 255.0;
    let dst_b = f32((dst_packed >> 16u) & 0xFFu) / 255.0;
    let dst_a = f32((dst_packed >> 24u) & 0xFFu) / 255.0;

    let inv_src_a = 1.0 - src_a;
    let out_r = src_r + dst_r * inv_src_a;
    let out_g = src_g + dst_g * inv_src_a;
    let out_b = src_b + dst_b * inv_src_a;
    let out_a = src_a + dst_a * inv_src_a;

    let packed = (u32(clamp(out_r * 255.0, 0.0, 255.0)) << 0u) |
                 (u32(clamp(out_g * 255.0, 0.0, 255.0)) << 8u) |
                 (u32(clamp(out_b * 255.0, 0.0, 255.0)) << 16u) |
                 (u32(clamp(out_a * 255.0, 0.0, 255.0)) << 24u);
    pixels[idx] = packed;
}
`

// circleUniformSize is the byte size of SDFCircleParams.
// 12 fields * 4 bytes = 48 bytes.
const circleUniformSize = 48

// rrectUniformSize is the byte size of SDFRRectParams.
// 14 fields * 4 bytes = 56 bytes.
const rrectUniformSize = 56
