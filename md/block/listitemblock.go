package block

import (
	"strconv"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

// ListItemBlock is a text block that contains a list item.
type ListItemBlock struct {
	*gtk.Box
	ListIndex *int // nil if unordered

	text *TextBlock
}

var bulletCSS = cssutil.Applier("md-listitem-bullet", `
	.md-listitem-bullet {
		margin-right: 0.5em;
		opacity: 0.85;
	}
	.md-listitem-unordered {
		font-size: 1.2em;
	}
`)

// NewListItemBlock creates a new ListItemBlock.
func NewListItemBlock(state *ContainerState, listIndex *int, offset int) *ListItemBlock {
	text := NewTextBlock(state)

	bullet := gtk.NewLabel("")
	bullet.SetHExpand(false)
	bullet.SetVExpand(false)
	bullet.SetVAlign(gtk.AlignStart)
	bullet.SetMarginStart(offset*6 - 6)
	bulletCSS(bullet)

	if listIndex != nil {
		bullet.SetText(strconv.Itoa(*listIndex) + ".")
		bullet.AddCSSClass("md-listitem-ordered")
	} else {
		bullet.SetText("•")
		bullet.AddCSSClass("md-listitem-unordered")
	}

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.AddCSSClass("md-listitem")
	box.Append(bullet)
	box.Append(text.TextView)

	return &ListItemBlock{
		Box:  box,
		text: text,
	}
}

// TextBlock returns the underlying TextBlock.
func (b *ListItemBlock) TextBlock() *TextBlock {
	return b.text
}
