package thumbnail

import (
	"context"
	"html"
	"mime"
	"path"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
)

// EmbedType indicates the type of the Embed being constructed. The type
// determines how it's displayed visually to the user.
type EmbedType uint8

const (
	EmbedTypeImage EmbedType = iota
	EmbedTypeVideo
	EmbedTypeGIF
)

// IsGIF returns true if the URL is a GIF URL.
func IsGIF(url string) bool {
	return path.Ext(url) == ".gif"
}

// TypeFromURL returns the EmbedType from the URL.
func TypeFromURL(url string) EmbedType {
	mime := mime.TypeByExtension(path.Ext(url))
	if mime == "" {
		return EmbedTypeImage // dunno
	}

	if mime == "image/gif" {
		return EmbedTypeGIF
	}

	if strings.HasPrefix(mime, "video/") {
		return EmbedTypeVideo
	}

	return EmbedTypeImage
}

// EmbedOpts contains options for Embed.
type EmbedOpts struct {
	// Type is the embed type. Default is Image.
	Type EmbedType
	// Provider is the image provider to use. Default is HTTPProvider.
	Provider imgutil.Provider
	// Whole, if true, will make errors show in its full information instead of
	// being hidden behind an error icon. Use this for messages only.
	Whole bool
	// CanHide, if true, will make the image hide itself on error. Use this for
	// anything not important, like embeds.
	CanHide bool
}

// Embed is a user-clickable image with an open callback.
type Embed struct {
	*gtk.Button
	Image *onlineimage.Picture

	openURL func()
	name    string

	curSize [2]int
	maxSize [2]int
	opts    EmbedOpts
}

var embedCSS = cssutil.Applier("thumbnail-embed", `
	.thumbnail-embed {
		padding: 0;
		margin:  0;
		/* margin-left: -2px; */
		/* border:  2px solid transparent; */
		transition-duration: 150ms;
		transition-property: all;
	}
	.thumbnail-embed,
	.thumbnail-embed:hover {
		background: none;
	}
	.thumbnail-embed:hover {
		/* border: 2px solid @theme_selected_bg_color; */
	}
	.thumbnail-embed .thumbnail-embed-image {
		background-color: black;
		transition: linear 50ms filter;
	}
	.thumbnail-embed:not(.thumbnail-embed-interactive):hover .thumbnail-embed-image {
		filter: contrast(80%) brightness(80%);
	}
	.thumbnail-embed-errorlabel {
		color: @error_color;
		padding: 4px;
	}
	.thumbnail-embed-play {
		background-color: alpha(@theme_bg_color, 0.85);
		border-radius: 999px;
		padding: 8px;
	}
	.thumbnail-embed:hover  .thumbnail-embed-play,
	.thumbnail-embed:active .thumbnail-embed-play {
		background-color: @theme_selected_bg_color;
	}
	.thumbnail-embed-gifmark {
		background-color: alpha(white, 0.85);
		color: black;
		padding: 0px 4px;
		margin:  4px;
		border-radius: 8px;
		font-weight: bold;
	}
	.message-normalembed-body:not(:only-child) {
		margin-right: 6px;
	}
`)

