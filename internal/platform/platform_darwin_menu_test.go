//go:build darwin

package platform

import "testing"

func TestMacOSMenuDestination(t *testing.T) {
	tests := []struct {
		name string
		item MenuItem
		want menuDestination
	}{
		{
			name: "submenu goes to menu bar",
			item: MenuItem{Title: "File", Submenu: []MenuItem{{Title: "Open"}}},
			want: menuDestMenuBar,
		},
		{
			name: "separator goes to app menu",
			item: MenuItem{Separator: true},
			want: menuDestAppMenu,
		},
		{
			name: "about role goes to app menu",
			item: MenuItem{Title: "About", Role: MenuRoleAbout},
			want: menuDestAppMenu,
		},
		{
			name: "quit role with action goes to app menu",
			item: MenuItem{Title: "Quit", Role: MenuRoleQuit, Action: func() {}},
			want: menuDestAppMenu,
		},
		{
			name: "leaf without role is skipped",
			item: MenuItem{Title: "Orphan", Action: func() {}},
			want: menuDestSkip,
		},
		{
			name: "submenu wins over role",
			item: MenuItem{
				Title:   "App",
				Role:    MenuRoleAbout,
				Submenu: []MenuItem{{Title: "Nested"}},
			},
			want: menuDestMenuBar,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := macOSMenuDestination(tt.item); got != tt.want {
				t.Fatalf("macOSMenuDestination() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMacOSMenuDestination_Issue456Reproduction(t *testing.T) {
	// Mirrors the #456 reproduction: separators interleaved with Role items
	// must all land in the App Menu, not the invisible menu bar.
	items := []MenuItem{
		{Separator: true},
		{Title: "About Foo", Role: MenuRoleAbout},
		{Separator: true},
		{Title: "Custom About", Role: MenuRoleAbout, Action: func() {}},
	}
	for i, item := range items {
		if got := macOSMenuDestination(item); got != menuDestAppMenu {
			t.Fatalf("item[%d] destination = %v, want app menu", i, got)
		}
	}
}
