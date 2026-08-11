package main

import (
	"fmt"
	"os"

	"github.com/gogpu/gg/text"
)

// adapted from https://github.com/gogpu/gg/blob/main/examples/gogpu_integration/main.go

// loadFontSource finds a system font and returns the font source.
func loadFontSource() (*text.FontSource, error) {
	fontPath := findSystemFont()
	if fontPath == "" {
		return nil, fmt.Errorf("no system font found")
	}

	source, err := text.NewFontSourceFromFile(fontPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load font %s: %v", fontPath, err)
	}

	return source, nil
}

// findSystemFont returns the path to a TTF font found on the system.
func findSystemFont() string {
	candidates := []string{
		// Windows
		"C:\\Windows\\Fonts\\arial.ttf",
		"C:\\Windows\\Fonts\\calibri.ttf",
		"C:\\Windows\\Fonts\\segoeui.ttf",
		// macOS
		"/System/Library/Fonts/Supplemental/Verdana.ttf",
		"/Library/Fonts/Arial.ttf",
		"/System/Library/Fonts/Supplemental/Arial.ttf",
		"/System/Library/Fonts/Supplemental/Courier New.ttf",
		"/System/Library/Fonts/Monaco.ttf",
		// Linux
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"/usr/share/fonts/liberation/LiberationSans-Regular.ttf",
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
