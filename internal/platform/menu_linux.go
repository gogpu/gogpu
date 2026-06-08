//go:build linux

// Linux D-Bus AppMenu integration (com.canonical.dbusmenu).
//
// Implements the com.canonical.AppMenu.Registrar + com.canonical.dbusmenu
// protocols for KDE Plasma, Ubuntu Unity, GNOME + appmenu extension, and
// compatible desktop environments. Activated at runtime via
// DBUS_SESSION_BUS_ADDRESS; falls back silently to a no-op when the session
// bus is absent or com.canonical.AppMenu.Registrar is unreachable.
//
// Wire protocol reuses the hand-written D-Bus encoder/decoder from
// dbus_linux.go (ADR-036 Phase 2). No new external dependencies.
//
// Thread model: serve() goroutine reads incoming METHOD_CALLs; set() (called
// from the application goroutine) writes LayoutUpdated signals. Both paths
// share the connection write side under writeMu.
package platform

import (
	"os"
	"strconv"
	"sync"
	"sync/atomic"
)

// D-Bus AppMenu constants.
const (
	dbusMenuIface = "com.canonical.dbusmenu"
	appMenuDest   = "com.canonical.AppMenu.Registrar"
	appMenuPath   = "/com/canonical/AppMenu/Registrar"
	appMenuIface  = "com.canonical.AppMenu.Registrar"
	menuObjPrefix = "/com/gogpu/menu/window/"

	dbusMenuLayoutSig    = "u(ia{sv}av)" // GetLayout return signature
	dbusMenuPropsSig     = "a(ia{sv})"   // GetGroupProperties return signature
	dbusMenuEventClicked = "clicked"     // dbusmenu event ID for item activation

	dbusFlagNoReplyExpected byte = 0x01 // D-Bus message flag: sender does not expect a METHOD_RETURN
)

// menuRootID is the dbusmenu virtual root node ID (always 0, never displayed).
const menuRootID = int32(0)

// menuNode is a single item in the dbusmenu ID tree.
// ID 0 is the virtual root; children start at 1 (depth-first order).
type menuNode struct {
	id       int32
	item     MenuItem
	children []*menuNode
}

// linuxMenuState manages the D-Bus dbusmenu server for Linux AppMenu support.
// Lazily started when attachWindow is called and D-Bus is reachable.
// Both x11Platform and waylandPlatform hold one instance.
type linuxMenuState struct {
	mu        sync.Mutex
	rebuildMu sync.Mutex // serializes rebuildTree; prevents actions.Clear()+Store() interleave on concurrent set() calls
	items     []MenuItem // menu tree stored before window is ready
	revision  uint32     // incremented on each SetApplicationMenu
	root      *menuNode  // current dbusmenu ID tree; nil until first set()

	objPath string         // /com/gogpu/menu/window/<winid>
	winID   uint32         // X11 XID (0 on Wayland)
	window  PlatformWindow // set by platform after CreateWindow; powers window-role actions

	started bool
	stopCh  chan struct{}

	writeMu        sync.Mutex    // serializes all writes to the D-Bus connection
	serial         atomic.Uint32 // per-connection message serial; starts at 0x10000
	registerSerial atomic.Uint32 // serial of the RegisterWindow call; consumed on first reply in serve()
	actions        sync.Map      // map[int32]func() — dispatched on Event("clicked")

	conn *dbusConn // nil until tryStart succeeds
}

// newLinuxMenuState allocates a menu state. The D-Bus server is not started
// until attachWindow is called with a valid window.
func newLinuxMenuState() *linuxMenuState {
	m := &linuxMenuState{}
	m.serial.Store(0x10000) // avoid collision with the Hello serial (≈1)
	return m
}

