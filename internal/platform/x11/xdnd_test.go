//go:build linux

package x11

import "testing"

func TestParseURIList(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single file",
			input: "file:///home/user/document.txt\r\n",
			want:  []string{"/home/user/document.txt"},
		},
		{
			name:  "multiple files",
			input: "file:///home/user/a.txt\r\nfile:///home/user/b.txt\r\n",
			want:  []string{"/home/user/a.txt", "/home/user/b.txt"},
		},
		{
			name:  "percent encoded spaces",
			input: "file:///home/user/my%20file.txt\r\n",
			want:  []string{"/home/user/my file.txt"},
		},
		{
			name:  "percent encoded unicode",
			input: "file:///home/user/%D0%B4%D0%BE%D0%BA.txt\r\n",
			want:  []string{"/home/user/\u0434\u043e\u043a.txt"},
		},
		{
			name:  "skip comments",
			input: "# comment line\r\nfile:///home/user/a.txt\r\n# another comment\r\n",
			want:  []string{"/home/user/a.txt"},
		},
		{
			name:  "skip non-file URIs",
			input: "http://example.com/file.txt\r\nfile:///home/user/a.txt\r\n",
			want:  []string{"/home/user/a.txt"},
		},
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name:  "only comments",
			input: "# comment\r\n# another\r\n",
			want:  nil,
		},
		{
			name:  "LF line endings",
			input: "file:///home/user/a.txt\nfile:///home/user/b.txt\n",
			want:  []string{"/home/user/a.txt", "/home/user/b.txt"},
		},
		{
			name:  "trailing empty lines",
			input: "file:///home/user/a.txt\r\n\r\n\r\n",
			want:  []string{"/home/user/a.txt"},
		},
		{
			name:  "path with special chars",
			input: "file:///home/user/file%23name.txt\r\n",
			want:  []string{"/home/user/file#name.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseURIList(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("parseURIList() returned %d paths, want %d\ngot:  %v\nwant: %v", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseURIList()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
