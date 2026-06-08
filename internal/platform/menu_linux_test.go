//go:build linux

package platform

import (
	"encoding/binary"
	"sync"
	"testing"
	"time"

	"github.com/gogpu/gpucontext"
)

// --- buildMenuNodes ---

func TestBuildMenuNodes_IDsDepthFirst(t *testing.T) {
	items := []MenuItem{
		{Title: "File", Submenu: []MenuItem{
			{Title: "New"},
			{Title: "Open"},
		}},
		{Title: "Edit", Submenu: []MenuItem{
			{Title: "Copy"},
		}},
	}
	var idGen int32
	var actions sync.Map
	nodes := buildMenuNodes(items, &idGen, &actions)

	// File=1, New=2, Open=3, Edit=4, Copy=5 (depth-first)
	if nodes[0].id != 1 {
		t.Errorf("File id = %d, want 1", nodes[0].id)
	}
	if nodes[0].children[0].id != 2 {
		t.Errorf("New id = %d, want 2", nodes[0].children[0].id)
	}
	if nodes[0].children[1].id != 3 {
		t.Errorf("Open id = %d, want 3", nodes[0].children[1].id)
	}
	if nodes[1].id != 4 {
		t.Errorf("Edit id = %d, want 4", nodes[1].id)
	}
	if nodes[1].children[0].id != 5 {
		t.Errorf("Copy id = %d, want 5", nodes[1].children[0].id)
	}
}

func TestBuildMenuNodes_ActionsRegisteredForLeaves(t *testing.T) {
	var called bool
	items := []MenuItem{
		{Title: "File", Submenu: []MenuItem{
			{Title: "Quit", Action: func() { called = true }},
		}},
	}
	var idGen int32
	var actions sync.Map
	buildMenuNodes(items, &idGen, &actions)

	// "File" (id=1) has submenu — no action stored
	// "Quit" (id=2) is a leaf with Action
	if _, ok := actions.Load(int32(1)); ok {
		t.Error("action stored for submenu parent (File), want none")
	}
	if fn, ok := actions.Load(int32(2)); !ok {
		t.Error("action not stored for leaf item (Quit)")
	} else {
		fn.(func())()
		if !called {
			t.Error("stored action for Quit did not invoke the original func")
		}
	}
}

func TestBuildMenuNodes_SeparatorNoAction(t *testing.T) {
	items := []MenuItem{
		{Separator: true, Action: func() {}},
	}
	var idGen int32
	var actions sync.Map
	buildMenuNodes(items, &idGen, &actions)

	if _, ok := actions.Load(int32(1)); ok {
		t.Error("action stored for separator item, want none")
	}
}

func TestBuildMenuNodes_NilAction(t *testing.T) {
	items := []MenuItem{{Title: "About"}}
	var idGen int32
	var actions sync.Map
	nodes := buildMenuNodes(items, &idGen, &actions)

	if nodes[0].id != 1 {
		t.Errorf("id = %d, want 1", nodes[0].id)
	}
	if _, ok := actions.Load(int32(1)); ok {
		t.Error("action stored for item with nil Action")
	}
}

// --- findNode ---

func TestFindNode_Root(t *testing.T) {
	root := &menuNode{id: 0}
	if findNode(root, 0) != root {
		t.Error("findNode(root, 0): expected root")
	}
}

func TestFindNode_Child(t *testing.T) {
	child := &menuNode{id: 3}
	root := &menuNode{id: 0, children: []*menuNode{{id: 1}, {id: 2, children: []*menuNode{child}}}}
	if findNode(root, 3) != child {
		t.Error("findNode did not return the nested child with id=3")
	}
}

func TestFindNode_Missing(t *testing.T) {
	root := &menuNode{id: 0, children: []*menuNode{{id: 1}}}
	if findNode(root, 99) != nil {
		t.Error("findNode with missing id should return nil")
	}
}

func TestFindNode_Nil(t *testing.T) {
	if findNode(nil, 0) != nil {
		t.Error("findNode(nil, 0) should return nil")
	}
}

// --- encodeItemProps ---

func TestEncodeItemProps_Separator(t *testing.T) {
	b := newMsgBuf(0)
	item := MenuItem{Separator: true}
	encodeItemProps(b, &item, 1)

	if len(b.data) == 0 {
		t.Fatal("encodeItemProps for separator produced no output")
	}
	// Must contain "separator" string but NOT "label"
	data := string(b.data)
	if !containsStr(data, "separator") {
		t.Error("separator item missing 'separator' type value")
	}
	if containsStr(data, "label") {
		t.Error("separator item must not have 'label' property")
	}
}

func TestEncodeItemProps_LeafEnabled(t *testing.T) {
	b := newMsgBuf(0)
	item := MenuItem{Title: "Open", Disabled: false}
	encodeItemProps(b, &item, 1)

	data := string(b.data)
	if !containsStr(data, "label") {
		t.Error("leaf item missing 'label' key")
	}
	if !containsStr(data, "Open") {
		t.Error("leaf item missing title 'Open'")
	}
	if !containsStr(data, "enabled") {
		t.Error("leaf item missing 'enabled' key")
	}
}