// attachWindow is called after CreateWindow with the X11 window XID (or 0 for
// Wayland). Starts the dbusmenu server goroutine if the session bus is reachable.
//
// On Wayland (winID=0) we keep winID=0 for the RegisterWindow call so that KDE
// Plasma uses its "match by D-Bus sender PID" path: the registrar calls
// GetConnectionCredentials on the sender and finds the Wayland surface by PID.
// Passing a non-zero synthetic PID value as the winID causes KDE to treat it as
// an X11 XID lookup which always fails on a pure Wayland session.
// The object path still uses the process PID for uniqueness.
func (m *linuxMenuState) attachWindow(winID uint32) {
	if !hasDBusSession() {
		return
	}
	// pathID is used only for the D-Bus object path (must be non-zero and unique).
	// winID is kept as-is for RegisterWindow: 0 on Wayland, real XID on X11.
	pathID := winID
	if pathID == 0 {
		pathID = menuSyntheticWinID()
	}
	m.mu.Lock()
	m.winID = winID
	m.objPath = menuObjPrefix + strconv.FormatUint(uint64(pathID), 10)
	m.mu.Unlock()
	m.tryStart()
}

// tryStart connects to the session bus, calls RegisterWindow on the AppMenu
// registrar, and launches the serve goroutine. No-ops if already started or
// if the registrar cannot be reached.
func (m *linuxMenuState) tryStart() {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}

	conn, err := dbusConnect()
	if err != nil {
		m.mu.Unlock()
		logger().Debug("AppMenu: D-Bus unavailable, menu bar disabled", "err", err)
		return // D-Bus unavailable — Phase 1 no-op
	}

	m.conn = conn
	m.started = true
	m.stopCh = make(chan struct{})
	items := m.items
	objPath := m.objPath
	winID := m.winID
	m.mu.Unlock()

	logger().Info("AppMenu: D-Bus connected", "busName", conn.name, "winID", winID, "objPath", objPath)

	regSerial, err := m.doRegisterWindow(conn, winID, objPath)
	if err != nil {
		logger().Info("AppMenu: RegisterWindow failed — menu bar may not appear", "err", err)
	} else {
		m.registerSerial.Store(regSerial)
		logger().Info("AppMenu: RegisterWindow sent", "winID", winID, "objPath", objPath)
	}

	if len(items) > 0 {
		m.rebuildTree(items)
	}

	go m.serve()
}

// close stops the serve goroutine and closes the D-Bus connection.
// Safe to call multiple times or when the server was never started.
func (m *linuxMenuState) close() {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return
	}
	m.started = false
	stopCh := m.stopCh
	conn := m.conn
	m.conn = nil
	m.mu.Unlock()

	close(stopCh)
	if conn != nil {
		conn.rw.Close()
	}
}

// set replaces the current menu tree and notifies the desktop environment.
// If the tree structure and titles are unchanged and only Disabled flags differ,
// ItemsPropertiesUpdated is emitted for just the changed items — avoiding the
// full menu flash that LayoutUpdated causes. Otherwise LayoutUpdated is emitted.
// Safe to call from any goroutine, including before attachWindow.
func (m *linuxMenuState) set(items []MenuItem) {
	m.mu.Lock()
	m.items = items
	m.revision++
	rev := m.revision
	prevRoot := m.root // nil before first set()
	conn := m.conn
	objPath := m.objPath
	m.mu.Unlock()

	m.rebuildTree(items)

	if conn == nil {
		return
	}

	m.mu.Lock()
	newRoot := m.root
	m.mu.Unlock()

	if prevRoot != nil && hasSameTreeStructure(prevRoot, newRoot) {
		if changed := collectDisabledChanges(prevRoot, newRoot); len(changed) > 0 {
			m.emitItemsPropertiesUpdated(conn, objPath, changed)
		}
	} else {
		m.emitLayoutUpdated(conn, objPath, rev)
	}
}

// --- PlatMenuManager implementation ---

// SetApplicationMenu implements PlatMenuManager for x11Platform.
// Replaces the native application menu bar via D-Bus AppMenu.
func (p *x11Platform) SetApplicationMenu(items []MenuItem) {
	p.menu.set(items)
}

