//go:build windows

package platform

import "testing"

func TestDlgBuildFilterSpec(t *testing.T) {
	tests := []struct {
		name string
		exts []string
		want string
	}{
		{
			name: "already glob-prefixed",
			exts: []string{"*.png", "*.jpg"},
			want: "*.png;*.jpg",
		},
		{
			name: "bare extensions",
			exts: []string{"png", "jpg"},
			want: "*.png;*.jpg",
		},
		{
			name: "dot-prefixed",
			exts: []string{".png", ".gif"},
			want: "*.png;*.gif",
		},
		{
			name: "mixed formats",
			exts: []string{"*.png", "jpg", ".gif"},
			want: "*.png;*.jpg;*.gif",
		},
		{
			name: "single extension",
			exts: []string{"pdf"},
			want: "*.pdf",
		},
		{
			name: "nil slice falls back to wildcard",
			exts: nil,
			want: "*.*",
		},
		{
			name: "empty slice falls back to wildcard",
			exts: []string{},
			want: "*.*",
		},
		{
			name: "blank entries are skipped",
			exts: []string{"", "*.", "."},
			want: "*.*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dlgBuildFilterSpec(tt.exts)
			if got != tt.want {
				t.Errorf("dlgBuildFilterSpec(%v) = %q, want %q", tt.exts, got, tt.want)
			}
		})
	}
}
