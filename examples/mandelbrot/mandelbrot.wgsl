// TODO(jbunds): implement efficient anti-aliasing
//
// i tried implementing both 4x SSAA and FXAA, but the results were inadequate, and the adverse performance impact was unacceptable

struct params { // field declaration order must cohere with the corresponding Go struct field declaration order
  paletteSize:  u32,
  frameCounter: f32,
  iterations:   f32,
  pad:          f32,

  width:        f32,
  height:       f32,
  zoomHi:       f32,
  zoomLo:       f32,

  targetXHi:    f32,
  targetYHi:    f32,
  targetXLo:    f32,
  targetYLo:    f32,
};

@group(0) @binding(0) var<uniform> p: params;
@group(0) @binding(1) var<storage, read> palette: array<u32>;
@group(1) @binding(0) var screenTex: texture_storage_2d<bgra8unorm, write>;

// emulated 64-bit floating-point math functions

fn to_dd(val: f32) -> vec2<f32> { return vec2<f32>(val, 0.0); }

fn primitive_sum(a: f32, b: f32) -> vec2<f32> {
  let s = a + b;
  let v = s - a;
  let e = (a - (s - v)) + (b - v);
  return vec2<f32>(s, e);
}

fn dd_add(a: vec2<f32>, b: vec2<f32>) -> vec2<f32> {
  let s_x  =  a.x + b.x;
  let s_y  = (a.x - (s_x  - (s_x  - a.x))) + (b.x - (s_x - a.x));
  let t_x  =  a.y + b.y;
  let t_y  = (a.y - (t_x  - (t_x  - a.y))) + (b.y - (t_x - a.y));
  let c1   =  s_y + t_x;
  let v1_x =  s_x + c1;
  let v1_y = (s_x - (v1_x - (v1_x - s_x))) + (c1 - (v1_x - s_x));

  let rem = v1_x + (v1_y + t_y);
  return vec2<f32>(rem, (v1_y + t_y) - (rem - v1_x));
}

fn dd_mul(a: vec2<f32>, b: vec2<f32>) -> vec2<f32> {
  let p_hi     = a.x * b.x;
  let p_lo     = fma(a.x, b.x, -p_hi);
  let cross    = a.x * b.y + a.y * b.x;
  let trailing = fma(a.y, b.y, cross + p_lo);
  let s        = p_hi + trailing;
  return vec2<f32>(s, trailing - (s - p_hi));
}

fn mandelbrot(cx: vec2<f32>, cy: vec2<f32>) -> f32 {
  var x  = vec2<f32>(0.0);
  var y  = vec2<f32>(0.0);
  var x2 = vec2<f32>(0.0);
  var y2 = vec2<f32>(0.0);

  let limit = u32(p.iterations);
  var i     = 0u;

  for (; i < limit; i++) {
    y  = dd_add(dd_mul(x,   y) * 2.0, cy);
    x  = dd_add(dd_add(x2, -y2), cx);

    x2 = dd_mul(x, x);
    y2 = dd_mul(y, y);

    if (x2.x + y2.x > 4.0) { break; }
  }

  if (i >= limit) { return p.iterations; }

  return f32(i) + 1.0 - log2(log2(x2.x + y2.x) * 0.5);
}

@compute @workgroup_size(16, 8, 1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
  if (id.x >= u32(p.width) || id.y >= u32(p.height)) { return; }

  let screenX = f32(id.x);
  let screenY = f32(id.y);
  let minDim  = min(p.width, p.height);

  let zoom    = vec2<f32>(p.zoomHi,    p.zoomLo);
  let targetX = vec2<f32>(p.targetXHi, p.targetXLo);
  let targetY = vec2<f32>(p.targetYHi, p.targetYLo);

  let scaleX  = to_dd((screenX - (p.width / 2.0)) / minDim);
  let scaleY  = to_dd((p.height / 2.0 - screenY)  / minDim);

  let cx      = dd_add(targetX, dd_mul(scaleX, zoom));
  let cy      = dd_add(targetY, dd_mul(scaleY, zoom));

  let iter    = mandelbrot(cx, cy);

  var color: vec4<f32>;
  if (iter >= p.iterations) {
    color   = vec4<f32>(0.0, 0.0, 0.0, 1.0);
  } else {
    let dyn_div     = 80.0 + (p.frameCounter * 0.5);
    let clamped_div = min(dyn_div, 600.0);
    let idx         = u32((iter / clamped_div) * f32(p.paletteSize)) % p.paletteSize;
    color           = unpack4x8unorm(palette[idx]);
  }

  textureStore(screenTex, vec2<u32>(id.xy), color);
}
