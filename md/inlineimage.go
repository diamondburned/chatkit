package md

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

// inlineImageHeightOffset is kept in sync with the -0.35em subtraction above,
// because GTK behaves weirdly with how the height is done. It only matters for
// small inline images, though.
const inlineImageHeightOffset = -4

// InlineImage is an inline image. The actual widget type depends on the
// constructor.
type InlineImage struct {
	gtk.Widgetter
}

// SetSizeRequest sets the minimum size of the inline image.
func (i *InlineImage) SetSizeRequest(w, h int) {
	// h += inlineImageHeightOffset
	gtk.BaseWidget(i).SetSizeRequest(w, h)
}

var inlineImageCSS = cssutil.Applier("md-inlineimage", `
	.md-inlineimage {
		margin-bottom: -0.45em;
	}
`)

// InsertImageWidget asynchronously inserts a new image widget. It does so in a
// way that the text position of the text buffer is not scrambled. Images
// created using this function will have the ".md-inlineimage" class.
func InsertImageWidget(view *gtk.TextView, anchor *gtk.TextChildAnchor) *InlineImage {
	image := gtk.NewImageFromIconName("image-x-generic-symbolic")
	return InsertCustomImageWidget(view, anchor, image)
}

// InsertCustomImageWidget is the custom variant of InsertImageWidget.
func InsertCustomImageWidget(view *gtk.TextView, anchor *gtk.TextChildAnchor, imager gtk.Widgetter) *InlineImage {
	image := gtk.BaseWidget(imager)
	inlineImageCSS(image)

	fixTextHeight(view, image)

	view.AddChildAtAnchor(image, anchor)
	view.AddCSSClass("md-hasimage")

	return &InlineImage{imager}
}

func fixTextHeight(view *gtk.TextView, image *gtk.Widget) {
	for _, class := range view.CSSClasses() {
		if class == "md-hasimage" {
			return
		}
	}

	gtkutil.OnFirstDrawUntil(view, func() bool {
		h := image.AllocatedHeight()
		if h < 1 {
			return true
		}

		// Workaround to account for GTK's weird height allocating when a widget
		// is added. We're removing most of the excess empty padding with this.
		h = h * 95 / 100
		cssutil.Applyf(view, `* { margin-bottom: -%dpx; }`, h)

		return false
	})
}
