package block

import (
	"strconv"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

// ListIndex is a list item index.
type ListIndex struct {
	// Level is the depth of the list the item is in.
	Level int
	// Index is the index of the list item.
	Index int
	// Unordered is true if the list is unordered.
	Unordered bool
}

// ListItemBlock is a text block that contains a list item.
type ListItemBlock struct {
	*gtk.Box
	ListIndex ListIndex

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
func NewListItemBlock(state *ContainerState, listIndex ListIndex) *ListItemBlock {
	text := NewTextBlock(state)

	bullet := gtk.NewLabel("")
	bullet.SetHExpand(false)
	bullet.SetVExpand(false)
	bullet.SetVAlign(gtk.AlignStart)
	bullet.SetMarginStart(listIndex.Level*6 - 6)
	bulletCSS(bullet)

	if listIndex.Unordered {
		bullet.SetText("•")
		bullet.AddCSSClass("md-listitem-unordered")
	} else {
		bullet.SetText(strconv.Itoa(listIndex.Index) + ".")
		bullet.AddCSSClass("md-listitem-ordered")
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
