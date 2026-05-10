package tui

const (
	headerLines  = 1  // slim header
	footerLines  = 1  // footer hint bar
	sidebarWidth = 24 // left panel, fixed
	minWidthWide = 100
	minWidth     = 80
	minHeight    = 24
)

type PanelLayout struct {
	Width  int
	Height int

	ContentHeight int // Height - headerLines - footerLines

	HasSidebar   bool
	SidebarWidth int
	DetailWidth  int // Width - SidebarWidth (when sidebar shown)
}

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
		ContentHeight: height - headerLines - footerLines,
	}
	if l.ContentHeight < 0 {
		l.ContentHeight = 0
	}
	if width >= minWidthWide {
		l.HasSidebar = true
		l.SidebarWidth = sidebarWidth
		l.DetailWidth = width - sidebarWidth
		if l.DetailWidth < 30 {
			l.DetailWidth = 30
		}
	} else {
		l.DetailWidth = width
	}
	return l
}
