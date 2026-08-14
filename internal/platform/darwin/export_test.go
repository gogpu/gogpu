//go:build darwin

package darwin

// MenuItemActionForTest returns the Go callback stored for item, or nil.
func MenuItemActionForTest(item ID) func() {
	return getMenuItemAction(item)
}
