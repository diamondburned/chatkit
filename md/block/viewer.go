package block

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// Viewer is a widget that renders a Markdown AST node into widgets. All widgets
// within the viewer are strictly immutable. A Viewer itself is a
// ContainerWidgetBlock.
type Viewer struct {
	*gtk.Box
	table *gtk.TextTagTable
	state *ContainerState
	ctx   context.Context
}

var (
	_ WidgetBlock          = (*Viewer)(nil)
	_ ContainerWidgetBlock = (*Viewer)(nil)
)

func newContainerBox() *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	return box
}

// NewViewer creates a new Markdown viewer.
func NewViewer(ctx context.Context) *Viewer {
	v := Viewer{
		ctx:   ctx,
		table: gtk.NewTextTagTable(),
	}
	v.Box = newContainerBox()
	v.state = newContainerState(&v, v.Box)
	return &v
}

// State returns the Viewer's ContainerState. It implements
// ContainerWidgetBlock.
func (v *Viewer) State() *ContainerState {
	return v.state
}

// TagTable returns the viewer's shared TextTagTable.
func (v *Viewer) TagTable() *gtk.TextTagTable {
	return v.table
}

// SetExtraMenu sets the given menu for all children widget nodes.
func (v *Viewer) SetExtraMenu(model gio.MenuModeller) {
	v.state.Walk(func(w WidgetBlock) bool {
		switch w := w.(type) {
		case TextWidgetBlock:
			w.TextBlock().SetExtraMenu(model)
		case interface{ SetExtraMenu(model gio.MenuModeller) }:
			w.SetExtraMenu(model)
		case *CodeBlock:
			w.text.SetExtraMenu(model)
		}
		return false
	})
}
