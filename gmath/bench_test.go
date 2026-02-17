package gmath

import (
	"math"
	"runtime"
	"testing"
)

// ---------------------------------------------------------------------------
// Vec2 benchmarks
// ---------------------------------------------------------------------------

func BenchmarkVec2Add(b *testing.B) {
	b.ReportAllocs()
	v1 := NewVec2(1.5, 2.5)
	v2 := NewVec2(3.5, 4.5)
	var result Vec2
	for b.Loop() {
		result = v1.Add(v2)
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec2Sub(b *testing.B) {
	b.ReportAllocs()
	v1 := NewVec2(5.5, 7.5)
	v2 := NewVec2(2.5, 3.5)
	var result Vec2
	for b.Loop() {
		result = v1.Sub(v2)
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec2Mul(b *testing.B) {
	b.ReportAllocs()
	v := NewVec2(3.0, 4.0)
	var result Vec2
	for b.Loop() {
		result = v.Mul(2.5)
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec2Dot(b *testing.B) {
	b.ReportAllocs()
	v1 := NewVec2(1.5, 2.5)
	v2 := NewVec2(3.5, 4.5)
	var result float32
	for b.Loop() {
		result = v1.Dot(v2)
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec2Length(b *testing.B) {
	b.ReportAllocs()
	v := NewVec2(3.0, 4.0)
	var result float32
	for b.Loop() {
		result = v.Length()
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec2LengthSquared(b *testing.B) {
	b.ReportAllocs()
	v := NewVec2(3.0, 4.0)
	var result float32
	for b.Loop() {
		result = v.LengthSquared()
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec2Normalize(b *testing.B) {
	b.ReportAllocs()
	v := NewVec2(3.0, 4.0)
	var result Vec2
	for b.Loop() {
		result = v.Normalize()
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec2Lerp(b *testing.B) {
	b.ReportAllocs()
	v1 := NewVec2(0, 0)
	v2 := NewVec2(10, 20)
	var result Vec2
	for b.Loop() {
		result = v1.Lerp(v2, 0.5)
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec2Distance(b *testing.B) {
	b.ReportAllocs()
	v1 := NewVec2(0, 0)
	v2 := NewVec2(3, 4)
	var result float32
	for b.Loop() {
		result = v1.Distance(v2)
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec2Rotate(b *testing.B) {
	b.ReportAllocs()
	v := NewVec2(1, 0)
	angle := float32(math.Pi / 4)
	var result Vec2
	for b.Loop() {
		result = v.Rotate(angle)
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec2Clamp(b *testing.B) {
	b.ReportAllocs()
	v := NewVec2(5, -2)
	minV := NewVec2(0, 0)
	maxV := NewVec2(3, 3)
	var result Vec2
	for b.Loop() {
		result = v.Clamp(minV, maxV)
	}
	runtime.KeepAlive(result)
}

// ---------------------------------------------------------------------------
// Vec3 benchmarks
// ---------------------------------------------------------------------------

func BenchmarkVec3Add(b *testing.B) {
	b.ReportAllocs()
	v1 := NewVec3(1.5, 2.5, 3.5)
	v2 := NewVec3(4.5, 5.5, 6.5)
	var result Vec3
	for b.Loop() {
		result = v1.Add(v2)
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec3Sub(b *testing.B) {
	b.ReportAllocs()
	v1 := NewVec3(5.5, 7.5, 9.5)
	v2 := NewVec3(2.5, 3.5, 4.5)
	var result Vec3
	for b.Loop() {
		result = v1.Sub(v2)
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec3Mul(b *testing.B) {
	b.ReportAllocs()
	v := NewVec3(3.0, 4.0, 5.0)
	var result Vec3
	for b.Loop() {
		result = v.Mul(2.5)
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec3Cross(b *testing.B) {
	b.ReportAllocs()
	v1 := NewVec3(1, 0, 0)
	v2 := NewVec3(0, 1, 0)
	var result Vec3
	for b.Loop() {
		result = v1.Cross(v2)
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec3Dot(b *testing.B) {
	b.ReportAllocs()
	v1 := NewVec3(1.5, 2.5, 3.5)
	v2 := NewVec3(4.5, 5.5, 6.5)
	var result float32
	for b.Loop() {
		result = v1.Dot(v2)
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec3Length(b *testing.B) {
	b.ReportAllocs()
	v := NewVec3(3.0, 4.0, 5.0)
	var result float32
	for b.Loop() {
		result = v.Length()
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec3Normalize(b *testing.B) {
	b.ReportAllocs()
	v := NewVec3(3.0, 4.0, 5.0)
	var result Vec3
	for b.Loop() {
		result = v.Normalize()
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec3Lerp(b *testing.B) {
	b.ReportAllocs()
	v1 := NewVec3(0, 0, 0)
	v2 := NewVec3(10, 20, 30)
	var result Vec3
	for b.Loop() {
		result = v1.Lerp(v2, 0.5)
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec3Reflect(b *testing.B) {
	b.ReportAllocs()
	v := NewVec3(1, -1, 0).Normalize()
	n := NewVec3(0, 1, 0)
	var result Vec3
	for b.Loop() {
		result = v.Reflect(n)
	}
	runtime.KeepAlive(result)
}

// ---------------------------------------------------------------------------
// Vec4 benchmarks
// ---------------------------------------------------------------------------

func BenchmarkVec4Add(b *testing.B) {
	b.ReportAllocs()
	v1 := NewVec4(1.5, 2.5, 3.5, 4.5)
	v2 := NewVec4(5.5, 6.5, 7.5, 8.5)
	var result Vec4
	for b.Loop() {
		result = v1.Add(v2)
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec4Sub(b *testing.B) {
	b.ReportAllocs()
	v1 := NewVec4(5.5, 6.5, 7.5, 8.5)
	v2 := NewVec4(1.5, 2.5, 3.5, 4.5)
	var result Vec4
	for b.Loop() {
		result = v1.Sub(v2)
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec4Mul(b *testing.B) {
	b.ReportAllocs()
	v := NewVec4(3.0, 4.0, 5.0, 1.0)
	var result Vec4
	for b.Loop() {
		result = v.Mul(2.5)
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec4Dot(b *testing.B) {
	b.ReportAllocs()
	v1 := NewVec4(1.5, 2.5, 3.5, 4.5)
	v2 := NewVec4(5.5, 6.5, 7.5, 8.5)
	var result float32
	for b.Loop() {
		result = v1.Dot(v2)
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec4Length(b *testing.B) {
	b.ReportAllocs()
	v := NewVec4(1.0, 2.0, 3.0, 4.0)
	var result float32
	for b.Loop() {
		result = v.Length()
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec4Normalize(b *testing.B) {
	b.ReportAllocs()
	v := NewVec4(1.0, 2.0, 3.0, 4.0)
	var result Vec4
	for b.Loop() {
		result = v.Normalize()
	}
	runtime.KeepAlive(result)
}

func BenchmarkVec4Lerp(b *testing.B) {
	b.ReportAllocs()
	v1 := NewVec4(0, 0, 0, 0)
	v2 := NewVec4(10, 20, 30, 40)
	var result Vec4
	for b.Loop() {
		result = v1.Lerp(v2, 0.5)
	}
	runtime.KeepAlive(result)
}

// ---------------------------------------------------------------------------
// Mat4 benchmarks — the core transform pipeline
// ---------------------------------------------------------------------------

func BenchmarkMat4Identity(b *testing.B) {
	b.ReportAllocs()
	var result Mat4
	for b.Loop() {
		result = Identity4()
	}
	runtime.KeepAlive(result)
}

func BenchmarkMat4Mul(b *testing.B) {
	b.ReportAllocs()
	// Realistic scenario: model * view matrix multiplication
	m1 := Translation(1, 2, 3)
	m2 := RotationZ(0.5)
	var result Mat4
	for b.Loop() {
		result = m1.Mul(m2)
	}
	runtime.KeepAlive(result)
}

func BenchmarkMat4MulChain(b *testing.B) {
	// Simulates typical MVP chain: projection * view * model
	b.ReportAllocs()
	model := Translation(1, 2, 3)
	view := LookAt(NewVec3(0, 0, 5), Zero3(), UnitY())
	proj := Perspective(float32(math.Pi/4), 16.0/9.0, 0.1, 100)
	var result Mat4
	for b.Loop() {
		result = proj.Mul(view.Mul(model))
	}
	runtime.KeepAlive(result)
}

func BenchmarkMat4MulVec4(b *testing.B) {
	// Per-vertex transform: the hottest inner loop in GPU frameworks
	b.ReportAllocs()
	mvp := Perspective(float32(math.Pi/4), 16.0/9.0, 0.1, 100).
		Mul(LookAt(NewVec3(0, 0, 5), Zero3(), UnitY())).
		Mul(Translation(1, 2, 3))
	v := NewVec4(1.0, 2.0, 3.0, 1.0)
	var result Vec4
	for b.Loop() {
		result = mvp.MulVec4(v)
	}
	runtime.KeepAlive(result)
}

func BenchmarkMat4MulVec3(b *testing.B) {
	b.ReportAllocs()
	m := Translation(10, 20, 30)
	v := NewVec3(1, 2, 3)
	var result Vec3
	for b.Loop() {
		result = m.MulVec3(v)
	}
	runtime.KeepAlive(result)
}

func BenchmarkMat4Transpose(b *testing.B) {
	b.ReportAllocs()
	m := Translation(1, 2, 3)
	var result Mat4
	for b.Loop() {
		result = m.Transpose()
	}
	runtime.KeepAlive(result)
}

func BenchmarkMat4Determinant(b *testing.B) {
	b.ReportAllocs()
	m := LookAt(NewVec3(0, 0, 5), Zero3(), UnitY())
	var result float32
	for b.Loop() {
		result = m.Determinant()
	}
	runtime.KeepAlive(result)
}

func BenchmarkMat4Perspective(b *testing.B) {
	b.ReportAllocs()
	var result Mat4
	for b.Loop() {
		result = Perspective(float32(math.Pi/4), 16.0/9.0, 0.1, 100)
	}
	runtime.KeepAlive(result)
}

func BenchmarkMat4Orthographic(b *testing.B) {
	b.ReportAllocs()
	var result Mat4
	for b.Loop() {
		result = Orthographic(-1, 1, -1, 1, 0.1, 100)
	}
	runtime.KeepAlive(result)
}

func BenchmarkMat4LookAt(b *testing.B) {
	b.ReportAllocs()
	eye := NewVec3(0, 0, 5)
	target := Zero3()
	up := UnitY()
	var result Mat4
	for b.Loop() {
		result = LookAt(eye, target, up)
	}
	runtime.KeepAlive(result)
}

func BenchmarkMat4Translation(b *testing.B) {
	b.ReportAllocs()
	var result Mat4
	for b.Loop() {
		result = Translation(1, 2, 3)
	}
	runtime.KeepAlive(result)
}

func BenchmarkMat4Scale(b *testing.B) {
	b.ReportAllocs()
	var result Mat4
	for b.Loop() {
		result = Scale(2, 3, 4)
	}
	runtime.KeepAlive(result)
}

func BenchmarkMat4RotationX(b *testing.B) {
	b.ReportAllocs()
	var result Mat4
	for b.Loop() {
		result = RotationX(0.5)
	}
	runtime.KeepAlive(result)
}

func BenchmarkMat4RotationY(b *testing.B) {
	b.ReportAllocs()
	var result Mat4
	for b.Loop() {
		result = RotationY(0.5)
	}
	runtime.KeepAlive(result)
}

func BenchmarkMat4RotationZ(b *testing.B) {
	b.ReportAllocs()
	var result Mat4
	for b.Loop() {
		result = RotationZ(0.5)
	}
	runtime.KeepAlive(result)
}

func BenchmarkMat4RotationAxis(b *testing.B) {
	b.ReportAllocs()
	axis := NewVec3(1, 1, 0)
	var result Mat4
	for b.Loop() {
		result = RotationAxis(axis, 0.5)
	}
	runtime.KeepAlive(result)
}

// ---------------------------------------------------------------------------
// Color benchmarks
// ---------------------------------------------------------------------------

func BenchmarkColorRGBA(b *testing.B) {
	b.ReportAllocs()
	var result Color
	for b.Loop() {
		result = RGBA(0.5, 0.6, 0.7, 0.8)
	}
	runtime.KeepAlive(result)
}

func BenchmarkColorHex(b *testing.B) {
	b.ReportAllocs()
	b.Run("RGB", func(b *testing.B) {
		b.ReportAllocs()
		var result Color
		for b.Loop() {
			result = Hex(0xFF8000)
		}
		runtime.KeepAlive(result)
	})
	b.Run("RGBA", func(b *testing.B) {
		b.ReportAllocs()
		var result Color
		for b.Loop() {
			result = Hex(0xFF000080)
		}
		runtime.KeepAlive(result)
	})
}

func BenchmarkColorLerp(b *testing.B) {
	b.ReportAllocs()
	c1 := Black
	c2 := White
	var result Color
	for b.Loop() {
		result = c1.Lerp(c2, 0.5)
	}
	runtime.KeepAlive(result)
}

func BenchmarkColorPremultiply(b *testing.B) {
	b.ReportAllocs()
	c := NewColor(1, 0.5, 0.25, 0.5)
	var result Color
	for b.Loop() {
		result = c.Premultiply()
	}
	runtime.KeepAlive(result)
}

func BenchmarkColorToVec4(b *testing.B) {
	b.ReportAllocs()
	c := NewColor(0.1, 0.2, 0.3, 0.4)
	var result Vec4
	for b.Loop() {
		result = c.ToVec4()
	}
	runtime.KeepAlive(result)
}

func BenchmarkColorWithAlpha(b *testing.B) {
	b.ReportAllocs()
	c := Red
	var result Color
	for b.Loop() {
		result = c.WithAlpha(0.5)
	}
	runtime.KeepAlive(result)
}

// ---------------------------------------------------------------------------
// Batch benchmarks — simulating realistic per-frame workloads
// ---------------------------------------------------------------------------

// BenchmarkTransformBatch simulates transforming 1000 vertices through
// a typical MVP matrix, which is the core per-frame workload.
func BenchmarkTransformBatch(b *testing.B) {
	b.ReportAllocs()

	mvp := Perspective(float32(math.Pi/4), 16.0/9.0, 0.1, 100).
		Mul(LookAt(NewVec3(0, 0, 5), Zero3(), UnitY())).
		Mul(Translation(1, 2, 3))

	const vertexCount = 1000
	vertices := make([]Vec4, vertexCount)
	for i := range vertices {
		f := float32(i)
		vertices[i] = NewVec4(f*0.1, f*0.2, f*0.3, 1.0)
	}
	output := make([]Vec4, vertexCount)

	b.ResetTimer()
	for b.Loop() {
		for i, v := range vertices {
			output[i] = mvp.MulVec4(v)
		}
	}
	runtime.KeepAlive(output)
}

// BenchmarkColorLerpBatch simulates blending 1000 color pairs,
// common in gradient and animation rendering.
func BenchmarkColorLerpBatch(b *testing.B) {
	b.ReportAllocs()

	const count = 1000
	colors1 := make([]Color, count)
	colors2 := make([]Color, count)
	output := make([]Color, count)

	for i := range colors1 {
		f := float32(i) / float32(count)
		colors1[i] = NewColor(f, 1-f, f*0.5, 1)
		colors2[i] = NewColor(1-f, f, 0.5, 0.8)
	}

	b.ResetTimer()
	for b.Loop() {
		for i := range colors1 {
			output[i] = colors1[i].Lerp(colors2[i], 0.5)
		}
	}
	runtime.KeepAlive(output)
}