func TestEncodeItemProps_Disabled(t *testing.T) {
	b := newMsgBuf(0)
	item := MenuItem{Title: "Grayed", Disabled: true}
	encodeItemProps(b, &item, 1)

	// "enabled" key present; value must be bool32(false) = 4 zero bytes.
	data := b.data
	if !bytesContain(data, []byte("enabled")) {
		t.Error("disabled item missing 'enabled' key")
	}
	// Check that the bool value 0 appears somewhere in the encoded output.
	hasZeroBool := false
	for i := 0; i+3 < len(data); i++ {
		if data[i] == 0 && data[i+1] == 0 && data[i+2] == 0 && data[i+3] == 0 {
			hasZeroBool = true
			break
		}
	}
	if !hasZeroBool {
		t.Error("disabled item: no zero-value bool32 found in encoding")
	}
}

func TestEncodeItemProps_WithSubmenu(t *testing.T) {
	b := newMsgBuf(0)
	item := MenuItem{Title: "File", Submenu: []MenuItem{{Title: "New"}}}
	encodeItemProps(b, &item, 1)

	if !containsStr(string(b.data), "children-display") {
		t.Error("item with submenu missing 'children-display' property")
	}
	if !containsStr(string(b.data), "submenu") {
		t.Error("item with submenu missing 'submenu' value")
	}
}

// --- encodeMenuLayout ---

// TestEncodeMenuLayout_EmptyRoot verifies the root node with no children
// produces a decodable (ia{sv}av) struct.
func TestEncodeMenuLayout_EmptyRoot(t *testing.T) {
	b := newMsgBuf(0)
	root := &menuNode{id: menuRootID}
	encodeMenuLayout(b, root, -1)

	if len(b.data) == 0 {
		t.Fatal("encodeMenuLayout produced no bytes")
	}

	d := newMsgDecoder(b.data, 0)
	_ = d.alignTo(8) // struct alignment
	id, err := d.readU32()
	if err != nil {
		t.Fatalf("decode id: %v", err)
	}
	if int32(id) != menuRootID {
		t.Errorf("root id = %d, want 0", id)
	}
}

// TestEncodeMenuLayout_ChildrenAppear verifies children are included at depth -1.
func TestEncodeMenuLayout_ChildrenAppear(t *testing.T) {
	b := newMsgBuf(0)
	root := &menuNode{
		id: menuRootID,
		children: []*menuNode{
			{id: 1, item: MenuItem{Title: "File"}},
			{id: 2, item: MenuItem{Title: "Edit"}},
		},
	}
	encodeMenuLayout(b, root, -1)

	// Both child titles must appear in the encoded bytes.
	data := string(b.data)
	if !containsStr(data, "File") {
		t.Error("encoded layout missing child title 'File'")
	}
	if !containsStr(data, "Edit") {
		t.Error("encoded layout missing child title 'Edit'")
	}
}

// TestEncodeMenuLayout_DepthZeroNoChildren verifies depth=0 omits children.
func TestEncodeMenuLayout_DepthZeroNoChildren(t *testing.T) {
	b := newMsgBuf(0)
	root := &menuNode{
		id: menuRootID,
		children: []*menuNode{
			{id: 1, item: MenuItem{Title: "Hidden"}},
		},
	}
	encodeMenuLayout(b, root, 0)

	if containsStr(string(b.data), "Hidden") {
		t.Error("depth=0: child title 'Hidden' must not appear")
	}
}

// --- handleGetLayout ---

func TestHandleGetLayout_EmptyMenu(t *testing.T) {
	m := newLinuxMenuState()
	body, sig := m.handleGetLayout(makeGetLayoutArgs(0, -1))

	if sig != dbusMenuLayoutSig {
		t.Errorf("signature = %q, want %q", sig, dbusMenuLayoutSig)
	}
	if len(body) == 0 {
		t.Fatal("GetLayout empty menu: empty body")
	}
	d := newMsgDecoder(body, 0)
	rev, err := d.readU32()
	if err != nil {
		t.Fatalf("read revision: %v", err)
	}
	if rev != 0 {
		t.Errorf("revision = %d, want 0 for fresh state", rev)
	}
}

func TestHandleGetLayout_AfterSet(t *testing.T) {
	m := newLinuxMenuState()
	m.set([]MenuItem{
		{Title: "File", Submenu: []MenuItem{{Title: "Quit"}}},
	})

	body, sig := m.handleGetLayout(makeGetLayoutArgs(0, -1))
	if sig != dbusMenuLayoutSig {
		t.Errorf("signature = %q, want %q", sig, dbusMenuLayoutSig)
	}

	d := newMsgDecoder(body, 0)
	rev, _ := d.readU32()
	if rev != 1 {
		t.Errorf("revision = %d, want 1 after one set() call", rev)
	}

	// Layout bytes must contain both menu titles.
	if !containsStr(string(body), "File") {
		t.Error("GetLayout body missing 'File'")
	}
	if !containsStr(string(body), "Quit") {
		t.Error("GetLayout body missing 'Quit'")
	}
}

