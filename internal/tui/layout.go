package tui

const (
	headerLines  = 2  // slim header (1) + tab bar (1)
	footerLines  = 1  // footer hint bar
	sidebarWidth = 22 // chars, fixed
	rightWidth   = 36 // chars, fixed
	minWidth     = 80
	minHeight    = 24
)

// PanelLayout holds the computed dimensions for all panels.
type PanelLayout struct {
	Width  int
	Height int

	HeaderHeight  int
	FooterHeight  int
	ContentHeight int // Height - HeaderHeight - FooterHeight

	HasSidebar   bool
	SidebarWidth int

	HasRight   bool
	RightWidth int

	CenterWidth int // Width - SidebarWidth (if shown) - RightWidth (if shown)
}

// ComputeLayout returns panel dimensions for a given terminal size.
func ComputeLayout(width, height int) PanelLayout {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	l := PanelLayout{
		Width:         width,
		Height:        height,
		HeaderHeight:  headerLines,
		FooterHeight:  footerLines,
		ContentHeight: height - headerLines - footerLines,
	}
	if l.ContentHeight < 0 {
		l.ContentHeight = 0
	}
	// Sidebar visible when terminal is wide enough
	if width >= 100 {
		l.HasSidebar = true
		l.SidebarWidth = sidebarWidth
	}
	// Right panel visible when terminal is wide enough
	if width >= 140 {
		l.HasRight = true
		l.RightWidth = rightWidth
	}
	used := 0
	if l.HasSidebar {
		used += l.SidebarWidth
	}
	if l.HasRight {
		used += l.RightWidth
	}
	l.CenterWidth = width - used
	if l.CenterWidth < 20 {
		l.CenterWidth = 20
	}
	return l
}
