//go:build linux

package x11

import (
	"testing"
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