func TestHandleGetLayout_RevisionIncrementsOnSet(t *testing.T) {
	m := newLinuxMenuState()
	m.set([]MenuItem{{Title: "A"}})
	m.set([]MenuItem{{Title: "B"}})

	body, _ := m.handleGetLayout(makeGetLayoutArgs(0, -1))
	d := newMsgDecoder(body, 0)
	rev, _ := d.readU32()
	if rev != 2 {
		t.Errorf("revision = %d, want 2 after two set() calls", rev)
	}
}

// --- handleAboutToShow ---

func TestHandleAboutToShow_ReturnsFalse(t *testing.T) {
	m := newLinuxMenuState()
	body, sig := m.handleAboutToShow()

	if sig != "b" {
		t.Errorf("sig = %q, want %q", sig, "b")
	}
	d := newMsgDecoder(body, 0)
	v, _ := d.readU32()
	if v != 0 {
		t.Errorf("AboutToShow = %d, want 0 (false)", v)
	}
}

// --- handleEvent ---

func TestHandleEvent_ClickedDispatchesAction(t *testing.T) {
	m := newLinuxMenuState()
	done := make(chan struct{})
	m.actions.Store(int32(42), func() { close(done) })

	m.handleEvent(makeEventArgs(42, dbusMenuEventClicked))

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("handleEvent 'clicked': action not called within 1s")
	}
}

func TestHandleEvent_OtherEventNoAction(t *testing.T) {
	m := newLinuxMenuState()
	fired := make(chan struct{}, 1)
	m.actions.Store(int32(1), func() { fired <- struct{}{} })

	m.handleEvent(makeEventArgs(1, "hovered"))

	select {
	case <-fired:
		t.Error("handleEvent 'hovered': action must not be called")
	case <-time.After(50 * time.Millisecond):
		// correct: action was not invoked
	}
}

func TestHandleEvent_UnknownID_NoPanic(t *testing.T) {
	m := newLinuxMenuState()
	m.handleEvent(makeEventArgs(999, "clicked"))
}

// --- handleGetGroupProperties ---

func TestHandleGetGroupProperties_KnownID(t *testing.T) {
	m := newLinuxMenuState()
	m.set([]MenuItem{{Title: "File"}})

	args := makeGetGroupPropsArgs([]int32{1})
	body, sig := m.handleGetGroupProperties(args)

	if sig != dbusMenuPropsSig {
		t.Errorf("sig = %q, want %q", sig, dbusMenuPropsSig)
	}
	if !containsStr(string(body), "File") {
		t.Error("GetGroupProperties missing 'File' label")
	}
}

func TestHandleGetGroupProperties_UnknownIDSkipped(t *testing.T) {
	m := newLinuxMenuState()
	args := makeGetGroupPropsArgs([]int32{99})
	body, sig := m.handleGetGroupProperties(args)

	if sig != dbusMenuPropsSig {
		t.Errorf("sig = %q, want %q", sig, dbusMenuPropsSig)
	}
	// Unknown ID → empty array; body must still be valid (non-zero for array length).
	if len(body) < 4 {
		t.Error("GetGroupProperties for unknown ID: body too short")
	}
}

// --- handleEventGroup ---

func TestHandleEventGroup_Empty(t *testing.T) {
	m := newLinuxMenuState()
	b := newMsgBuf(0)
	lp, cp := b.arrayStart(8)
	b.arrayEnd(lp, cp)

	body, sig := m.handleEventGroup(b.data)
	if sig != "ai" {
		t.Errorf("sig = %q, want %q", sig, "ai")
	}
	if len(body) < 4 {
		t.Error("EventGroup: body too short")
	}
}

func TestHandleEventGroup_ClickedDispatches(t *testing.T) {
	m := newLinuxMenuState()
	done := make(chan struct{})
	m.actions.Store(int32(7), func() { close(done) })

	m.handleEventGroup(makeEventGroupArgs([]eventGroupEntry{{id: 7, eventID: dbusMenuEventClicked}}))

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("handleEventGroup 'clicked': action not called within 1s")
	}
}

// --- menuEncodeReturn ---

func TestMenuEncodeReturn_FixedHeader(t *testing.T) {
	body := []byte{0x01, 0x02}
	raw := menuEncodeReturn(10, 5, ":1.33", "u", body)

	if len(raw) < 16 {
		t.Fatalf("message too short: %d bytes", len(raw))
	}
	if raw[0] != 'l' {
		t.Errorf("endian byte = %c, want 'l'", raw[0])
	}
	if raw[1] != dbusMsgReturn {
		t.Errorf("type = %d, want %d (METHOD_RETURN)", raw[1], dbusMsgReturn)
	}
	if raw[3] != 1 {
		t.Errorf("protocol version = %d, want 1", raw[3])
	}
	if binary.LittleEndian.Uint32(raw[8:12]) != 10 {
		t.Error("serial field != 10")
	}
	// DESTINATION must be present so the daemon routes the reply back to the caller.
	if !containsStr(string(raw), ":1.33") {
		t.Error("METHOD_RETURN missing DESTINATION field — daemon cannot route reply")
	}
}

