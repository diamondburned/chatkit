package embed

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/diamondburned/chatkit/components/progress"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/httputil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gotkit/utils/cachegc"
	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
)

// EmbedType indicates the type of the Embed being constructed. The type
// determines how it's displayed visually to the user.
type EmbedType uint8

const (
	EmbedTypeImage EmbedType = iota
	EmbedTypeVideo
	EmbedTypeGIF
	EmbedTypeGIFV // video GIF
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

func (t EmbedType) IsLooped() bool {
	return t == EmbedTypeGIF || t == EmbedTypeGIFV
}

func (t EmbedType) IsMuted() bool {
	return t != EmbedTypeVideo
}

// Opts contains options for Embed.
type Opts struct {
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
	// IgnoreWidth, if true, will cause Embed to be initialized without ever
	// setting a width request. This has the benefit of allowing the Embed to be
	// shrunken to any width, but it will introduce letterboxing.
	IgnoreWidth bool
}

// Embed is a user-clickable image with an open callback.
//
// Widget hierarchy:
//
//   - Widgetter (?)
//   - Button
//   - Thumbnail
type Embed struct {
	*gtk.Frame
	Button    *gtk.Button
	Thumbnail *onlineimage.Picture

	ctx   context.Context
	extra interface{ extra() }

	click func()
	name  string
	url   string

	curSize [2]int
	maxSize [2]int
	opts    Opts
}

type extraVideoEmbed struct {
	box      *gtk.Box
	play     *gtk.Image
	progress *progress.Bar

	video *gtk.Video
	media *gtk.MediaFile

	ctx gtkutil.Cancellable
}

type extraImageEmbed struct{}

func (*extraVideoEmbed) extra() {}
func (*extraImageEmbed) extra() {}

var embedCSS = cssutil.Applier("thumbnail-embed", `
	.thumbnail-embed > button {
		padding: 0;
		margin:  0;
		/* margin-left: -2px; */
		/* border:  2px solid transparent; */
		transition-duration: 150ms;
		transition-property: all;
	}
	.thumbnail-embed > button,
	.thumbnail-embed > button:hover {
		background: none;
	}
	.thumbnail-embed:not(.thumbnail-embed-interactive) > button:hover .thumbnail-embed-image {
		filter: contrast(80%) brightness(80%);
	}
	.thumbnail-embed .thumbnail-embed-image {
		background-color: black;
		border-radius: inherit;
		transition: linear 50ms filter;
	}
	.thumbnail-embed-errorlabel {
		color: @error_color;
		padding: 4px;
	}
	.thumbnail-embed-play {
		color: white;
		background-color: alpha(black, 0.75);
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
	.thumbnail-embed .progress-bar {
		margin-top: 8px;
		border-radius: 4px 4px 0 0;
		color: alpha(white, 0.75);
		background: alpha(black, 0.5);
	}
	.thumbnail-embed .progress-label {
		margin: 4px;
	}
`)

// New creates a thumbnail Embed.
func New(ctx context.Context, maxW, maxH int, opts Opts) *Embed {
	if opts.Provider == nil {
		opts.Provider = imgutil.HTTPProvider
	}

	e := &Embed{
		maxSize: [2]int{maxW, maxH},
		opts:    opts,
		ctx:     ctx,
	}

	ctx = imgutil.WithOpts(ctx,
		imgutil.WithErrorFn(e.onError),
		// imgutil.WithRescale(maxW, maxH),
	)

	e.Thumbnail = onlineimage.NewPicture(ctx, opts.Provider)
	e.Thumbnail.AddCSSClass("thumbnail-embed-image")
	e.Thumbnail.SetCanShrink(true)
	e.Thumbnail.SetKeepAspectRatio(true)

	e.Button = gtk.NewButton()
	e.Button.SetHasFrame(false)
	e.Button.ConnectClicked(func() { e.click() })

	e.Frame = gtk.NewFrame("")
	e.Frame.SetOverflow(gtk.OverflowHidden)
	e.Frame.SetChild(e.Button)
	embedCSS(e)

	if opts.Type == EmbedTypeImage {
		e.AddCSSClass("thumbnail-embed-typeimage")
		e.Button.SetChild(e.Thumbnail)
	} else {
		overlay := gtk.NewOverlay()
		overlay.SetChild(e.Thumbnail)
		overlay.AddCSSClass("thumbnail-embed-overlay")
		e.Button.SetChild(overlay)

		switch opts.Type {
		case EmbedTypeVideo, EmbedTypeGIFV:
			e.AddCSSClass("thumbnail-embed-interactive")
			switch opts.Type {
			case EmbedTypeVideo:
				e.AddCSSClass("thumbnail-embed-typevideo")
			case EmbedTypeGIFV:
				e.AddCSSClass("thumbnail-embed-typegifv")
			}

			play := gtk.NewImageFromIconName("media-playback-start-symbolic")
			play.AddCSSClass("thumbnail-embed-play")
			play.SetIconSize(gtk.IconSizeNormal)
			play.SetHAlign(gtk.AlignCenter)

			progress := progress.NewBar()
			progress.SetLabelFunc(func(n, max int64) string {
				if max == 0 {
					return "Downloading..."
				}
				return fmt.Sprintf(
					"Downloading... (%s/%s)",
					humanize.Bytes(uint64(n)),
					humanize.Bytes(uint64(max)),
				)
			})
			progress.Hide()

			box := gtk.NewBox(gtk.OrientationVertical, 0)
			box.SetVAlign(gtk.AlignCenter)
			box.SetHAlign(gtk.AlignCenter)
			box.Append(play)
			box.Append(progress)

			overlay.AddOverlay(box)

			e.extra = &extraVideoEmbed{
				box:      box,
				play:     play,
				progress: progress,
			}

		case EmbedTypeGIF:
			e.AddCSSClass("thumbnail-embed-typegif")

			gif := gtk.NewLabel("GIF")
			gif.AddCSSClass("thumbnail-embed-gifmark")
			gif.SetCanTarget(false)
			gif.SetVAlign(gtk.AlignStart) // top
			gif.SetHAlign(gtk.AlignEnd)   // right

			overlay.AddOverlay(gif)

			if onlineimage.CanAnimate {
				e.AddCSSClass("thumbnail-embed-interactive")

				motion := gtk.NewEventControllerMotion()
				e.Button.AddController(motion)

				anim := e.Thumbnail.EnableAnimation()

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

	e.NotifyImage(func() {
		if p := e.Thumbnail.Paintable(); p != nil {
			e.setSize(p.IntrinsicWidth(), p.IntrinsicHeight())
			e.finishSetting()
		}
	})

	e.SetOpenURL(e.ActivateDefault)
	return e
}

// SetHAlign sets the horizontal alignment of the embed relative to its parent.
func (e *Embed) SetHAlign(align gtk.Align) {
	e.Frame.SetHAlign(align)
	e.Button.SetHAlign(align)
}

// SetName sets the given embed name into everything that's displaying the embed
// name.
func (e *Embed) SetName(name string) {
	e.name = name
	if !e.Button.HasCSSClass("thumbnail-embed-interactive") {
		e.Button.SetTooltipText(name)
	}
}

// URL returns the Embed's current URL.
func (e *Embed) URL() string {
	return e.url
}

// SetFromURL sets the URL of the thumbnail embed.
func (e *Embed) SetFromURL(url string) {
	e.url = url
	e.Thumbnail.SetURL(url)
}

// NotifyImage calls f everytime the Embed thumbnail changes.
func (e *Embed) NotifyImage(f func()) glib.SignalHandle {
	return e.Thumbnail.NotifyProperty("paintable", f)
}

// undo effects
func (e *Embed) finishSetting() {
	if e.opts.CanHide {
		e.Show()
	}

	if e.opts.Whole {
		e.Button.SetChild(e.Thumbnail)
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
		e.Thumbnail.SetPaintable(iconMissing)
	}

	var tooltip string
	if e.name != "" {
		tooltip += html.EscapeString(e.name) + "\n"
	}
	tooltip += "<b>Error:</b> " + html.EscapeString(err.Error())
	e.Button.SetTooltipMarkup(tooltip)
}

func (e *Embed) isBusy() bool {
	base := gtk.BaseWidget(e)
	return !base.IsSensitive()
}

func (e *Embed) setBusy(busy bool) {
	gtk.BaseWidget(e).SetSensitive(!busy)
}

func (e *Embed) downloadVideo(vi *extraVideoEmbed) {
	if vi.ctx == nil {
		vi.ctx = gtkutil.WithVisibility(e.ctx, e)
		vi.ctx.OnRenew(func(context.Context) func() {
			// If we don't have a MediaFile yet and the video is reshown, then
			// allow clicking the video so the user can redownload.
			e.Frame.SetChild(e.Button)
			if vi.media == nil {
				base := gtk.BaseWidget(e)
				base.SetSensitive(true)
			}

			return func() {}
		})
	}

	if vi.video == nil {
		vi.video = gtk.NewVideo()
		vi.video.AddCSSClass("thumbnail-embed-video")
		vi.video.SetLoop(e.opts.Type.IsLooped())

		var handle glib.SignalHandle
		handle = e.ConnectUnmap(func() {
			e.HandlerDisconnect(handle)

			if vi.media != nil {
				vi.media.Ended()
				vi.video.Unparent()
			}

			vi.video = nil
		})
	}

	if !e.isBusy() && vi.media == nil {
		vi.progress.Show()
		if e.url == "" {
			vi.progress.Error(errors.New("video has no URL"))
			return
		}

		e.setBusy(true)
		cleanup := func() { e.setBusy(false) }

		ctx := vi.ctx.Take()

		u, err := url.Parse(e.url)
		if err != nil {
			vi.progress.Error(errors.Wrap(err, "invalid URL"))
			return
		}

		gtkutil.Async(ctx, func() func() {
			var file string

			switch u.Scheme {
			case "http", "https":
				cacheDir := app.FromContext(ctx).CachePath("videos")
				cacheDst := urlPath(cacheDir, u.String())
				if !fetchURL(ctx, u.String(), cacheDst, vi.progress) {
					return cleanup
				}
				file = cacheDst
			case "file":
				file = u.Host + u.Path
			default:
				return func() {
					vi.progress.Error(fmt.Errorf("unknown scheme %q (go do the refactor!)", u.Scheme))
					cleanup()
				}
			}

			return func() {
				cleanup()

				// TODO: consider just using vi.media directly, since it's a
				// Paintable, so we can just directly use it. Integrating it
				// with onlineimage.Picture might be tricky, however.
				vi.media = gtk.NewMediaFileForFilename(file)
				vi.media.SetLoop(e.opts.Type.IsLooped())
				vi.media.SetMuted(e.opts.Type.IsMuted())
				vi.media.Play()

				vi.video.SetMediaStream(vi.media)

				// Override Frame's child with the actual Video. The user won't
				// be seeing the thumbnail anymore.
				e.Frame.SetChild(vi.video)
			}
		})
	}
}

// SetOpenURL sets the callback to be called when the user clicks the image.
func (e *Embed) SetOpenURL(f func()) {
	e.click = f
	e.Button.SetCanTarget(f != nil)
}

// ActivateDefault triggers the default function that's called by default by
// SetOpenURL. It opens the browser unless it's a video, then it plays.
func (e *Embed) ActivateDefault() {
	switch e.opts.Type {
	case EmbedTypeVideo, EmbedTypeGIFV:
		vi := e.extra.(*extraVideoEmbed)
		e.downloadVideo(vi)
	default:
		// TODO: large image popup
		app.OpenURI(e.ctx, e.url)
	}
}

// DownloadVideoOnClick triggers the DownloadVideo function when the Embed is
// clicked. If the Embed is not a video, then the function does nothing.
func (e *Embed) DownloadVideoOnClick() {
	vi, ok := e.extra.(*extraVideoEmbed)
	if !ok {
		return
	}

	e.SetOpenURL(func() { e.downloadVideo(vi) })
}

// SetMaxSize sets the maximum size of the image.
func (e *Embed) SetMaxSize(w, h int) {
	e.maxSize = [2]int{w, h}
}

// ShrinkMaxSize sets the maximum size of the image to be the smaller of the
// current maximum size and the given size.
func (e *Embed) ShrinkMaxSize(w, h int) {
	w, h = imgutil.MaxSize(w, h, e.maxSize[0], e.maxSize[1])
	e.SetMaxSize(w, h)
}

// SetSizeRequest sets the minimum size of a widget. The dimensions are clamped
// to the maximum size given during construction, if any.
func (e *Embed) SetSizeRequest(w, h int) {
	if e.maxSize != [2]int{} {
		w, h = imgutil.MaxSize(w, h, e.maxSize[0], e.maxSize[1])
	}
	if e.opts.IgnoreWidth {
		w = -1
	}
	e.Frame.SetSizeRequest(w, h)
}

// setSize sets the size of the image embed.
func (e *Embed) setSize(w, h int) {
	if e.maxSize != [2]int{} {
		w, h = imgutil.MaxSize(w, h, e.maxSize[0], e.maxSize[1])
	}

	e.curSize = [2]int{w, h}

	if e.opts.IgnoreWidth {
		w = -1
	}
	e.Frame.SetSizeRequest(w, h)
}

// Size returns the original embed size optionally scaled down, or 0 if no
// images have been fetched yet or if SetSize has never been called before.
func (e *Embed) Size() (w, h int) {
	return e.curSize[0], e.curSize[1]
}

func fetchURL(ctx context.Context, url, cacheDst string, bar *progress.Bar) bool {
	err := cachegc.WithTmpFile(cacheDst, "*", func(f *os.File) error {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}

		client := httputil.FromContext(ctx, http.DefaultClient)

		r, err := client.Do(req)
		if err != nil {
			return err
		}
		defer r.Body.Close()

		if r.StatusCode < 200 || r.StatusCode > 299 {
			return fmt.Errorf("unexpected status code %d getting %q", r.StatusCode, url)
		}

		if r.ContentLength != -1 {
			glib.IdleAdd(func() { bar.SetMax(r.ContentLength) })
		}

		progr := progress.WrapReader(r.Body, bar)

		if _, err := io.Copy(f, progr); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		glib.IdleAdd(func() { bar.Error(err) })
		return false
	}

	return true
}

func urlPath(baseDir, url string) string {
	b := sha1.Sum([]byte(url))
	f := base64.URLEncoding.EncodeToString(b[:])
	return filepath.Join(baseDir, f)
}