// NewEmbed creates a thumbnail Embed.
func NewEmbed(ctx context.Context, maxW, maxH int, opts EmbedOpts) *Embed {
	if opts.Provider == nil {
		opts.Provider = imgutil.HTTPProvider
	}

	e := &Embed{
		maxSize: [2]int{maxW, maxH},
		opts:    opts,
	}

	ctx = imgutil.WithOpts(ctx,
		imgutil.WithErrorFn(e.onError),
		// imgutil.WithRescale(maxW, maxH),
	)

	e.Image = onlineimage.NewPicture(ctx, opts.Provider)
	e.Image.AddCSSClass("thumbnail-embed-image")
	e.Image.SetLayoutManager(gtk.NewConstraintLayout()) // magically left aligned
	e.Image.SetCanFocus(false)
	e.Image.SetCanShrink(true)
	e.Image.SetKeepAspectRatio(true)

	e.Button = gtk.NewButton()
	e.Button.SetOverflow(gtk.OverflowHidden)
	e.Button.SetHAlign(gtk.AlignStart)
	e.Button.SetHasFrame(false)
	e.Button.SetCanTarget(false)
	e.Button.ConnectClicked(func() { e.openURL() })
	embedCSS(e)

	if opts.Type == EmbedTypeImage {
		e.Button.AddCSSClass("thumbnail-embed-typeimage")
		e.Button.SetChild(e.Image)
	} else {
		overlay := gtk.NewOverlay()
		overlay.SetChild(e.Image)
		overlay.AddCSSClass("thumbnail-embed-overlay")
		e.Button.SetChild(overlay)

		switch opts.Type {
		case EmbedTypeVideo:
			e.Button.AddCSSClass("thumbnail-embed-typevideo")

			play := gtk.NewImageFromIconName("media-playback-start-symbolic")
			play.AddCSSClass("thumbnail-embed-play")
			play.SetHAlign(gtk.AlignCenter)
			play.SetVAlign(gtk.AlignCenter)
			play.SetIconSize(gtk.IconSizeLarge)

			overlay.AddOverlay(play)

		case EmbedTypeGIF:
			e.Button.AddCSSClass("thumbnail-embed-typegif")

			gif := gtk.NewLabel("GIF")
			gif.AddCSSClass("thumbnail-embed-gifmark")
			gif.SetCanTarget(false)
			gif.SetVAlign(gtk.AlignStart) // top
			gif.SetHAlign(gtk.AlignEnd)   // right

			overlay.AddOverlay(gif)

			if onlineimage.CanAnimate {
				e.Button.AddCSSClass("thumbnail-embed-interactive")

				motion := gtk.NewEventControllerMotion()
				e.Button.AddController(motion)

				anim := e.Image.EnableAnimation()

				// Show or hide the GIF icon while it's playing.
				motion.ConnectEnter(func(x, y float64) {
					anim.Start()
					gif.Hide()
				})
				motion.ConnectLeave(func() {
					anim.Stop()
					gif.Show()
				})
			}
		}
	}

	e.Image.NotifyProperty("paintable", func() {
		if p := e.Image.Paintable(); p != nil {
			e.SetSize(p.IntrinsicWidth(), p.IntrinsicHeight())
			e.finishSetting()
		}
	})

	return e
}

// SetName sets the given embed name into everything that's displaying the embed
// name.
func (e *Embed) SetName(name string) {
	e.name = name
	if !e.Button.HasCSSClass("thumbnail-embed-interactive") {
		e.Button.SetTooltipText(name)
	}
}

// SetFromURL sets the URL of the thumbnail embed.
func (e *Embed) SetFromURL(url string) {
	e.Image.SetURL(url)
}

// undo effects
func (e *Embed) finishSetting() {
	if e.opts.CanHide {
		e.Show()
	}

	if e.opts.Whole {
		e.Button.SetChild(e.Image)
	}
}

func (e *Embed) onError(err error) {
	if e.opts.CanHide {
		e.Hide()
		return
	}

	if e.opts.Whole {
		// Mild annoyance: the padding of this label actually grows the image a
		// bit. Not sure how to fix it.
		errLabel := gtk.NewLabel("Error fetching image: " + html.EscapeString(err.Error()))
		errLabel.AddCSSClass("mcontent-image-errorlabel")
		errLabel.SetEllipsize(pango.EllipsizeEnd)
		errLabel.SetWrap(true)
		errLabel.SetWrapMode(pango.WrapWordChar)
		errLabel.SetLines(2)
		e.Button.SetChild(errLabel)
	} else {
		size := e.curSize
		if size == [2]int{} {
			// No size; pick the max size.
			size = e.maxSize
		}
		iconMissing := imgutil.IconPaintable("image-missing", size[0], size[1])
		e.Image.SetPaintable(iconMissing)
	}

	var tooltip string
	if e.name != "" {
		tooltip += html.EscapeString(e.name) + "\n"
	}
	tooltip += "<b>Error:</b> " + html.EscapeString(err.Error())
	e.Button.SetTooltipMarkup(tooltip)
}

// SetOpenURL sets the callback to be called when the user clicks the image.
func (e *Embed) SetOpenURL(f func()) {
	e.openURL = f
	e.Button.SetCanTarget(f != nil)
}

// SetSize sets the size of the image embed.
func (e *Embed) SetSize(w, h int) {
	w, h = imgutil.MaxSize(w, h, e.maxSize[0], e.maxSize[1])
	e.curSize = [2]int{w, h}
	e.SetSizeRequest(w, h)
}

// Size returns the embed size, or 0 if no images have been fetched yet or if
// SetSize has never been called before.
func (e *Embed) Size() (w, h int) {
	return e.curSize[0], e.curSize[1]
}