func TestMenuEncodeReturn_EmptyBody(t *testing.T) {
	raw := menuEncodeReturn(1, 2, ":1.42", "", nil)
	bodyLen := binary.LittleEndian.Uint32(raw[4:8])
	if bodyLen != 0 {
		t.Errorf("empty body: bodyLen = %d, want 0", bodyLen)
	}
}

func TestMenuEncodeReturn_DestinationRouting(t *testing.T) {
	// Without DESTINATION the D-Bus daemon cannot route the reply and the
	// caller times out. Verify the field is always encoded.
	raw := menuEncodeReturn(1, 42, ":1.100", "b", []byte{1, 0, 0, 0})
	if !containsStr(string(raw), ":1.100") {
		t.Error("METHOD_RETURN: DESTINATION header field missing or not encoded")
	}
}

// --- menuEncodeSignal ---

func TestMenuEncodeSignal_FixedHeader(t *testing.T) {
	raw := menuEncodeSignal(3, "/path", "iface", "Member", "u", []byte{0, 0, 0, 0})

	if raw[1] != dbusMsgSignal {
		t.Errorf("type = %d, want %d (SIGNAL)", raw[1], dbusMsgSignal)
	}
	if binary.LittleEndian.Uint32(raw[8:12]) != 3 {
		t.Error("serial field != 3")
	}
}

// --- dbusAssembleMsg alignment ---

func TestDbusAssembleMsg_8ByteAligned(t *testing.T) {
	raw := dbusAssembleMsg(dbusMsgReturn, 0, 1, []byte{0xAA}, nil)
	hdrArrayLen := binary.LittleEndian.Uint32(raw[12:16])
	headerTotal := 16 + int(hdrArrayLen)
	// Body must start on an 8-byte boundary.
	padLen := (8 - headerTotal%8) % 8
	expected := headerTotal + padLen
	if len(raw) != expected {
		t.Errorf("total length = %d, want %d (8-byte aligned header)", len(raw), expected)
	}
}

func TestDbusAssembleMsg_FlagsWritten(t *testing.T) {
	raw := dbusAssembleMsg(dbusMsgCall, dbusFlagNoReplyExpected, 1, nil, nil)
	if raw[2] != dbusFlagNoReplyExpected {
		t.Errorf("flags byte = %#x, want %#x", raw[2], dbusFlagNoReplyExpected)
	}
}

func TestDbusAssembleMsg_ZeroFlags(t *testing.T) {
	raw := dbusAssembleMsg(dbusMsgReturn, 0, 1, nil, nil)
	if raw[2] != 0 {
		t.Errorf("flags byte = %#x, want 0 for METHOD_RETURN", raw[2])
	}
}

// TestMenuEncodeCall_NoFlags verifies that RegisterWindow calls have flags=0 so
// the registrar sends a METHOD_RETURN or ERROR that serve() can log.
func TestMenuEncodeCall_NoFlags(t *testing.T) {
	raw := menuEncodeCall(1, "dest", "/path", "iface", "Method", "", nil)
	if len(raw) < 3 {
		t.Fatal("encoded call too short")
	}
	if raw[2] != 0 {
		t.Errorf("flags byte = %#x, want 0 (reply expected)", raw[2])
	}
}

// TestMenuEncodeReturn_NoFlags verifies METHOD_RETURN has no flags set.
func TestMenuEncodeReturn_NoFlags(t *testing.T) {
	raw := menuEncodeReturn(1, 1, ":1.1", "", nil)
	if raw[2] != 0 {
		t.Errorf("METHOD_RETURN flags = %#x, want 0", raw[2])
	}
}

// TestMenuEncodeSignal_NoFlags verifies SIGNAL has no flags set.
func TestMenuEncodeSignal_NoFlags(t *testing.T) {
	raw := menuEncodeSignal(1, "/path", "iface", "Signal", "", nil)
	if raw[2] != 0 {
		t.Errorf("SIGNAL flags = %#x, want 0", raw[2])
	}
}

// --- Role actions ---

// stubWindow implements PlatformWindow for testing role-based menu actions.
type stubWindow struct {
	closed, minimized, maximized bool
	fullscreen                   bool
}

func (s *stubWindow) ID() WindowID                                                         { return 0 }
func (s *stubWindow) GetHandle() (uintptr, uintptr)                                        { return 0, 0 }
func (s *stubWindow) LogicalSize() (int, int)                                              { return 0, 0 }
func (s *stubWindow) PhysicalSize() (int, int)                                             { return 0, 0 }
func (s *stubWindow) ScaleFactor() float64                                                 { return 1 }
func (s *stubWindow) PrepareFrame() PrepareFrameResult                                     { return PrepareFrameResult{} }
func (s *stubWindow) InSizeMove() bool                                                     { return false }
func (s *stubWindow) ShouldClose() bool                                                    { return s.closed }
func (s *stubWindow) SetTitle(_ string)                                                    {}
func (s *stubWindow) SetCursor(_ int)                                                      {}
func (s *stubWindow) SetFrameless(_ bool)                                                  {}
func (s *stubWindow) IsFrameless() bool                                                    { return false }
func (s *stubWindow) SetFullscreen(fs bool)                                                { s.fullscreen = fs }
func (s *stubWindow) IsFullscreen() bool                                                   { return s.fullscreen }
func (s *stubWindow) SetHitTestCallback(_ func(float64, float64) gpucontext.HitTestResult) {}
func (s *stubWindow) Minimize()                                                            { s.minimized = true }
func (s *stubWindow) Maximize()                                                            { s.maximized = true }
func (s *stubWindow) IsMaximized() bool                                                    { return s.maximized }
func (s *stubWindow) Close()                                                               { s.closed = true }
func (s *stubWindow) Show()                                                                {}
func (s *stubWindow) SyncFrame()                                                           {}
func (s *stubWindow) SetCursorMode(_ int)                                                  {}
func (s *stubWindow) CursorMode() int                                                      { return 0 }
func (s *stubWindow) SetModalFrameCallback(_ func())                                       {}
func (s *stubWindow) Destroy()                                                             {}

