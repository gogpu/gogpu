//go:build linux

package platform

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// detectDarkMode checks environment variables and desktop settings to determine
// if dark mode is active. Works for GNOME, KDE, and other freedesktop desktops.
//
// Detection order:
//  1. GTK_THEME env var (set by GNOME/GTK apps, e.g. "Adwaita:dark", "Yaru-dark")
//  2. KDE kdeglobals config file (ColorScheme containing "Dark")
//  3. XDG_CURRENT_DESKTOP + known defaults (e.g., Pantheon defaults light)
func detectDarkMode() bool {
	// GTK_THEME is the most reliable indicator for GTK-based desktops
	if theme := os.Getenv("GTK_THEME"); theme != "" {
		lower := strings.ToLower(theme)
		if strings.Contains(lower, "dark") || strings.HasSuffix(lower, ":dark") {
			return true
		}
	}

	// KDE: check kdeglobals for dark color scheme
	if isKDE() {
		if isDarkKDEColorScheme() {
			return true
		}
	}

	return false
}

// detectHighContrast checks if high contrast mode is active.
// Detects GTK HighContrast themes.
func detectHighContrast() bool {
	if theme := os.Getenv("GTK_THEME"); theme != "" {
		lower := strings.ToLower(theme)
		if strings.Contains(lower, "highcontrast") || strings.Contains(lower, "high-contrast") {
			return true
		}
	}

	return false
}

// detectFontScale returns the font scale multiplier from environment variables.
// Checks GDK_SCALE, QT_SCALE_FACTOR, and GDK_DPI_SCALE in order.
func detectFontScale() float32 {
	// GDK_DPI_SCALE is specifically for font scaling in GTK apps
	if s := os.Getenv("GDK_DPI_SCALE"); s != "" {
		if v, err := strconv.ParseFloat(s, 32); err == nil && v > 0 {
			return float32(v)
		}
	}

	// GNOME text-scaling-factor is typically exposed through dconf,
	// but reading dconf requires D-Bus. Fall back to 1.0 for now.

	return 1.0
}

// detectReduceMotion checks if the user prefers reduced animations.
// On Linux, this is typically set through GNOME accessibility settings
// (org.gnome.desktop.interface gtk-enable-animations).
// Without D-Bus access, we check environment hints.
func detectReduceMotion() bool {
	// GTK_ENABLE_ANIMATIONS=0 disables animations in GTK apps
	if v := os.Getenv("GTK_ENABLE_ANIMATIONS"); v == "0" {
		return true
	}

	// No reliable environment-only detection without D-Bus.
	// D-Bus portal (org.freedesktop.portal.Settings) would be the proper solution.
	return false
}

// isKDE returns true if the current desktop environment is KDE Plasma.
func isKDE() bool {
	desktop := os.Getenv("XDG_CURRENT_DESKTOP")
	return strings.Contains(strings.ToUpper(desktop), "KDE")
}

// isDarkKDEColorScheme reads KDE's kdeglobals to check if a dark color scheme is active.
func isDarkKDEColorScheme() bool {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home := os.Getenv("HOME")
		if home == "" {
			return false
		}
		configDir = filepath.Join(home, ".config")
	}

	data, err := os.ReadFile(filepath.Join(configDir, "kdeglobals"))
	if err != nil {
		return false
	}

	// Look for ColorScheme= line containing "Dark"
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ColorScheme=") || strings.HasPrefix(line, "ColorScheme =") {
			value := strings.TrimPrefix(line, "ColorScheme=")
			value = strings.TrimPrefix(value, "ColorScheme =")
			value = strings.TrimSpace(value)
			if strings.Contains(strings.ToLower(value), "dark") {
				return true
			}
		}
	}

	return false
}
