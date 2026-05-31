//go:build linux

package platform

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// --- zenity arg construction ---

func TestZenityOpenArgs(t *testing.T) {
	tests := []struct {
		name string
		opts FileDialogOptions
		want []string
	}{
		{
			name: "basic open",
			opts: FileDialogOptions{},
			want: []string{"--file-selection"},
		},
		{
			name: "with title",
			opts: FileDialogOptions{Title: "Open File"},
			want: []string{"--file-selection", "--title=Open File"},
		},
		{
			name: "directory mode",
			opts: FileDialogOptions{Directory: true},
			want: []string{"--file-selection", "--directory"},
		},
		{
			name: "multiple selection",
			opts: FileDialogOptions{Multiple: true},
			want: []string{"--file-selection", "--multiple", "--separator=\n"},
		},
		{
			name: "initial directory",
			opts: FileDialogOptions{InitialDirectory: "/home/user"},
			want: []string{"--file-selection", "--filename=/home/user/"},
		},
		{
			name: "with filters",
			opts: FileDialogOptions{
				Filters: []FileTypeFilter{{Name: "Images", Extensions: []string{"png", "jpg"}}},
			},
			want: []string{"--file-selection", "--file-filter=Images | *.png *.jpg"},
		},
		{
			name: "filter with glob prefix stripped",
			opts: FileDialogOptions{
				Filters: []FileTypeFilter{{Name: "Go", Extensions: []string{"*.go"}}},
			},
			want: []string{"--file-selection", "--file-filter=Go | *.go"},
		},
		{
			name: "filter with dot prefix stripped",
			opts: FileDialogOptions{
				Filters: []FileTypeFilter{{Name: "Docs", Extensions: []string{".pdf", ".docx"}}},
			},
			want: []string{"--file-selection", "--file-filter=Docs | *.pdf *.docx"},
		},
		{
			name: "filter with blank extensions omitted",
			opts: FileDialogOptions{
				Filters: []FileTypeFilter{{Name: "Empty", Extensions: []string{"", "*.", "."}}},
			},
			want: []string{"--file-selection"}, // empty filter → not added
		},
		{
			name: "multiple + title + filter",
			opts: FileDialogOptions{
				Title:    "Pick Files",
				Multiple: true,
				Filters:  []FileTypeFilter{{Name: "Images", Extensions: []string{"png"}}},
			},
			want: []string{
				"--file-selection",
				"--multiple", "--separator=\n",
				"--title=Pick Files",
				"--file-filter=Images | *.png",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := zenityOpenArgs(tt.opts)
			if !slices.Equal(got, tt.want) {
				t.Errorf("zenityOpenArgs() = %v\nwant %v", got, tt.want)
			}
		})
	}
}

func TestZenitySaveArgs(t *testing.T) {
	tests := []struct {
		name string
		opts FileDialogOptions
		want []string
	}{
		{
			name: "basic save",
			opts: FileDialogOptions{},
			want: []string{"--file-selection", "--save", "--confirm-overwrite"},
		},
		{
			name: "with title",
			opts: FileDialogOptions{Title: "Save As"},
			want: []string{"--file-selection", "--save", "--confirm-overwrite", "--title=Save As"},
		},
		{
			name: "default filename only",
			opts: FileDialogOptions{DefaultFilename: "output.png"},
			want: []string{"--file-selection", "--save", "--confirm-overwrite", "--filename=output.png"},
		},
		{
			name: "initial dir + default filename",
			opts: FileDialogOptions{InitialDirectory: "/tmp", DefaultFilename: "out.txt"},
			want: []string{"--file-selection", "--save", "--confirm-overwrite", "--filename=/tmp/out.txt"},
		},
		{
			name: "initial dir without filename",
			opts: FileDialogOptions{InitialDirectory: "/tmp"},
			want: []string{"--file-selection", "--save", "--confirm-overwrite", "--filename=/tmp/"},
		},
		{
			name: "with filter",
			opts: FileDialogOptions{
				Filters: []FileTypeFilter{{Name: "Text", Extensions: []string{"txt"}}},
			},
			want: []string{"--file-selection", "--save", "--confirm-overwrite", "--file-filter=Text | *.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := zenitySaveArgs(tt.opts)
			if !slices.Equal(got, tt.want) {
				t.Errorf("zenitySaveArgs() = %v\nwant %v", got, tt.want)
			}
		})
	}
}