// AddToSystemMenu implements PlatMenuManager for x11Platform.
// Returns false: Linux has no Apple-style application menu.
func (p *x11Platform) AddToSystemMenu(_ SystemMenu, _ []MenuItem) bool {
	return false
}

// SetApplicationMenu implements PlatMenuManager for waylandPlatform.
// Replaces the native application menu bar via D-Bus AppMenu.
func (p *waylandPlatform) SetApplicationMenu(items []MenuItem) {
	p.menu.set(items)
}

// AddToSystemMenu implements PlatMenuManager for waylandPlatform.
// Returns false: Linux has no Apple-style application menu.
func (p *waylandPlatform) AddToSystemMenu(_ SystemMenu, _ []MenuItem) bool {
	return false
}

// Compile-time checks: both platform types must satisfy PlatMenuManager.
var _ PlatMenuManager = (*x11Platform)(nil)
var _ PlatMenuManager = (*waylandPlatform)(nil)

// --- Menu node tree ---

// rebuildTree replaces the dbusmenu ID tree from items, assigns sequential
// int32 IDs (depth-first, starting at 1), and re-registers all action callbacks.
// rebuildMu is held for the entire operation so that concurrent set() calls
// cannot interleave actions.Clear() with Store() calls from another rebuild.
func (m *linuxMenuState) rebuildTree(items []MenuItem) {
	m.rebuildMu.Lock()
	defer m.rebuildMu.Unlock()

	m.actions.Clear()
	var idGen int32
	root := &menuNode{id: menuRootID}
	root.children = buildMenuNodes(items, &idGen, &m.actions)
	m.registerRoleActions(root)
	m.mu.Lock()
	m.root = root
	m.mu.Unlock()
}

// registerRoleActions walks the tree and registers platform actions for all
// MenuRole values. Called after buildMenuNodes so that role-based items get
// platform-level behavior (close, minimize, etc.) in addition to any user
// Action that buildMenuNodes may have already stored.
func (m *linuxMenuState) registerRoleActions(node *menuNode) {
	if node.item.Role != MenuRoleNone {
		if fn := buildRoleAction(node.item.Role, node.item.Action, m.window); fn != nil {
			m.actions.Store(node.id, fn)
		}
	}
	for _, child := range node.children {
		m.registerRoleActions(child)
	}
}

// buildRoleAction returns the platform action for a menu role.
// ua is the user-supplied callback (may be nil); w is the platform window (may be nil).
// Returns nil for roles that have no platform operation and no user Action.
//
// Role mapping:
//   - Quit       → user Action + close window (os.Exit fallback when window is nil)
//   - Close      → user Action + close window
//   - Minimize   → user Action + minimize window
//   - Zoom       → user Action + maximize window
//   - FullScreen → user Action + toggle fullscreen
//   - Others     → user Action only (About, Preferences, Hide, etc.)
func buildRoleAction(role MenuRole, ua func(), w PlatformWindow) func() {
	switch role {
	case MenuRoleQuit:
		return makeQuitAction(ua, w)
	case MenuRoleClose:
		return makeCloseAction(ua, w)
	case MenuRoleMinimize:
		return makeMinimizeAction(ua, w)
	case MenuRoleZoom:
		return makeZoomAction(ua, w)
	case MenuRoleFullScreen:
		return makeFullScreenAction(ua, w)
	default:
		return ua // About, Preferences, etc.; nil when user provided no Action
	}
}

func makeQuitAction(ua func(), w PlatformWindow) func() {
	return func() {
		if ua != nil {
			ua()
		}
		if w != nil {
			w.Close()
		} else {
			os.Exit(0)
		}
	}
}

func makeCloseAction(ua func(), w PlatformWindow) func() {
	return func() {
		if ua != nil {
			ua()
		}
		if w != nil {
			w.Close()
		}
	}
}

