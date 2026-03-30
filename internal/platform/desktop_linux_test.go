//go:build linux

package platform

import (
	"os"
	"path/filepath"
	"testing"
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