func TestRoleQuit_ClosesWindow(t *testing.T) {
	m := newLinuxMenuState()
	win := &stubWindow{}
	m.window = win
	m.set([]MenuItem{{Title: "Quit", Role: MenuRoleQuit}})

	fn, ok := m.actions.Load(int32(1))
	if !ok {
		t.Fatal("MenuRoleQuit: no action registered")
	}
	fn.(func())()
	if !win.closed {
		t.Error("MenuRoleQuit: window.Close() not called")
	}
}

func TestRoleQuit_CallsUserActionFirst(t *testing.T) {
	m := newLinuxMenuState()
	win := &stubWindow{}
	m.window = win
	var order []string
	m.set([]MenuItem{{
		Title:  "Quit",
		Role:   MenuRoleQuit,
		Action: func() { order = append(order, "user") },
	}})

	fn, ok := m.actions.Load(int32(1))
	if !ok {
		t.Fatal("no action for MenuRoleQuit with user Action")
	}
	fn.(func())()
	if len(order) != 1 || order[0] != "user" {
		t.Errorf("user action not called before quit; order=%v", order)
	}
	if !win.closed {
		t.Error("window.Close() not called")
	}
}

func TestRoleQuit_NoWindow_FallsBackToExit(t *testing.T) {
	// window is nil → os.Exit(0) path. Just verify the action is registered.
	m := newLinuxMenuState()
	m.set([]MenuItem{{Title: "Quit", Role: MenuRoleQuit}})
	if _, ok := m.actions.Load(int32(1)); !ok {
		t.Fatal("MenuRoleQuit without window: no action registered")
	}
}

func TestRoleMinimize_CallsWindowMinimize(t *testing.T) {
	m := newLinuxMenuState()
	win := &stubWindow{}
	m.window = win
	m.set([]MenuItem{{Title: "Minimize", Role: MenuRoleMinimize}})

	fn, ok := m.actions.Load(int32(1))
	if !ok {
		t.Fatal("MenuRoleMinimize: no action registered")
	}
	fn.(func())()
	if !win.minimized {
		t.Error("Minimize role: window.Minimize() not called")
	}
}

func TestRoleZoom_CallsWindowMaximize(t *testing.T) {
	m := newLinuxMenuState()
	win := &stubWindow{}
	m.window = win
	m.set([]MenuItem{{Title: "Zoom", Role: MenuRoleZoom}})

	fn, ok := m.actions.Load(int32(1))
	if !ok {
		t.Fatal("MenuRoleZoom: no action registered")
	}
	fn.(func())()
	if !win.maximized {
		t.Error("Zoom role: window.Maximize() not called")
	}
}

func TestRoleFullScreen_TogglesFullscreen(t *testing.T) {
	m := newLinuxMenuState()
	win := &stubWindow{}
	m.window = win
	m.set([]MenuItem{{Title: "FullScreen", Role: MenuRoleFullScreen}})

	fn, ok := m.actions.Load(int32(1))
	if !ok {
		t.Fatal("MenuRoleFullScreen: no action registered")
	}
	fn.(func())()
	if !win.fullscreen {
		t.Error("FullScreen role: window.SetFullscreen(true) not called")
	}
	fn.(func())() // toggle back
	if win.fullscreen {
		t.Error("FullScreen role: second call should toggle off")
	}
}

func TestRoleAbout_DispatchesUserAction(t *testing.T) {
	m := newLinuxMenuState()
	var called bool
	m.set([]MenuItem{{
		Title:  "About",
		Role:   MenuRoleAbout,
		Action: func() { called = true },
	}})

	fn, ok := m.actions.Load(int32(1))
	if !ok {
		t.Fatal("MenuRoleAbout with Action: no action registered")
	}
	fn.(func())()
	if !called {
		t.Error("MenuRoleAbout: user Action not called")
	}
}

func TestRoleAbout_NoAction_NotRegistered(t *testing.T) {
	m := newLinuxMenuState()
	m.set([]MenuItem{{Title: "About", Role: MenuRoleAbout}})
	if _, ok := m.actions.Load(int32(1)); ok {
		t.Error("MenuRoleAbout without Action should not register any action")
	}
}

// --- menuSyntheticWinID ---