func makeMinimizeAction(ua func(), w PlatformWindow) func() {
	return func() {
		if ua != nil {
			ua()
		}
		if w != nil {
			w.Minimize()
		}
	}
}

func makeZoomAction(ua func(), w PlatformWindow) func() {
	return func() {
		if ua != nil {
			ua()
		}
		if w != nil {
			w.Maximize()
		}
	}
}

func makeFullScreenAction(ua func(), w PlatformWindow) func() {
	return func() {
		if ua != nil {
			ua()
		}
		if w != nil {
			w.SetFullscreen(!w.IsFullscreen())
		}
	}
}

// buildMenuNodes recursively assigns IDs and registers actions for items.
func buildMenuNodes(items []MenuItem, idGen *int32, actions *sync.Map) []*menuNode {
	nodes := make([]*menuNode, 0, len(items))
	for _, item := range items {
		*idGen++
		node := &menuNode{id: *idGen, item: item}
		if len(item.Submenu) > 0 {
			node.children = buildMenuNodes(item.Submenu, idGen, actions)
		} else if item.Action != nil && !item.Separator {
			act := item.Action
			actions.Store(node.id, act)
		}
		nodes = append(nodes, node)
	}
	return nodes
}

// findNode returns the first node with the given ID in the subtree at node,
// or nil if not found.
func findNode(node *menuNode, id int32) *menuNode {
	if node == nil {
		return nil
	}
	if node.id == id {
		return node
	}
	for _, child := range node.children {
		if n := findNode(child, id); n != nil {
			return n
		}
	}
	return nil
}

// --- D-Bus server ---

// serve reads incoming METHOD_CALL messages on our object path and dispatches
// them. Exits when close() closes the connection. stopCh distinguishes an
// intentional shutdown from an unexpected read error so only the latter is logged.
func (m *linuxMenuState) serve() {
	m.mu.Lock()
	conn := m.conn
	objPath := m.objPath
	stopCh := m.stopCh
	m.mu.Unlock()

	for {
		msg, err := conn.readMsg()
		if err != nil {
			select {
			case <-stopCh:
				// intentional shutdown — conn.rw.Close() was called by close()
			default:
				logger().Debug("AppMenu: D-Bus connection lost", "err", err)
			}
			return
		}

		// One-shot: consume the RegisterWindow reply to log success or rejection.
		if regSerial := m.registerSerial.Load(); regSerial != 0 && msg.ReplyTo == regSerial {
			m.registerSerial.Store(0) // consume
			if msg.Type == dbusMsgError {
				logger().Info("AppMenu: RegisterWindow rejected by registrar — menu bar may not appear",
					"errorName", msg.ErrorName)
			} else {
				logger().Debug("AppMenu: RegisterWindow accepted")
			}
			continue
		}

		if msg.Type != dbusMsgCall || msg.Path != objPath {
			continue
		}
		m.handleCall(conn, msg)
	}
}

// handleCall dispatches an incoming dbusmenu METHOD_CALL and sends the reply.
func (m *linuxMenuState) handleCall(conn *dbusConn, msg *dbusMsg) {
	switch msg.Member {
	case "GetLayout":
		body, sig := m.handleGetLayout(msg.Body)
		m.sendReturn(conn, msg, sig, body)
	case "GetGroupProperties":
		body, sig := m.handleGetGroupProperties(msg.Body)
		m.sendReturn(conn, msg, sig, body)
	case "AboutToShow":
		body, sig := m.handleAboutToShow()
		m.sendReturn(conn, msg, sig, body)
	case "Event":
		m.handleEvent(msg.Body)
		m.sendReturn(conn, msg, "", nil)
	case "EventGroup":
		body, sig := m.handleEventGroup(msg.Body)
		m.sendReturn(conn, msg, sig, body)
	default:
		// Respond to unknown methods so the DE does not hang waiting for a reply.
		m.sendReturn(conn, msg, "", nil)
	}
}

