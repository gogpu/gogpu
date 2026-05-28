//go:build linux

package platform

import "fmt"

func showOpenFileDialog(_ FileDialogOptions) ([]string, error) {
	return nil, fmt.Errorf("file dialog: not yet implemented on Linux (xdg-portal support planned)")
}

func showSaveFileDialog(_ FileDialogOptions) (string, error) {
	return "", fmt.Errorf("file dialog: not yet implemented on Linux (xdg-portal support planned)")
}
