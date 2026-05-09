//go:build linux

package x11

import (
	"testing"

	"github.com/gogpu/gpucontext"
)

func TestParseXftDPI(t *testing.T) {
	tests := []struct {
		name      string
		resources string
		want      float64
	}{
		{
			name:      "standard 96 DPI",
			resources: "Xft.dpi:\t96\n",
			want:      96,
		},
		{
			name:      "HiDPI 144",
			resources: "Xft.dpi:\t144\n",
			want:      144,
		},
		{
			name:      "HiDPI 192",
			resources: "Xft.dpi:\t192\n",
			want:      192,
		},
		{
			name:      "space separator",
			resources: "Xft.dpi: 120\n",
			want:      120,
		},
		{
			name:      "among other resources",
			resources: "Xft.antialias:\t1\nXft.hinting:\t1\nXft.dpi:\t168\nXft.rgba:\trgb\n",
			want:      168,
		},
		{
			name:      "fractional DPI",
			resources: "Xft.dpi:\t120.5\n",
			want:      120.5,
		},
		{
			name:      "no Xft.dpi present",
			resources: "Xft.antialias:\t1\nXft.hinting:\t1\n",
			want:      0,
		},
		{
			name:      "empty string",
			resources: "",
			want:      0,
		},
		{
			name:      "invalid value",
			resources: "Xft.dpi:\tabc\n",
			want:      0,
		},
		{
			name:      "zero DPI",
			resources: "Xft.dpi:\t0\n",
			want:      0,
		},
		{
			name:      "negative DPI",
			resources: "Xft.dpi:\t-96\n",
			want:      0,
		},
		{
			name:      "no trailing newline",
			resources: "Xft.dpi:\t96",
			want:      96,
		},
		{
			name:      "whitespace around value",
			resources: "Xft.dpi:  144  \n",
			want:      144,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseXftDPI(tt.resources)
			if got != tt.want {
				t.Errorf("parseXftDPI() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseXftRGBA(t *testing.T) {
	tests := []struct {
		name      string
		resources string
		want      gpucontext.SubpixelLayout
	}{
		{
			name:      "rgb layout",
			resources: "Xft.rgba:\trgb\n",
			want:      gpucontext.SubpixelRGB,
		},
		{
			name:      "bgr layout",
			resources: "Xft.rgba:\tbgr\n",
			want:      gpucontext.SubpixelBGR,
		},
		{
			name:      "vrgb layout",
			resources: "Xft.rgba:\tvrgb\n",
			want:      gpucontext.SubpixelVRGB,
		},
		{
			name:      "vbgr layout",
			resources: "Xft.rgba:\tvbgr\n",
			want:      gpucontext.SubpixelVBGR,
		},
		{
			name:      "none layout",
			resources: "Xft.rgba:\tnone\n",
			want:      gpucontext.SubpixelNone,
		},
		{
			name:      "space separator",
			resources: "Xft.rgba: rgb\n",
			want:      gpucontext.SubpixelRGB,
		},
		{
			name:      "among other resources",
			resources: "Xft.antialias:\t1\nXft.hinting:\t1\nXft.dpi:\t168\nXft.rgba:\tbgr\n",
			want:      gpucontext.SubpixelBGR,
		},
		{
			name:      "no Xft.rgba present",
			resources: "Xft.antialias:\t1\nXft.hinting:\t1\n",
			want:      gpucontext.SubpixelRGB, // default
		},
		{
			name:      "empty string",
			resources: "",
			want:      gpucontext.SubpixelRGB, // default
		},
		{
			name:      "uppercase RGB",
			resources: "Xft.rgba:\tRGB\n",
			want:      gpucontext.SubpixelRGB,
		},
		{
			name:      "mixed case",
			resources: "Xft.rgba:\tBgr\n",
			want:      gpucontext.SubpixelBGR,
		},
		{
			name:      "unknown value defaults to RGB",
			resources: "Xft.rgba:\tunknown\n",
			want:      gpucontext.SubpixelRGB,
		},
		{
			name:      "no trailing newline",
			resources: "Xft.rgba:\trgb",
			want:      gpucontext.SubpixelRGB,
		},
		{
			name:      "whitespace around value",
			resources: "Xft.rgba:  bgr  \n",
			want:      gpucontext.SubpixelBGR,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseXftRGBA(tt.resources)
			if got != tt.want {
				t.Errorf("parseXftRGBA() = %v, want %v", got, tt.want)
			}
		})
	}
}