// handleGetLayout decodes GetLayout(i parentId, i recursionDepth, as propertyNames)
// and returns the encoded response body and D-Bus signature "u(ia{sv}av)".
func (m *linuxMenuState) handleGetLayout(args []byte) ([]byte, string) {
	d := newMsgDecoder(args, 0)
	parentIDRaw, _ := d.readU32()
	parentID := int32(parentIDRaw)
	depthRaw, _ := d.readU32()
	depth := int32(depthRaw)
	// propertyNames (as) intentionally skipped — always return all properties.

	m.mu.Lock()
	rev := m.revision
	root := m.root
	m.mu.Unlock()

	if root == nil {
		root = &menuNode{id: menuRootID}
	}

	start := root
	if parentID != menuRootID {
		if n := findNode(root, parentID); n != nil {
			start = n
		}
	}

	b := newMsgBuf(0)
	b.u32(rev)
	encodeMenuLayout(b, start, int(depth))
	return b.data, dbusMenuLayoutSig
}

// handleGetGroupProperties decodes GetGroupProperties(ai ids, as propertyNames)
// and returns the encoded properties for each requested item ID.
func (m *linuxMenuState) handleGetGroupProperties(args []byte) ([]byte, string) {
	d := newMsgDecoder(args, 0)
	arrLen, _ := d.readU32()
	end := d.pos + int(arrLen)

	var ids []int32
	for d.pos < end {
		v, err := d.readU32()
		if err != nil {
			break
		}
		ids = append(ids, int32(v))
	}

	m.mu.Lock()
	root := m.root
	m.mu.Unlock()

	b := newMsgBuf(0)
	lp, cp := b.arrayStart(8) // a(ia{sv}): struct alignment = 8
	for _, id := range ids {
		node := findNode(root, id)
		if node == nil {
			continue
		}
		b.padTo(8)
		b.u32(uint32(node.id))
		plp, pcp := b.arrayStart(8)
		encodeItemProps(b, &node.item, node.id)
		b.arrayEnd(plp, pcp)
	}
	b.arrayEnd(lp, cp)
	return b.data, dbusMenuPropsSig
}

// handleAboutToShow returns (b false) — the menu layout is always current and
// does not need a DE-triggered refresh before display.
func (m *linuxMenuState) handleAboutToShow() ([]byte, string) {
	b := newMsgBuf(0)
	b.bool32(false)
	return b.data, "b"
}

// handleEvent decodes Event(i id, s eventId, v data, u timestamp) and
// dispatches the registered action when eventId is "clicked".
func (m *linuxMenuState) handleEvent(args []byte) {
	d := newMsgDecoder(args, 0)
	idRaw, err := d.readU32()
	if err != nil {
		return
	}
	eventID, err := d.readStr()
	if err != nil {
		return
	}
	if eventID != dbusMenuEventClicked {
		return
	}
	if fn, ok := m.actions.Load(int32(idRaw)); ok {
		go fn.(func())()
	}
}

// handleEventGroup decodes EventGroup(a(isvu) events), dispatches "clicked"
// actions, and returns (ai idErrors) — always empty on success.
func (m *linuxMenuState) handleEventGroup(args []byte) ([]byte, string) {
	d := newMsgDecoder(args, 0)
	arrLen, err := d.readU32()
	if err == nil {
		_ = d.alignTo(8) // (isvu) struct alignment = 8
		end := d.pos + int(arrLen)
		for d.pos < end {
			_ = d.alignTo(8)
			idRaw, e1 := d.readU32()
			eventID, e2 := d.readStr()
			if e1 != nil || e2 != nil {
				break
			}
			vsig, _ := d.readSig()
			_ = d.skipValue(vsig)
			_, _ = d.readU32() // timestamp (u)

			if eventID == dbusMenuEventClicked {
				if fn, ok := m.actions.Load(int32(idRaw)); ok {
					go fn.(func())()
				}
			}
		}
	}

	b := newMsgBuf(0)
	lp, cp := b.arrayStart(4) // ai: empty error list
	b.arrayEnd(lp, cp)
	return b.data, "ai"
}