func TestMenuSyntheticWinID_NonZero(t *testing.T) {
	id := menuSyntheticWinID()
	if id == 0 {
		t.Error("menuSyntheticWinID() = 0, want non-zero PID-based ID")
	}
}

func TestMenuSyntheticWinID_Stable(t *testing.T) {
	a := menuSyntheticWinID()
	b := menuSyntheticWinID()
	if a != b {
		t.Errorf("menuSyntheticWinID() not stable: %d != %d", a, b)
	}
}

// --- linuxMenuState.attachWindow Wayland behavior ---

// TestAttachWindow_WaylandKeepsWinIDZero verifies that on a pure Wayland session
// (winID=0) the registration winID stays 0 so KDE matches by D-Bus sender PID,
// while the object path is non-empty and PID-based for uniqueness.
func TestAttachWindow_WaylandKeepsWinIDZero(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/nonexistent")

	m := newLinuxMenuState()
	m.attachWindow(0) // Wayland: no X11 XID

	m.mu.Lock()
	wid := m.winID
	path := m.objPath
	m.mu.Unlock()

	if wid != 0 {
		t.Errorf("winID = %d, want 0 for Wayland (KDE matches by D-Bus sender PID)", wid)
	}
	if path == "" {
		t.Error("objPath is empty, want PID-based path for uniqueness")
	}
	if path == menuObjPrefix+"0" {
		t.Errorf("objPath = %q ends with /0 — should use PID, not zero", path)
	}
}

// TestAttachWindow_X11PreservesXID verifies that on X11 the real XID is kept
// as both the registration winID and the object path suffix.
func TestAttachWindow_X11PreservesXID(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/nonexistent")

	const xid uint32 = 0xDEAD
	m := newLinuxMenuState()
	m.attachWindow(xid)

	m.mu.Lock()
	wid := m.winID
	path := m.objPath
	m.mu.Unlock()

	if wid != xid {
		t.Errorf("winID = %d, want %d (X11 XID preserved)", wid, xid)
	}
	want := menuObjPrefix + "57005" // 0xDEAD = 57005
	if path != want {
		t.Errorf("objPath = %q, want %q", path, want)
	}
}

// --- linuxMenuState.set ---

func TestLinuxMenuState_SetBeforeAttach(t *testing.T) {
	m := newLinuxMenuState()
	m.set([]MenuItem{{Title: "File"}})

	m.mu.Lock()
	n := len(m.items)
	rev := m.revision
	m.mu.Unlock()

	if n != 1 {
		t.Errorf("items length = %d, want 1", n)
	}
	if rev != 1 {
		t.Errorf("revision = %d, want 1", rev)
	}
}

func TestLinuxMenuState_SetBuildsTree(t *testing.T) {
	m := newLinuxMenuState()
	m.set([]MenuItem{
		{Title: "File", Submenu: []MenuItem{{Title: "Open"}}},
	})

	m.mu.Lock()
	root := m.root
	m.mu.Unlock()

	if root == nil {
		t.Fatal("root is nil after set()")
	}
	if len(root.children) != 1 {
		t.Fatalf("root children = %d, want 1", len(root.children))
	}
	if root.children[0].item.Title != "File" {
		t.Errorf("first child title = %q, want %q", root.children[0].item.Title, "File")
	}
}

// --- AddToSystemMenu ---

func TestX11AddToSystemMenu_AlwaysFalse(t *testing.T) {
	p := &x11Platform{menu: newLinuxMenuState()}
	for _, m := range []SystemMenu{SystemMenuApplication, SystemMenuWindow} {
		if p.AddToSystemMenu(m, nil) {
			t.Errorf("AddToSystemMenu(%v) = true, want false", m)
		}
	}
}

func TestWaylandAddToSystemMenu_AlwaysFalse(t *testing.T) {
	p := &waylandPlatform{menu: newLinuxMenuState()}
	for _, m := range []SystemMenu{SystemMenuApplication, SystemMenuWindow} {
		if p.AddToSystemMenu(m, nil) {
			t.Errorf("AddToSystemMenu(%v) = true, want false", m)
		}
	}
}

// --- helpers ---

// makeGetLayoutArgs encodes GetLayout(i parentId, i recursionDepth, as propertyNames) args.
func makeGetLayoutArgs(parentID, depth int32) []byte {
	b := newMsgBuf(0)
	b.u32(uint32(parentID))
	b.u32(uint32(depth))
	lp, cp := b.arrayStart(4) // as: array of strings
	b.arrayEnd(lp, cp)
	return b.data
}

// makeEventArgs encodes Event(i id, s eventId, v data, u timestamp) args.
func makeEventArgs(id int32, eventID string) []byte {
	b := newMsgBuf(0)
	b.u32(uint32(id))
	b.str(eventID)
	b.variantStr("") // v data (empty string variant)
	b.u32(0)         // timestamp
	return b.data
}

// makeGetGroupPropsArgs encodes GetGroupProperties(ai ids, as propertyNames) args.
func makeGetGroupPropsArgs(ids []int32) []byte {
	b := newMsgBuf(0)
	lp, cp := b.arrayStart(4)
	for _, id := range ids {
		b.u32(uint32(id))
	}
	b.arrayEnd(lp, cp)
	// as propertyNames (empty)
	lp2, cp2 := b.arrayStart(4)
	b.arrayEnd(lp2, cp2)
	return b.data
}

