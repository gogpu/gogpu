//go:build linux

package platform

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gogpu/gpucontext"
)

func TestDetectDarkMode(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		want    bool
	}{
		{
			name:    "no env vars",
			envVars: map[string]string{},
			want:    false,
		},
		{
			name:    "GTK_THEME with dark suffix",
			envVars: map[string]string{"GTK_THEME": "Adwaita:dark"},
			want:    true,
		},
		{
			name:    "GTK_THEME with Dark in name",
			envVars: map[string]string{"GTK_THEME": "Yaru-dark"},
			want:    true,
		},
		{
			name:    "GTK_THEME light theme",
			envVars: map[string]string{"GTK_THEME": "Adwaita"},
			want:    false,
		},
		{
			name:    "GTK_THEME case insensitive",
			envVars: map[string]string{"GTK_THEME": "Adwaita:Dark"},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore env vars
			savedGTK := os.Getenv("GTK_THEME")
			savedDesktop := os.Getenv("XDG_CURRENT_DESKTOP")
			defer func() {
				os.Setenv("GTK_THEME", savedGTK)
				os.Setenv("XDG_CURRENT_DESKTOP", savedDesktop)
			}()

			// Clear all relevant env vars first
			os.Unsetenv("GTK_THEME")
			os.Unsetenv("XDG_CURRENT_DESKTOP")

			// Set test env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			got := detectDarkMode()
			if got != tt.want {
				t.Errorf("detectDarkMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectHighContrast(t *testing.T) {
	tests := []struct {
		name     string
		gtkTheme string
		want     bool
	}{
		{"no theme", "", false},
		{"normal theme", "Adwaita", false},
		{"high contrast", "HighContrast", true},
		{"high contrast dark", "HighContrastInverse", true},
		{"high-contrast hyphenated", "High-Contrast", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saved := os.Getenv("GTK_THEME")
			defer os.Setenv("GTK_THEME", saved)

			if tt.gtkTheme == "" {
				os.Unsetenv("GTK_THEME")
			} else {
				os.Setenv("GTK_THEME", tt.gtkTheme)
			}

			got := detectHighContrast()
			if got != tt.want {
				t.Errorf("detectHighContrast() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectFontScale(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		want    float32
	}{
		{
			name:    "no env vars",
			envVars: map[string]string{},
			want:    1.0,
		},
		{
			name:    "GDK_DPI_SCALE set",
			envVars: map[string]string{"GDK_DPI_SCALE": "1.5"},
			want:    1.5,
		},
		{
			name:    "GDK_DPI_SCALE invalid",
			envVars: map[string]string{"GDK_DPI_SCALE": "abc"},
			want:    1.0,
		},
		{
			name:    "GDK_DPI_SCALE zero",
			envVars: map[string]string{"GDK_DPI_SCALE": "0"},
			want:    1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saved := os.Getenv("GDK_DPI_SCALE")
			defer os.Setenv("GDK_DPI_SCALE", saved)

			os.Unsetenv("GDK_DPI_SCALE")
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			got := detectFontScale()
			if got != tt.want {
				t.Errorf("detectFontScale() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectReduceMotion(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		want    bool
	}{
		{
			name:    "no env vars",
			envVars: map[string]string{},
			want:    false,
		},
		{
			name:    "GTK_ENABLE_ANIMATIONS=0",
			envVars: map[string]string{"GTK_ENABLE_ANIMATIONS": "0"},
			want:    true,
		},
		{
			name:    "GTK_ENABLE_ANIMATIONS=1",
			envVars: map[string]string{"GTK_ENABLE_ANIMATIONS": "1"},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saved := os.Getenv("GTK_ENABLE_ANIMATIONS")
			defer os.Setenv("GTK_ENABLE_ANIMATIONS", saved)

			os.Unsetenv("GTK_ENABLE_ANIMATIONS")
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			got := detectReduceMotion()
			if got != tt.want {
				t.Errorf("detectReduceMotion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsKDE(t *testing.T) {
	saved := os.Getenv("XDG_CURRENT_DESKTOP")
	defer os.Setenv("XDG_CURRENT_DESKTOP", saved)

	os.Setenv("XDG_CURRENT_DESKTOP", "KDE")
	if !isKDE() {
		t.Error("isKDE() = false for XDG_CURRENT_DESKTOP=KDE")
	}

	os.Setenv("XDG_CURRENT_DESKTOP", "GNOME")
	if isKDE() {
		t.Error("isKDE() = true for XDG_CURRENT_DESKTOP=GNOME")
	}

	os.Unsetenv("XDG_CURRENT_DESKTOP")
	if isKDE() {
		t.Error("isKDE() = true with no XDG_CURRENT_DESKTOP")
	}
}

func TestIsDarkKDEColorScheme(t *testing.T) {
	// Create a temporary kdeglobals file
	tmpDir := t.TempDir()

	// Test with dark color scheme
	kdeglobals := filepath.Join(tmpDir, "kdeglobals")
	err := os.WriteFile(kdeglobals, []byte("[General]\nColorScheme=BreezeDark\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	savedConfig := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", savedConfig)

	os.Setenv("XDG_CONFIG_HOME", tmpDir)

	if !isDarkKDEColorScheme() {
		t.Error("isDarkKDEColorScheme() = false for BreezeDark")
	}

	// Test with light color scheme
	err = os.WriteFile(kdeglobals, []byte("[General]\nColorScheme=BreezeLight\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	if isDarkKDEColorScheme() {
		t.Error("isDarkKDEColorScheme() = true for BreezeLight")
	}

	// Test with no config file
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "nonexistent"))
	if isDarkKDEColorScheme() {
		t.Error("isDarkKDEColorScheme() = true with no config file")
	}
}

func TestIsHiDPI(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		want    bool
	}{
		{"no env vars", map[string]string{}, false},
		{"GDK_SCALE=1", map[string]string{"GDK_SCALE": "1"}, false},
		{"GDK_SCALE=2", map[string]string{"GDK_SCALE": "2"}, true},
		{"GDK_SCALE=3", map[string]string{"GDK_SCALE": "3"}, true},
		{"QT_SCALE_FACTOR=1.5", map[string]string{"QT_SCALE_FACTOR": "1.5"}, false},
		{"QT_SCALE_FACTOR=2", map[string]string{"QT_SCALE_FACTOR": "2"}, true},
		{"invalid value", map[string]string{"GDK_SCALE": "abc"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			savedGDK := os.Getenv("GDK_SCALE")
			savedQT := os.Getenv("QT_SCALE_FACTOR")
			defer func() {
				os.Setenv("GDK_SCALE", savedGDK)
				os.Setenv("QT_SCALE_FACTOR", savedQT)
			}()

			os.Unsetenv("GDK_SCALE")
			os.Unsetenv("QT_SCALE_FACTOR")
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			got := isHiDPI()
			if got != tt.want {
				t.Errorf("isHiDPI() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseFontconfigRGBA(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantOK     bool
		wantLayout gpucontext.SubpixelLayout
	}{
		{
			name:       "rgb",
			content:    `<match><edit name="rgba" mode="assign"><const>rgb</const></edit></match>`,
			wantOK:     true,
			wantLayout: gpucontext.SubpixelRGB,
		},
		{
			name:       "bgr",
			content:    `<match><edit name="rgba" mode="assign"><const>bgr</const></edit></match>`,
			wantOK:     true,
			wantLayout: gpucontext.SubpixelBGR,
		},
		{
			name:       "vrgb",
			content:    `<match><edit name="rgba" mode="assign"><const>vrgb</const></edit></match>`,
			wantOK:     true,
			wantLayout: gpucontext.SubpixelVRGB,
		},
		{
			name:       "vbgr",
			content:    `<match><edit name="rgba" mode="assign"><const>vbgr</const></edit></match>`,
			wantOK:     true,
			wantLayout: gpucontext.SubpixelVBGR,
		},
		{
			name:       "none",
			content:    `<match><edit name="rgba" mode="assign"><const>none</const></edit></match>`,
			wantOK:     true,
			wantLayout: gpucontext.SubpixelNone,
		},
		{
			name:       "no rgba setting",
			content:    `<match><edit name="antialias"><bool>true</bool></edit></match>`,
			wantOK:     false,
			wantLayout: gpucontext.SubpixelNone,
		},
		{
			name:       "empty file",
			content:    "",
			wantOK:     false,
			wantLayout: gpucontext.SubpixelNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "fonts.conf")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			layout, ok := parseFontconfigRGBA(path)
			if ok != tt.wantOK {
				t.Errorf("parseFontconfigRGBA() ok = %v, want %v", ok, tt.wantOK)
			}
			if layout != tt.wantLayout {
				t.Errorf("parseFontconfigRGBA() layout = %v, want %v", layout, tt.wantLayout)
			}
		})
	}
}

func TestParseFontconfigRGBA_NonexistentFile(t *testing.T) {
	layout, ok := parseFontconfigRGBA("/nonexistent/path/fonts.conf")
	if ok {
		t.Error("parseFontconfigRGBA() ok = true for nonexistent file")
	}
	if layout != gpucontext.SubpixelNone {
		t.Errorf("parseFontconfigRGBA() = %v, want SubpixelNone", layout)
	}
}

func TestParseFontconfigRGBA_EmptyPath(t *testing.T) {
	layout, ok := parseFontconfigRGBA("")
	if ok {
		t.Error("parseFontconfigRGBA() ok = true for empty path")
	}
	if layout != gpucontext.SubpixelNone {
		t.Errorf("parseFontconfigRGBA() = %v, want SubpixelNone", layout)
	}
}

func TestDetectSubpixelLayout(t *testing.T) {
	// Save and restore env vars
	savedGDK := os.Getenv("GDK_SCALE")
	savedQT := os.Getenv("QT_SCALE_FACTOR")
	savedConfig := os.Getenv("XDG_CONFIG_HOME")
	savedHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("GDK_SCALE", savedGDK)
		os.Setenv("QT_SCALE_FACTOR", savedQT)
		os.Setenv("XDG_CONFIG_HOME", savedConfig)
		os.Setenv("HOME", savedHome)
	}()

	// Test 1: HiDPI returns SubpixelNone
	os.Setenv("GDK_SCALE", "2")
	os.Setenv("XDG_CONFIG_HOME", "/nonexistent")
	os.Unsetenv("HOME")
	got := detectSubpixelLayout()
	if got != gpucontext.SubpixelNone {
		t.Errorf("detectSubpixelLayout() with HiDPI = %v, want SubpixelNone", got)
	}

	// Test 2: Non-HiDPI with fontconfig
	os.Unsetenv("GDK_SCALE")
	os.Unsetenv("QT_SCALE_FACTOR")
	tmpDir := t.TempDir()
	fontconfigDir := filepath.Join(tmpDir, "fontconfig")
	if err := os.MkdirAll(fontconfigDir, 0755); err != nil {
		t.Fatal(err)
	}
	fontconfigFile := filepath.Join(fontconfigDir, "fonts.conf")
	if err := os.WriteFile(fontconfigFile, []byte(
		`<match><edit name="rgba" mode="assign"><const>bgr</const></edit></match>`,
	), 0644); err != nil {
		t.Fatal(err)
	}
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	got = detectSubpixelLayout()
	if got != gpucontext.SubpixelBGR {
		t.Errorf("detectSubpixelLayout() with fontconfig bgr = %v, want SubpixelBGR", got)
	}

	// Test 3: No fontconfig, no HiDPI → default RGB
	os.Setenv("XDG_CONFIG_HOME", "/nonexistent")
	os.Setenv("HOME", "/nonexistent")
	got = detectSubpixelLayout()
	if got != gpucontext.SubpixelRGB {
		t.Errorf("detectSubpixelLayout() default = %v, want SubpixelRGB", got)
	}
}