// --- Encoding helpers ---

// encodeMenuLayout encodes a single menuNode as a dbusmenu (ia{sv}av) struct.
// depth controls how many child levels are included: -1 = unlimited, 0 = node only.
func encodeMenuLayout(b *msgBuf, node *menuNode, depth int) {
	b.padTo(8)             // (ia{sv}av) struct alignment
	b.u32(uint32(node.id)) // i: item ID (int32 wire-compatible with uint32)

	// a{sv} properties
	lp, cp := b.arrayStart(8) // dict-entry {sv} alignment = 8
	if node.id == menuRootID {
		writeStrProp(b, "children-display", "submenu")
	} else {
		encodeItemProps(b, &node.item, node.id)
	}
	b.arrayEnd(lp, cp)

	// av children (each child is a variant wrapping another (ia{sv}av))
	childLp, childCp := b.arrayStart(1) // variant alignment = 1 per D-Bus spec
	if depth != 0 {
		nextDepth := depth - 1
		if depth < 0 {
			nextDepth = -1
		}
		for _, child := range node.children {
			b.sig("(ia{sv}av)")                   // variant type signature
			encodeMenuLayout(b, child, nextDepth) // struct content (padTo(8) inside)
		}
	}
	b.arrayEnd(childLp, childCp)
}

// encodeItemProps writes the a{sv} properties for a non-root dbusmenu item.
func encodeItemProps(b *msgBuf, item *MenuItem, _ int32) {
	if item.Separator {
		writeStrProp(b, "type", "separator")
		return
	}
	writeStrProp(b, "label", item.Title)
	writeBoolProp(b, "enabled", !item.Disabled)
	writeBoolProp(b, "visible", true)
	if len(item.Submenu) > 0 {
		writeStrProp(b, "children-display", "submenu")
	}
}

// writeStrProp encodes a {sv} dict entry with a string value.
func writeStrProp(b *msgBuf, key, value string) {
	b.padTo(8)          // {sv} struct alignment
	b.str(key)          // s key
	b.variantStr(value) // v(s) value
}

// writeBoolProp encodes a {sv} dict entry with a boolean value.
func writeBoolProp(b *msgBuf, key string, value bool) {
	b.padTo(8)           // {sv} struct alignment
	b.str(key)           // s key
	b.variantBool(value) // v(b) value
}

// busNameAndPath returns the D-Bus unique bus name and dbusmenu object path
// after a successful connection, or empty strings if not yet connected.
func (m *linuxMenuState) busNameAndPath() (busName, objPath string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.conn == nil {
		return "", ""
	}
	return m.conn.name, m.objPath
}

// menuSyntheticWinID returns a non-zero window identifier for Wayland sessions
// where no X11 XID is available. Uses the lower 32 bits of the process PID,
// which is unique per process and accepted by com.canonical.AppMenu.Registrar.
// KDE Plasma on Wayland correlates AppMenu registrations by D-Bus sender name,
// so the numeric window ID only needs to be non-zero and stable.
func menuSyntheticWinID() uint32 {
	return uint32(os.Getpid())
}

// --- D-Bus message encoding ---

// nextSerial returns the next unique serial for outgoing messages on this connection.
func (m *linuxMenuState) nextSerial() uint32 {
	return m.serial.Add(1)
}

// write sends raw bytes to the D-Bus connection, serialized across goroutines.
func (m *linuxMenuState) write(conn *dbusConn, data []byte) error {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	_, err := conn.rw.Write(data)
	return err
}