type eventGroupEntry struct {
	id      int32
	eventID string
}

// makeEventGroupArgs encodes EventGroup(a(isvu) events) args.
func makeEventGroupArgs(events []eventGroupEntry) []byte {
	b := newMsgBuf(0)
	lp, cp := b.arrayStart(8) // array of structs
	for _, e := range events {
		b.padTo(8) // struct alignment
		b.u32(uint32(e.id))
		b.str(e.eventID)
		b.variantStr("")
		b.u32(0) // timestamp
	}
	b.arrayEnd(lp, cp)
	return b.data
}

// --- hasSameTreeStructure ---

func TestHasSameTreeStructure_IdenticalTrees(t *testing.T) {
	tree := func() *menuNode {
		return &menuNode{id: menuRootID, children: []*menuNode{
			{id: 1, item: MenuItem{Title: "File"}, children: []*menuNode{
				{id: 2, item: MenuItem{Title: "Open"}},
				{id: 3, item: MenuItem{Title: "Quit"}},
			}},
		}}
	}
	if !hasSameTreeStructure(tree(), tree()) {
		t.Error("identical trees: expected same structure")
	}
}

func TestHasSameTreeStructure_DisabledDiffers_StillSame(t *testing.T) {
	a := &menuNode{id: menuRootID, children: []*menuNode{
		{id: 1, item: MenuItem{Title: "Save", Disabled: false}},
	}}
	b := &menuNode{id: menuRootID, children: []*menuNode{
		{id: 1, item: MenuItem{Title: "Save", Disabled: true}},
	}}
	if !hasSameTreeStructure(a, b) {
		t.Error("Disabled-only diff: structure must still be considered identical")
	}
}

func TestHasSameTreeStructure_DifferentChildCount(t *testing.T) {
	a := &menuNode{id: menuRootID, children: []*menuNode{
		{id: 1, item: MenuItem{Title: "File"}},
	}}
	b := &menuNode{id: menuRootID, children: []*menuNode{
		{id: 1, item: MenuItem{Title: "File"}},
		{id: 2, item: MenuItem{Title: "Edit"}},
	}}
	if hasSameTreeStructure(a, b) {
		t.Error("different child count: expected different structure")
	}
}

func TestHasSameTreeStructure_DifferentTitle(t *testing.T) {
	a := &menuNode{id: menuRootID, children: []*menuNode{
		{id: 1, item: MenuItem{Title: "File"}},
	}}
	b := &menuNode{id: menuRootID, children: []*menuNode{
		{id: 1, item: MenuItem{Title: "Document"}},
	}}
	if hasSameTreeStructure(a, b) {
		t.Error("different title: expected different structure")
	}
}

func TestHasSameTreeStructure_SeparatorDiffers(t *testing.T) {
	a := &menuNode{id: menuRootID, children: []*menuNode{
		{id: 1, item: MenuItem{Separator: false}},
	}}
	b := &menuNode{id: menuRootID, children: []*menuNode{
		{id: 1, item: MenuItem{Separator: true}},
	}}
	if hasSameTreeStructure(a, b) {
		t.Error("separator mismatch: expected different structure")
	}
}

func TestHasSameTreeStructure_NilInputs(t *testing.T) {
	if hasSameTreeStructure(nil, nil) != true {
		t.Error("(nil, nil) should be true")
	}
	root := &menuNode{id: menuRootID}
	if hasSameTreeStructure(nil, root) != false {
		t.Error("(nil, non-nil) should be false")
	}
	if hasSameTreeStructure(root, nil) != false {
		t.Error("(non-nil, nil) should be false")
	}
}

// --- collectDisabledChanges ---

func TestCollectDisabledChanges_OneChanged(t *testing.T) {
	prev := &menuNode{id: menuRootID, children: []*menuNode{
		{id: 1, item: MenuItem{Title: "Cut", Disabled: false}},
		{id: 2, item: MenuItem{Title: "Paste", Disabled: false}},
	}}
	next := &menuNode{id: menuRootID, children: []*menuNode{
		{id: 1, item: MenuItem{Title: "Cut", Disabled: false}},
		{id: 2, item: MenuItem{Title: "Paste", Disabled: true}},
	}}
	changed := collectDisabledChanges(prev, next)
	if len(changed) != 1 {
		t.Fatalf("changed count = %d, want 1", len(changed))
	}
	if changed[0].id != 2 {
		t.Errorf("changed node id = %d, want 2 (Paste)", changed[0].id)
	}
}

func TestCollectDisabledChanges_NoneChanged(t *testing.T) {
	root := &menuNode{id: menuRootID, children: []*menuNode{
		{id: 1, item: MenuItem{Title: "New", Disabled: false}},
	}}
	changed := collectDisabledChanges(root, root)
	if len(changed) != 0 {
		t.Errorf("no changes expected, got %d", len(changed))
	}
}

