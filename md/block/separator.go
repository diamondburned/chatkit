package block

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

// SeparatorBlock is a horizontal line block.
type SeparatorBlock struct {
	*gtk.Separator
}

// NewSeparatorBlock creates a new SeparatorBlock.
func NewSeparatorBlock() *SeparatorBlock {
	sep := gtk.NewSeparator(gtk.OrientationHorizontal)
	sep.AddCSSClass("mcontent-separator-block")
	return &SeparatorBlock{sep}
}