// sendReturn encodes and sends a D-Bus METHOD_RETURN for the given incoming call.
// The DESTINATION field is set to msg.Sender so the D-Bus daemon can route the
// reply back to the caller — without it the daemon drops the message and the
// caller times out waiting for a response.
func (m *linuxMenuState) sendReturn(conn *dbusConn, msg *dbusMsg, sig string, body []byte) {
	if err := m.write(conn, menuEncodeReturn(m.nextSerial(), msg.Serial, msg.Sender, sig, body)); err != nil {
		logger().Debug("AppMenu: write failed", "op", "sendReturn", "err", err)
	}
}

// hasSameTreeStructure reports whether a and b have identical tree shape:
// same children count at each level, same Separator flag, and same Title.
// Used to decide whether only Disabled flags changed (→ ItemsPropertiesUpdated)
// or the full structure changed (→ LayoutUpdated).
func hasSameTreeStructure(a, b *menuNode) bool {
	if a == nil || b == nil {
		return a == b
	}
	if len(a.children) != len(b.children) {
		return false
	}
	if a.id != menuRootID {
		if a.item.Separator != b.item.Separator {
			return false
		}
		if a.item.Title != b.item.Title {
			return false
		}
	}
	for i := range a.children {
		if !hasSameTreeStructure(a.children[i], b.children[i]) {
			return false
		}
	}
	return true
}

// collectDisabledChanges returns nodes from next whose Disabled flag differs
// from the corresponding node in prev. Both trees must have the same structure
// (verified by hasSameTreeStructure before calling). Separators are skipped
// because they have no "enabled" property in the dbusmenu protocol.
func collectDisabledChanges(prev, next *menuNode) []*menuNode {
	var changed []*menuNode
	if next.id != menuRootID && !next.item.Separator && prev.item.Disabled != next.item.Disabled {
		changed = append(changed, next)
	}
	for i := range next.children {
		changed = append(changed, collectDisabledChanges(prev.children[i], next.children[i])...)
	}
	return changed
}

// buildItemsPropsBody encodes the body for a com.canonical.dbusmenu.ItemsPropertiesUpdated
// signal: a(ia{sv}) updatedProps + a(ias) removedProps (always empty).
// Only the "enabled" property is included — this signal is used exclusively for
// Disabled flag live sync; structural changes use LayoutUpdated instead.
func buildItemsPropsBody(nodes []*menuNode) []byte {
	b := newMsgBuf(0)
	lp, cp := b.arrayStart(8) // a(ia{sv}): struct alignment = 8
	for _, node := range nodes {
		b.padTo(8)
		b.u32(uint32(node.id))
		plp, pcp := b.arrayStart(8) // a{sv}: dict-entry alignment = 8
		writeBoolProp(b, "enabled", !node.item.Disabled)
		b.arrayEnd(plp, pcp)
	}
	b.arrayEnd(lp, cp)
	lp2, cp2 := b.arrayStart(8) // a(ias): empty removed props, struct alignment = 8
	b.arrayEnd(lp2, cp2)
	return b.data
}

// emitItemsPropertiesUpdated sends a com.canonical.dbusmenu.ItemsPropertiesUpdated
// signal for nodes whose Disabled flag changed. The DE updates individual items
// without triggering a full LayoutUpdated reload, preventing menu flickering.
func (m *linuxMenuState) emitItemsPropertiesUpdated(conn *dbusConn, objPath string, nodes []*menuNode) {
	if err := m.write(conn, menuEncodeSignal(
		m.nextSerial(), objPath, dbusMenuIface,
		"ItemsPropertiesUpdated", "a(ia{sv})a(ias)", buildItemsPropsBody(nodes),
	)); err != nil {
		logger().Debug("AppMenu: write failed", "op", "emitItemsPropertiesUpdated", "err", err)
	}
}

