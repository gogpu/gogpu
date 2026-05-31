//go:build linux

package platform

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
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
			if !reflect.DeepEqual(got, tt.want) {
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
			if !reflect.DeepEqual(got, tt.want) {
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
			if !reflect.DeepEqual(got, tt.want) {
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
			if !reflect.DeepEqual(got, tt.want) {
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
			if !reflect.DeepEqual(got, tt.want) {
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
			if !reflect.DeepEqual(got, tt.want) {
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
	if !reflect.DeepEqual(paths, want) {
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
	if !reflect.DeepEqual(paths, want) {
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
	if !reflect.DeepEqual(paths, want) {
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

// --- D-Bus encoding tests ---

func TestDBusHandlePath(t *testing.T) {
	tests := []struct {
		sender string
		token  string
		want   string
	}{
		{
			sender: ":1.42",
			token:  "gogpu_1",
			want:   "/org/freedesktop/portal/desktop/request/1_42/gogpu_1",
		},
		{
			sender: ":1.123",
			token:  "gogpu_2",
			want:   "/org/freedesktop/portal/desktop/request/1_123/gogpu_2",
		},
		{
			sender: ":2.0",
			token:  "mytoken",
			want:   "/org/freedesktop/portal/desktop/request/2_0/mytoken",
		},
	}

	for _, tt := range tests {
		got := dbusHandlePath(tt.sender, tt.token)
		if got != tt.want {
			t.Errorf("dbusHandlePath(%q, %q) = %q, want %q", tt.sender, tt.token, got, tt.want)
		}
	}
}

func TestDBusParseParams(t *testing.T) {
	tests := []struct {
		input string
		want  map[string]string
	}{
		{
			input: "path=/run/user/1000/bus",
			want:  map[string]string{"path": "/run/user/1000/bus"},
		},
		{
			input: "abstract=/tmp/dbus-abc,guid=1234",
			want:  map[string]string{"abstract": "/tmp/dbus-abc", "guid": "1234"},
		},
		{
			input: "",
			want:  map[string]string{},
		},
	}

	for _, tt := range tests {
		got := dbusParseParams(tt.input)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("dbusParseParams(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestDBusURIToPath(t *testing.T) {
	tests := []struct {
		uri    string
		want   string
		wantOK bool
	}{
		{"file:///home/user/file.txt", "/home/user/file.txt", true},
		{"file:///tmp/my%20file.txt", "/tmp/my file.txt", true},
		{"file:///path/to/%C3%A9t%C3%A9.txt", "/path/to/été.txt", true},
		{"https://example.com/file.txt", "", false},
		{"file://", "", true},
	}

	for _, tt := range tests {
		got, ok := dbusURIToPath(tt.uri)
		if ok != tt.wantOK {
			t.Errorf("dbusURIToPath(%q) ok = %v, want %v", tt.uri, ok, tt.wantOK)
			continue
		}
		if ok && got != tt.want {
			t.Errorf("dbusURIToPath(%q) = %q, want %q", tt.uri, got, tt.want)
		}
	}
}

func TestMsgBufAlignment(t *testing.T) {
	b := newMsgBuf(0)

	// Write byte at 0 → pos 1
	b.u8(0xAA)
	if b.pos != 1 {
		t.Fatalf("pos after u8: got %d, want 1", b.pos)
	}

	// Write u32 at pos 1 → align to 4, pad 3 bytes → write at pos 4
	b.u32(0xDEADBEEF)
	if b.pos != 8 {
		t.Fatalf("pos after u32: got %d, want 8", b.pos)
	}
	if b.data[1] != 0 || b.data[2] != 0 || b.data[3] != 0 {
		t.Error("expected 3 padding zero bytes before u32")
	}
	if binary.LittleEndian.Uint32(b.data[4:8]) != 0xDEADBEEF {
		t.Error("u32 value not written correctly")
	}
}

func TestMsgBufStr(t *testing.T) {
	b := newMsgBuf(0)
	b.str("hello")

	// u32(5) = 4 bytes + "hello"(5) + NUL(1) = 10 bytes
	if len(b.data) != 10 {
		t.Fatalf("str encoding length: got %d, want 10", len(b.data))
	}
	if binary.LittleEndian.Uint32(b.data[0:4]) != 5 {
		t.Error("str: length field incorrect")
	}
	if string(b.data[4:9]) != "hello" {
		t.Error("str: content incorrect")
	}
	if b.data[9] != 0 {
		t.Error("str: missing NUL terminator")
	}
}

func TestMsgBufSig(t *testing.T) {
	b := newMsgBuf(0)
	b.sig("ssa{sv}")

	// u8(7) + "ssa{sv}"(7) + NUL(1) = 9 bytes
	if len(b.data) != 9 {
		t.Fatalf("sig encoding length: got %d, want 9", len(b.data))
	}
	if b.data[0] != 7 {
		t.Error("sig: length byte incorrect")
	}
	if string(b.data[1:8]) != "ssa{sv}" {
		t.Error("sig: content incorrect")
	}
	if b.data[8] != 0 {
		t.Error("sig: missing NUL terminator")
	}
}

func TestMsgBufArrayRoundTrip(t *testing.T) {
	b := newMsgBuf(0)
	lp, cp := b.arrayStart(4) // array of u32
	b.u32(1)
	b.u32(2)
	b.arrayEnd(lp, cp)

	// Decode: u32 length = 8, then u32(1), u32(2)
	d := newMsgDecoder(b.data, 0)
	n, err := d.readU32()
	if err != nil {
		t.Fatal(err)
	}
	if n != 8 {
		t.Errorf("array length: got %d, want 8", n)
	}
	v1, _ := d.readU32()
	v2, _ := d.readU32()
	if v1 != 1 || v2 != 2 {
		t.Errorf("array values: got %d %d, want 1 2", v1, v2)
	}
}

// TestDecodePortalResponse_Cancel verifies response code 1 returns nil, nil.
func TestDecodePortalResponse_Cancel(t *testing.T) {
	// Build body: u32(1) + empty a{sv}
	b := newMsgBuf(0)
	b.u32(1) // user canceled
	lp, cp := b.arrayStart(8)
	b.arrayEnd(lp, cp)

	paths, err := decodePortalResponse(b.data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if paths != nil {
		t.Errorf("expected nil on cancel, got %v", paths)
	}
}

// TestDecodePortalResponse_Success verifies URI extraction from a success response.
func TestDecodePortalResponse_Success(t *testing.T) {
	// Build body: u32(0) + a{sv} with "uris" -> v(as) [file:///tmp/test.txt]
	b := newMsgBuf(0)
	b.u32(0) // success

	// a{sv}
	lp, cp := b.arrayStart(8)

	// dict entry: "uris" -> v(as)
	b.padTo(8)
	b.str("uris")
	b.sig("as") // variant signature
	// a(s): array of strings
	ilp, icp := b.arrayStart(4) // strings are 4-byte aligned
	b.str("file:///tmp/test.txt")
	b.arrayEnd(ilp, icp)

	b.arrayEnd(lp, cp)

	paths, err := decodePortalResponse(b.data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"/tmp/test.txt"}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("decodePortalResponse() = %v, want %v", paths, want)
	}
}

// TestDecodePortalResponse_MultipleURIs verifies multiple URI extraction.
func TestDecodePortalResponse_MultipleURIs(t *testing.T) {
	b := newMsgBuf(0)
	b.u32(0)

	lp, cp := b.arrayStart(8)

	b.padTo(8)
	b.str("uris")
	b.sig("as")
	ilp, icp := b.arrayStart(4)
	b.str("file:///home/user/a.png")
	b.str("file:///home/user/b.png")
	b.arrayEnd(ilp, icp)

	b.arrayEnd(lp, cp)

	paths, err := decodePortalResponse(b.data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"/home/user/a.png", "/home/user/b.png"}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("decodePortalResponse() = %v, want %v", paths, want)
	}
}

// TestDecodePortalResponse_SkipsUnknownKeys verifies unknown dict entries are skipped.
func TestDecodePortalResponse_SkipsUnknownKeys(t *testing.T) {
	b := newMsgBuf(0)
	b.u32(0)

	lp, cp := b.arrayStart(8)

	// unknown entry "writable" -> v(b) true
	b.padTo(8)
	b.str("writable")
	b.variantBool(true)

	// "uris" -> v(as)
	b.padTo(8)
	b.str("uris")
	b.sig("as")
	ilp, icp := b.arrayStart(4)
	b.str("file:///tmp/result.txt")
	b.arrayEnd(ilp, icp)

	b.arrayEnd(lp, cp)

	paths, err := decodePortalResponse(b.data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"/tmp/result.txt"}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("decodePortalResponse() = %v, want %v", paths, want)
	}
}

// TestDecodePortalResponse_ErrorCode verifies non-zero non-cancel codes return error.
func TestDecodePortalResponse_ErrorCode(t *testing.T) {
	b := newMsgBuf(0)
	b.u32(2) // error code
	lp, cp := b.arrayStart(8)
	b.arrayEnd(lp, cp)

	_, err := decodePortalResponse(b.data)
	if err == nil {
		t.Error("expected error for response code 2")
	}
}

// TestEncodeFileChooserBody verifies basic structure of the encoded portal body.
func TestEncodeFileChooserBody(t *testing.T) {
	body := encodeFileChooserBody("", "Open File", FileDialogOptions{
		Multiple: true,
		Filters:  []FileTypeFilter{{Name: "Images", Extensions: []string{"png", "jpg"}}},
	}, "gogpu_1", false)

	d := newMsgDecoder(body, 0)

	// parent_window
	pw, err := d.readStr()
	if err != nil || pw != "" {
		t.Errorf("parent_window = %q, want %q", pw, "")
	}

	// title
	title, err := d.readStr()
	if err != nil || title != "Open File" {
		t.Errorf("title = %q, want %q", title, "Open File")
	}

	// options a{sv} length (just verify it's non-zero)
	n, err := d.readU32()
	if err != nil || n == 0 {
		t.Errorf("options dict length = %d, want > 0 (err=%v)", n, err)
	}
}

// TestEncodePortalFilters verifies filter array encoding round-trips through the decoder.
func TestEncodePortalFilters(t *testing.T) {
	b := newMsgBuf(0)
	filters := []FileTypeFilter{
		{Name: "Images", Extensions: []string{"png", "jpg"}},
		{Name: "Docs", Extensions: []string{"pdf"}},
	}
	encodePortalFilters(b, filters)

	// Verify it decodes without error (structural smoke test).
	d := newMsgDecoder(b.data, 0)
	outerLen, err := d.readU32()
	if err != nil {
		t.Fatalf("read outer array len: %v", err)
	}
	if outerLen == 0 {
		t.Fatal("expected non-zero filter array length")
	}
}

// TestDBusEncodeMsg verifies the message fixed header fields.
func TestDBusEncodeMsg(t *testing.T) {
	msg := dbusEncodeMsg(
		dbusMsgCall, 7,
		"org.freedesktop.portal.Desktop",
		"/org/freedesktop/portal/desktop",
		"org.freedesktop.portal.FileChooser",
		"OpenFile",
		"ssa{sv}",
		[]byte{0x01, 0x02, 0x03, 0x04},
	)

	if len(msg) < 16 {
		t.Fatalf("message too short: %d bytes", len(msg))
	}
	if msg[0] != 'l' {
		t.Errorf("endian: got %c, want 'l'", msg[0])
	}
	if msg[1] != dbusMsgCall {
		t.Errorf("type: got %d, want %d", msg[1], dbusMsgCall)
	}
	if msg[3] != 1 {
		t.Errorf("protocol version: got %d, want 1", msg[3])
	}
	bodyLen := binary.LittleEndian.Uint32(msg[4:8])
	if bodyLen != 4 {
		t.Errorf("body length: got %d, want 4", bodyLen)
	}
	serial := binary.LittleEndian.Uint32(msg[8:12])
	if serial != 7 {
		t.Errorf("serial: got %d, want 7", serial)
	}

	// Total length must be 8-byte aligned (header + padding + body)
	hdrArrayLen := binary.LittleEndian.Uint32(msg[12:16])
	headerTotal := 16 + int(hdrArrayLen)
	padLen := (8 - headerTotal%8) % 8
	expected := headerTotal + padLen + 4
	if len(msg) != expected {
		t.Errorf("total length: got %d, want %d", len(msg), expected)
	}
}

// TestMsgDecoderSkipValue verifies skipValue handles common portal response types.
func TestMsgDecoderSkipValue(t *testing.T) {
	tests := []struct {
		name   string
		sig    string
		encode func(*msgBuf)
	}{
		{
			name:   "bool",
			sig:    "b",
			encode: func(b *msgBuf) { b.bool32(true) },
		},
		{
			name:   "uint32",
			sig:    "u",
			encode: func(b *msgBuf) { b.u32(42) },
		},
		{
			name:   "string",
			sig:    "s",
			encode: func(b *msgBuf) { b.str("hello world") },
		},
		{
			name:   "variant bool",
			sig:    "v",
			encode: func(b *msgBuf) { b.variantBool(false) },
		},
		{
			name: "array of strings",
			sig:  "as",
			encode: func(b *msgBuf) {
				lp, cp := b.arrayStart(4)
				b.str("one")
				b.str("two")
				b.arrayEnd(lp, cp)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := newMsgBuf(0)
			tt.encode(enc)

			// Append sentinel byte after the encoded value.
			enc.u8(0xFF)
			sentinelPos := len(enc.data) - 1

			d := newMsgDecoder(enc.data, 0)
			if err := d.skipValue(tt.sig); err != nil {
				t.Fatalf("skipValue(%q): %v", tt.sig, err)
			}
			// After skip, cursor must land exactly on the sentinel byte —
			// neither overrun (reads past it) nor underrun (stops early).
			if d.pos != sentinelPos {
				t.Errorf("skipValue(%q): pos=%d, want %d (sentinel)", tt.sig, d.pos, sentinelPos)
			}
		})
	}
}
