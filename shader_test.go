package gogpu

import (
	"strings"
	"testing"
)

func TestShaderSources(t *testing.T) {
	tests := []struct {
		name     string
		fn       func() string
		contains []string
	}{
		{
			name: "TexturedQuadShader",
			fn:   TexturedQuadShader,
			contains: []string{
				"@vertex", "@fragment", "textureSample",
				"sampler", "texture_2d", "uniforms",
			},
		},
		{
			name: "SimpleTextureShader",
			fn:   SimpleTextureShader,
			contains: []string{
				"@vertex", "@fragment", "textureSample",
				"sampler", "texture_2d",
			},
		},
		{
			name: "PositionedQuadShader",
			fn:   PositionedQuadShader,
			contains: []string{
				"@vertex", "@fragment", "textureSample",
				"QuadUniforms", "premultiplied",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shader := tt.fn()
			if shader == "" {
				t.Errorf("%s() returned empty string", tt.name)
			}
			for _, s := range tt.contains {
				if !strings.Contains(shader, s) {
					t.Errorf("%s() missing %q", tt.name, s)
				}
			}
		})
	}
}

func TestColoredTriangleShaderSource(t *testing.T) {
	// The constant is not exported, but we can verify it's non-empty
	// by checking it contains expected WGSL markers.
	shader := coloredTriangleShaderSource
	if shader == "" {
		t.Error("coloredTriangleShaderSource is empty")
	}
	if !strings.Contains(shader, "@vertex") {
		t.Error("coloredTriangleShaderSource missing @vertex")
	}
	if !strings.Contains(shader, "@fragment") {
		t.Error("coloredTriangleShaderSource missing @fragment")
	}
	if !strings.Contains(shader, "1.0, 0.0, 0.0") {
		t.Error("coloredTriangleShaderSource should contain red color (1.0, 0.0, 0.0)")
	}
}