// emitLayoutUpdated sends a com.canonical.dbusmenu.LayoutUpdated signal (ui)
// with the new revision and parent=0 (root changed).
func (m *linuxMenuState) emitLayoutUpdated(conn *dbusConn, objPath string, rev uint32) {
	b := newMsgBuf(0)
	b.u32(rev) // u: revision
	b.u32(0)   // i: parent ID (0 = root changed)
	if err := m.write(conn, menuEncodeSignal(m.nextSerial(), objPath, dbusMenuIface, "LayoutUpdated", "ui", b.data)); err != nil {
		logger().Debug("AppMenu: write failed", "op", "emitLayoutUpdated", "err", err)
	}
}

// doRegisterWindow sends com.canonical.AppMenu.Registrar.RegisterWindow(u, o)
// to associate our dbusmenu object path with the given X11 window ID.
// Returns the message serial used for the call so serve() can match the reply.
func (m *linuxMenuState) doRegisterWindow(conn *dbusConn, winID uint32, objPath string) (uint32, error) {
	b := newMsgBuf(0)
	b.u32(winID)   // u: windowId (X11 XID; 0 on Wayland)
	b.str(objPath) // o: menuObjectPath
	serial := m.nextSerial()
	raw := menuEncodeCall(serial, appMenuDest, appMenuPath, appMenuIface, "RegisterWindow", "uo", b.data)
	return serial, m.write(conn, raw)
}

// menuEncodeReturn encodes a D-Bus METHOD_RETURN message.
// dest is the Sender from the original call (required for daemon routing).
// replySerial is the serial of the incoming call being answered.
func menuEncodeReturn(serial, replySerial uint32, dest, sig string, body []byte) []byte {
	hdr := newMsgBuf(16)
	dbusWriteHdrField(hdr, dbusFieldReplySerial, "u", func() { hdr.u32(replySerial) })
	if dest != "" {
		dbusWriteHdrField(hdr, dbusFieldDest, "s", func() { hdr.str(dest) })
	}
	if sig != "" {
		dbusWriteHdrField(hdr, dbusFieldSignature, "g", func() { hdr.sig(sig) })
	}
	return dbusAssembleMsg(dbusMsgReturn, 0, serial, hdr.data, body)
}

// menuEncodeSignal encodes a D-Bus SIGNAL message.
func menuEncodeSignal(serial uint32, path, iface, member, sig string, body []byte) []byte {
	hdr := newMsgBuf(16)
	dbusWriteHdrField(hdr, dbusFieldPath, "o", func() { hdr.str(path) })
	dbusWriteHdrField(hdr, dbusFieldInterface, "s", func() { hdr.str(iface) })
	dbusWriteHdrField(hdr, dbusFieldMember, "s", func() { hdr.str(member) })
	if sig != "" {
		dbusWriteHdrField(hdr, dbusFieldSignature, "g", func() { hdr.sig(sig) })
	}
	return dbusAssembleMsg(dbusMsgSignal, 0, serial, hdr.data, body)
}

// menuEncodeCall encodes a D-Bus METHOD_CALL message (used for RegisterWindow).
// Flags are 0 (no NO_REPLY_EXPECTED) so the registrar sends a METHOD_RETURN or
// ERROR that serve() can consume to log the registration result.
func menuEncodeCall(serial uint32, dest, path, iface, member, sig string, body []byte) []byte {
	hdr := newMsgBuf(16)
	dbusWriteHdrField(hdr, dbusFieldPath, "o", func() { hdr.str(path) })
	dbusWriteHdrField(hdr, dbusFieldInterface, "s", func() { hdr.str(iface) })
	dbusWriteHdrField(hdr, dbusFieldMember, "s", func() { hdr.str(member) })
	dbusWriteHdrField(hdr, dbusFieldDest, "s", func() { hdr.str(dest) })
	if sig != "" {
		dbusWriteHdrField(hdr, dbusFieldSignature, "g", func() { hdr.sig(sig) })
	}
	return dbusAssembleMsg(dbusMsgCall, 0, serial, hdr.data, body)
}
