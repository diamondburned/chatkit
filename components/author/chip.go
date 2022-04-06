package author

import (
	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
)

// Chip describes a user chip. It is used for display in messages and for
// composing messages in the composer bar.
//
// In Material Design, chips are described as "compact elements tha represen an
// input, attribute, or action." People who have used Google Mail before will
// have seen it when they input an email address into the "To" field and having
// it turn into a small box showing the user information in a friendlier way
// instead of being an email address.
//
// Note that due to how chips are currently implemented, it only works well at
// certain font scale ranges. Once the font scale is beyond ~1.3, flaws will
// start to be very apparent. In that case, the user should use proper graphics
// scaling using Wayland, not using hacks like font scaling.
type Chip struct {
	*gtk.Box
	Avatar *onlineimage.Avatar
	Name   *gtk.Label

	ctx   context.Context
	color string
	css   struct {
		last *gtk.CSSProvider
	}
}

var chipCSS = cssutil.Applier("mauthor-chip", `
	.mauthor-chip {
		border-radius: 9999px 9999px;
		margin-bottom: -0.45em;
	}
	.mauthor-chip-unpadded {
		margin-bottom: 0;
	}
	.mauthor-chip-colored {
		background-color: transparent; /* override custom CSS */
	}
	/*
     * Workaround for GTK padding an extra line at the bottom of the TextView if
	 * even one widget is inserted for some weird reason.
     */
	.mauthor-haschip {
		margin-bottom: -1em;
	}
`)

const ChipAvatarSize = 20 // smaller than usual

// NewChip creates a new Chip widget.
func NewChip(ctx context.Context, avatarProvider imgutil.Provider) *Chip {
	c := Chip{ctx: ctx}

	c.Name = gtk.NewLabel("")
	c.Name.AddCSSClass("mauthor-chip-colored")
	c.Name.SetXAlign(0.4) // account for the right round corner

	c.Avatar = onlineimage.NewAvatar(ctx, avatarProvider, ChipAvatarSize)
	c.Avatar.ConnectLabel(c.Name)

	c.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	c.Box.SetOverflow(gtk.OverflowHidden)
	c.Box.Append(c.Avatar)
	c.Box.Append(c.Name)
	chipCSS(c)

	gtkutil.OnFirstDrawUntil(c.Name, func() bool {
		// Update the avatar size using the Label's height for consistency. From
		// my experiments, the Label's height is 21, so 21 or 22 would've
		// sufficed, but we're doing this just to make sure the chip is only as
		// tall as it needs to be.
		h := c.Name.AllocatedHeight()
		if h < 1 {
			return true
		}
		c.Avatar.SetSizeRequest(h)
		return false
	})

	return &c
}

// Unpad removes the negative margin in the Chip.
func (c *Chip) Unpad() {
	c.AddCSSClass("mauthor-chip-unpadded")
}

// InsertText inserts the chip into the given TextView at the given TextIter.
// The inserted anchor is returned.
func (c *Chip) InsertText(text *gtk.TextView, iter *gtk.TextIter) *gtk.TextChildAnchor {
	buffer := text.Buffer()
	buffer.Insert(iter, "\u200b")

	anchor := buffer.CreateChildAnchor(iter)
	text.AddChildAtAnchor(c, anchor)

	text.AddCSSClass("mauthor-haschip")
	text.QueueResize()

	return anchor
}

// SetAvatar calls c.Avatar.SetFromURL.
func (c *Chip) SetAvatar(url string) {
	c.Avatar.SetFromURL(url)
}

const maxChipWidth = 200

// SetName sets the username.
func (c *Chip) SetName(label string) {
	c.Name.SetEllipsize(pango.EllipsizeNone)
	c.Name.SetText(label)

	// Properly limit the size of the label.
	layout := c.Name.Layout()

	width, _ := layout.PixelSize()
	width += 8 // padding

	if width > maxChipWidth {
		width = maxChipWidth
	}

	c.Name.SetSizeRequest(width, -1)
	c.Name.SetEllipsize(pango.EllipsizeEnd)
}

func (c *Chip) colorWidgets() []gtk.Widgetter {
	return []gtk.Widgetter{
		c.Name, c.Box,
	}
}

// customChipCSSf is the CSS fmt string that's specific to each user. The 0.8 is
// taken from the 0x33 alpha: 0x33/0xFF = 0.2.
const customChipCSSf = `
	box {
		background-color: mix(%[1]s, @theme_bg_color, 0.8);
	}
	label {
		color: %[1]s;
	}
`

// SetColor sets the chip's color in a hexadecimal string #FFFFFF.
func (c *Chip) SetColor(color string) {
	colorWidgets := c.colorWidgets()

	if c.css.last != nil {
		for _, w := range colorWidgets {
			s := gtk.BaseWidget(w).StyleContext()
			s.RemoveProvider(c.css.last)
		}
	}

	c.css.last = gtk.NewCSSProvider()
	c.css.last.LoadFromData(fmt.Sprintf(customChipCSSf, color))

	for _, w := range colorWidgets {
		s := gtk.BaseWidget(w).StyleContext()
		s.AddProvider(c.css.last, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	}
}
