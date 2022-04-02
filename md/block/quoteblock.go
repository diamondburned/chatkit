package block

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

// Blockquote is a widget block that boxes another list of widget blocks. To use
// it, create a new Blockquote using NewBlockquote with the current
// ContainerState, then use Blockquote.State to walk further.
type Blockquote struct {
	*gtk.Box
	State *ContainerState
}

var quoteBlockCSS = cssutil.Applier("md-blockquote", `
	.md-blockquote {
		border-left:  3px solid alpha(@theme_fg_color, 0.5);
		padding-left: 5px;
	}
	.md-blockquote:not(:last-child) {
		margin-bottom: 3px;
	}
	.md-blockquote > textview.mauthor-haschip {
		margin-bottom: -1em;
	}
`)

// NewBlockquote creates a new Blockquote.
func NewBlockquote(current *ContainerState) *Blockquote {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.SetOverflow(gtk.OverflowHidden)

	quote := Blockquote{
		Box:   box,
		State: current.WithParent(box),
	}
	quoteBlockCSS(quote)
	return &quote
}