func TestZenityFilterSpec(t *testing.T) {
	tests := []struct {
		name string
		f    FileTypeFilter
		want string
	}{
		{
			name: "basic",
			f:    FileTypeFilter{Name: "Images", Extensions: []string{"png", "jpg"}},
			want: "Images | *.png *.jpg",
		},
		{
			name: "glob prefix stripped",
			f:    FileTypeFilter{Name: "Go", Extensions: []string{"*.go"}},
			want: "Go | *.go",
		},
		{
			name: "dot prefix stripped",
			f:    FileTypeFilter{Name: "PDF", Extensions: []string{".pdf"}},
			want: "PDF | *.pdf",
		},
		{
			name: "empty extensions returns empty string",
			f:    FileTypeFilter{Name: "None", Extensions: nil},
			want: "",
		},
		{
			name: "all blank extensions returns empty string",
			f:    FileTypeFilter{Name: "Bad", Extensions: []string{"", "*.", "."}},
			want: "",
		},
		{
			name: "single extension",
			f:    FileTypeFilter{Name: "Rust", Extensions: []string{"rs"}},
			want: "Rust | *.rs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := zenityFilterSpec(tt.f)
			if got != tt.want {
				t.Errorf("zenityFilterSpec() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- kdialog arg construction ---

func TestKdialogOpenArgs(t *testing.T) {
	tests := []struct {
		name string
		opts FileDialogOptions
		want []string
	}{
		{
			name: "single file",
			opts: FileDialogOptions{},
			want: []string{"--getopenfilename", "."},
		},
		{
			name: "multiple files",
			opts: FileDialogOptions{Multiple: true},
			want: []string{"--getopenfilenames", "."},
		},
		{
			name: "directory",
			opts: FileDialogOptions{Directory: true},
			want: []string{"--getexistingdirectory", "."},
		},
		{
			name: "with initial dir",
			opts: FileDialogOptions{InitialDirectory: "/home/user"},
			want: []string{"--getopenfilename", "/home/user"},
		},
		{
			name: "with title",
			opts: FileDialogOptions{Title: "Open"},
			want: []string{"--getopenfilename", ".", "--title", "Open"},
		},
		{
			name: "with filter",
			opts: FileDialogOptions{
				Filters: []FileTypeFilter{{Name: "Images", Extensions: []string{"png", "jpg"}}},
			},
			want: []string{"--getopenfilename", ".", "Images (*.png *.jpg)"},
		},
		{
			name: "directory mode ignores filters",
			opts: FileDialogOptions{
				Directory: true,
				Filters:   []FileTypeFilter{{Name: "Ignored", Extensions: []string{"txt"}}},
			},
			want: []string{"--getexistingdirectory", "."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kdialogOpenArgs(tt.opts)
			if !slices.Equal(got, tt.want) {
				t.Errorf("kdialogOpenArgs() = %v\nwant %v", got, tt.want)
			}
		})
	}
}

func TestKdialogSaveArgs(t *testing.T) {
	tests := []struct {
		name string
		opts FileDialogOptions
		want []string
	}{
		{
			name: "basic",
			opts: FileDialogOptions{},
			want: []string{"--getsavefilename", "."},
		},
		{
			name: "with default filename",
			opts: FileDialogOptions{DefaultFilename: "out.txt"},
			want: []string{"--getsavefilename", "./out.txt"},
		},
		{
			name: "with initial dir and filename",
			opts: FileDialogOptions{InitialDirectory: "/tmp", DefaultFilename: "out.txt"},
			want: []string{"--getsavefilename", "/tmp/out.txt"},
		},
		{
			name: "with filter",
			opts: FileDialogOptions{
				Filters: []FileTypeFilter{{Name: "Text", Extensions: []string{"txt"}}},
			},
			want: []string{"--getsavefilename", ".", "Text (*.txt)"},
		},
		{
			name: "with title",
			opts: FileDialogOptions{Title: "Save As"},
			want: []string{"--getsavefilename", ".", "--title", "Save As"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kdialogSaveArgs(tt.opts)
			if !slices.Equal(got, tt.want) {
				t.Errorf("kdialogSaveArgs() = %v\nwant %v", got, tt.want)
			}
		})
	}
}

func TestKdialogFilterSpec(t *testing.T) {
	tests := []struct {
		name    string
		filters []FileTypeFilter
		want    string
	}{
		{
			name:    "nil filters returns empty",
			filters: nil,
			want:    "",
		},
		{
			name:    "empty filters returns empty",
			filters: []FileTypeFilter{},
			want:    "",
		},
		{
			name: "single filter",
			filters: []FileTypeFilter{
				{Name: "Images", Extensions: []string{"png", "jpg"}},
			},
			want: "Images (*.png *.jpg)",
		},
		{
			name: "multiple filters joined with ;;",
			filters: []FileTypeFilter{
				{Name: "Images", Extensions: []string{"png"}},
				{Name: "Docs", Extensions: []string{"pdf"}},
			},
			want: "Images (*.png);;Docs (*.pdf)",
		},
		{
			name: "glob prefix stripped",
			filters: []FileTypeFilter{
				{Name: "Go", Extensions: []string{"*.go"}},
			},
			want: "Go (*.go)",
		},
		{
			name: "dot prefix stripped",
			filters: []FileTypeFilter{
				{Name: "Rust", Extensions: []string{".rs"}},
			},
			want: "Rust (*.rs)",
		},
		{
			name: "filter with all blank extensions skipped",
			filters: []FileTypeFilter{
				{Name: "Bad", Extensions: []string{"", "*.", "."}},
				{Name: "Good", Extensions: []string{"txt"}},
			},
			want: "Good (*.txt)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kdialogFilterSpec(tt.filters)
			if got != tt.want {
				t.Errorf("kdialogFilterSpec() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- output parsing ---

func TestSplitNewlines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty string", "", nil},
		{"single path", "/home/user/file.txt", []string{"/home/user/file.txt"}},
		{"two paths", "/a/b\n/c/d", []string{"/a/b", "/c/d"}},
		{"trailing newline stripped by caller", "/a\n/b", []string{"/a", "/b"}},
		{"ignores empty lines", "/a\n\n/b", []string{"/a", "/b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitNewlines(tt.input)
			if !slices.Equal(got, tt.want) {
				t.Errorf("splitNewlines(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestKdialogParsePaths(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		multiple bool
		want     []string
	}{
		{"empty single", "", false, nil},
		{"empty multiple", "", true, nil},
		{"single path", "/home/user/file.txt", false, []string{"/home/user/file.txt"}},
		{
			name:     "multiple space-separated",
			raw:      "/a/b.txt /c/d.txt",
			multiple: true,
			want:     []string{"/a/b.txt", "/c/d.txt"},
		},
		{
			name:     "single path not split when multiple=false",
			raw:      "/a/b c/d.txt",
			multiple: false,
			want:     []string{"/a/b c/d.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kdialogParsePaths(tt.raw, tt.multiple)
			if !slices.Equal(got, tt.want) {
				t.Errorf("kdialogParsePaths(%q, %v) = %v, want %v", tt.raw, tt.multiple, got, tt.want)
			}
		})
	}
}

// --- fake binary tests ---

// writeFakeBinary creates an executable shell script in dir named name.
func writeFakeBinary(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, name)
	content := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(p, []byte(content), 0o755); err != nil {
		t.Fatalf("writeFakeBinary: %v", err)
	}
}

func TestZenityOpen_FakeBinary_OK(t *testing.T) {
	dir := t.TempDir()
	writeFakeBinary(t, dir, "zenity", `echo "/home/user/photo.png"`)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	paths, err := zenityOpen(FileDialogOptions{Title: "Open"})
	if err != nil {
		t.Fatalf("zenityOpen error: %v", err)
	}
	want := []string{"/home/user/photo.png"}
	if !slices.Equal(paths, want) {
		t.Errorf("zenityOpen() = %v, want %v", paths, want)
	}
}

func TestZenityOpen_FakeBinary_Cancel(t *testing.T) {
	dir := t.TempDir()
	writeFakeBinary(t, dir, "zenity", `exit 1`)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	paths, err := zenityOpen(FileDialogOptions{})
	if err != nil {
		t.Fatalf("unexpected error on cancel: %v", err)
	}
	if paths != nil {
		t.Errorf("expected nil paths on cancel, got %v", paths)
	}
}

func TestZenityOpen_FakeBinary_Multiple(t *testing.T) {
	dir := t.TempDir()
	writeFakeBinary(t, dir, "zenity", `printf "/a/one.txt\n/b/two.txt"`)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	paths, err := zenityOpen(FileDialogOptions{Multiple: true})
	if err != nil {
		t.Fatalf("zenityOpen error: %v", err)
	}
	want := []string{"/a/one.txt", "/b/two.txt"}
	if !slices.Equal(paths, want) {
		t.Errorf("zenityOpen() = %v, want %v", paths, want)
	}
}

func TestZenitySave_FakeBinary_OK(t *testing.T) {
	dir := t.TempDir()
	writeFakeBinary(t, dir, "zenity", `echo "/home/user/output.txt"`)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	path, err := zenitySave(FileDialogOptions{Title: "Save"})
	if err != nil {
		t.Fatalf("zenitySave error: %v", err)
	}
	if path != "/home/user/output.txt" {
		t.Errorf("zenitySave() = %q, want %q", path, "/home/user/output.txt")
	}
}

func TestZenitySave_FakeBinary_Cancel(t *testing.T) {
	dir := t.TempDir()
	writeFakeBinary(t, dir, "zenity", `exit 1`)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	path, err := zenitySave(FileDialogOptions{})
	if err != nil {
		t.Fatalf("unexpected error on cancel: %v", err)
	}
	if path != "" {
		t.Errorf("expected empty path on cancel, got %q", path)
	}
}

func TestKdialogOpen_FakeBinary_OK(t *testing.T) {
	dir := t.TempDir()
	writeFakeBinary(t, dir, "kdialog", `echo "/home/user/doc.pdf"`)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	paths, err := kdialogOpen(FileDialogOptions{})
	if err != nil {
		t.Fatalf("kdialogOpen error: %v", err)
	}
	want := []string{"/home/user/doc.pdf"}
	if !slices.Equal(paths, want) {
		t.Errorf("kdialogOpen() = %v, want %v", paths, want)
	}
}

func TestKdialogOpen_FakeBinary_Cancel(t *testing.T) {
	dir := t.TempDir()
	writeFakeBinary(t, dir, "kdialog", `exit 1`)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	paths, err := kdialogOpen(FileDialogOptions{})
	if err != nil {
		t.Fatalf("unexpected error on cancel: %v", err)
	}
	if paths != nil {
		t.Errorf("expected nil on cancel, got %v", paths)
	}
}

func TestSubprocOpenFile_NoToolError(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty dir: no zenity, no kdialog

	_, err := subprocOpenFile(FileDialogOptions{})
	if err == nil {
		t.Error("expected error when no dialog tool available")
	}
}

func TestSubprocSaveFile_NoToolError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := subprocSaveFile(FileDialogOptions{})
	if err == nil {
		t.Error("expected error when no dialog tool available")
	}
}