func TestCollectDisabledChanges_SeparatorSkipped(t *testing.T) {
	prev := &menuNode{id: menuRootID, children: []*menuNode{
		{id: 1, item: MenuItem{Separator: true, Disabled: false}},
	}}
	next := &menuNode{id: menuRootID, children: []*menuNode{
		{id: 1, item: MenuItem{Separator: true, Disabled: true}}, // separators have no enabled prop
	}}
	changed := collectDisabledChanges(prev, next)
	if len(changed) != 0 {
		t.Errorf("separator: expected 0 changed nodes, got %d", len(changed))
	}
}

func TestCollectDisabledChanges_DeepTree(t *testing.T) {
	prev := &menuNode{id: menuRootID, children: []*menuNode{
		{id: 1, item: MenuItem{Title: "File"}, children: []*menuNode{
			{id: 2, item: MenuItem{Title: "Open", Disabled: false}},
			{id: 3, item: MenuItem{Title: "Save", Disabled: false}},
		}},
	}}
	next := &menuNode{id: menuRootID, children: []*menuNode{
		{id: 1, item: MenuItem{Title: "File"}, children: []*menuNode{
			{id: 2, item: MenuItem{Title: "Open", Disabled: true}},
			{id: 3, item: MenuItem{Title: "Save", Disabled: false}},
		}},
	}}
	changed := collectDisabledChanges(prev, next)
	if len(changed) != 1 {
		t.Fatalf("deep tree: changed = %d, want 1", len(changed))
	}
	if changed[0].id != 2 {
		t.Errorf("changed node id = %d, want 2 (Open)", changed[0].id)
	}
}

// --- buildItemsPropsBody ---

func TestBuildItemsPropsBody_ContainsEnabledKey(t *testing.T) {
	nodes := []*menuNode{
		{id: 3, item: MenuItem{Title: "Open", Disabled: false}},
		{id: 5, item: MenuItem{Title: "Save", Disabled: true}},
	}
	data := buildItemsPropsBody(nodes)
	if len(data) == 0 {
		t.Fatal("buildItemsPropsBody returned empty body")
	}
	if !bytesContain(data, []byte("enabled")) {
		t.Error("body missing 'enabled' property key")
	}
}

func TestBuildItemsPropsBody_BoolValues(t *testing.T) {
	nodes := []*menuNode{
		{id: 1, item: MenuItem{Disabled: false}}, // enabled=true  → bool32(1)
		{id: 2, item: MenuItem{Disabled: true}},  // enabled=false → bool32(0)
	}
	data := buildItemsPropsBody(nodes)

	hasTrue := false
	hasFalse := false
	for i := 0; i+3 < len(data); i++ {
		if data[i] == 1 && data[i+1] == 0 && data[i+2] == 0 && data[i+3] == 0 {
			hasTrue = true
		}
		if data[i] == 0 && data[i+1] == 0 && data[i+2] == 0 && data[i+3] == 0 {
			hasFalse = true
		}
	}
	if !hasTrue {
		t.Error("missing bool32(1) for enabled item")
	}
	if !hasFalse {
		t.Error("missing bool32(0) for disabled item")
	}
}

func TestBuildItemsPropsBody_EmptyNodes(t *testing.T) {
	data := buildItemsPropsBody(nil)
	// Body must still encode two empty arrays (a(ia{sv}) + a(ias)).
	if len(data) < 8 {
		t.Errorf("empty nodes: body length = %d, want >= 8 (two empty arrays)", len(data))
	}
}

// --- set() smart signal routing ---

func TestLinuxMenuState_Set_StructureChange_EmitsLayoutUpdated(t *testing.T) {
	m := newLinuxMenuState()

	// First set — establishes the tree (no conn, so no signal; just tree built)
	m.set([]MenuItem{{Title: "File"}})

	// Second set with more items — structural change
	m.set([]MenuItem{
		{Title: "File"},
		{Title: "Edit"}, // new top-level item
	})

	m.mu.Lock()
	root := m.root
	m.mu.Unlock()

	// Verify tree was rebuilt with the new structure (2 top-level items).
	if len(root.children) != 2 {
		t.Errorf("root children = %d, want 2 after structural change", len(root.children))
	}
}

func TestLinuxMenuState_Set_DisabledOnlyChange_TreeUpdated(t *testing.T) {
	m := newLinuxMenuState()

	m.set([]MenuItem{{Title: "Open", Disabled: false}})

	m.set([]MenuItem{{Title: "Open", Disabled: true}})

	m.mu.Lock()
	root := m.root
	m.mu.Unlock()

	if len(root.children) != 1 {
		t.Fatalf("root children = %d, want 1", len(root.children))
	}
	if !root.children[0].item.Disabled {
		t.Error("tree not updated: Disabled flag should be true after second set()")
	}
}

// containsStr reports whether s contains substr as a substring of raw bytes.
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// bytesContain reports whether data contains the given pattern.
func bytesContain(data, pattern []byte) bool {
	if len(pattern) == 0 {
		return true
	}
	for i := 0; i <= len(data)-len(pattern); i++ {
		match := true
		for j, b := range pattern {
			if data[i+j] != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
