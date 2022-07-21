package embed

import (
	"context"
	"math"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
)

type Viewer struct {
	*gtk.Dialog
	Scroll *gtk.ScrolledWindow
	Embed  *Embed
	Button ViewerButtons

	extra viewerExtra
	vadj  *gtk.Adjustment
	hadj  *gtk.Adjustment

	ctx context.Context
}

type ViewerButtons struct {
	Back         *gtk.Button
	Download     *gtk.Button
	CopyURL      *gtk.Button
	OpenOriginal *gtk.Button
}

type viewerExtra interface {
	viewerExtra()
}

type viewerExtraImage struct {
	fixed *gtk.Fixed
	scale float64
}

func (viewerExtraImage) viewerExtra() {}

func NewViewer(ctx context.Context, opts Opts) *Viewer {
	v := Viewer{ctx: ctx}
	v.Embed = New(ctx, 0, 0, opts)

	v.Scroll = gtk.NewScrolledWindow()
	v.Scroll.SetVExpand(true)
	v.Scroll.SetHExpand(true)

	v.vadj = v.Scroll.VAdjustment()
	v.hadj = v.Scroll.HAdjustment()

	v.Dialog = gtk.NewDialogWithFlags(
		app.FromContext(ctx).SuffixedTitle("Preview"),
		app.GTKWindowFromContext(ctx),
		gtk.DialogModal|gtk.DialogUseHeaderBar|gtk.DialogDestroyWithParent)
	v.Dialog.SetDefaultSize(400, 400)
	v.Dialog.SetChild(v.Scroll)

	v.Button = ViewerButtons{
		Back:         newActionButton(v, "Back", "go-previous-symbolic", "embedviewer.close"),
		Download:     newActionButton(v, "Download", "folder-download-symbolic", "embedviewer.download"),
		CopyURL:      newActionButton(v, "Copy URL", "edit-copy-symbolic", "embedviewer.copy-url"),
		OpenOriginal: newActionButton(v, "Open Original", "text-html-symbolic", "embedviewer.open-original"),
	}

	header := v.Dialog.HeaderBar()
	header.SetShowTitleButtons(false)
	header.PackStart(v.Button.Back)
	header.PackEnd(v.Button.CopyURL)
	header.PackEnd(v.Button.Download)
	header.PackEnd(v.Button.OpenOriginal)

	gtkutil.BindActionMap(v, map[string]func(){
		"embedviewer.close":         v.close,
		"embedviewer.download":      v.download,
		"embedviewer.copy-url":      v.copyURL,
		"embedviewer.open-original": v.openOriginal,
	})

	switch opts.Type {
	case EmbedTypeImage, EmbedTypeGIF:
		// Allow pinch-to-zoom, since we have no video player UI.
		fixed := gtk.NewFixed()
		fixed.SetVAlign(gtk.AlignCenter)
		fixed.SetHAlign(gtk.AlignCenter)
		fixed.Put(v.Embed, 0, 0)

		v.Scroll.SetChild(fixed)
		v.Scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)

		v.extra = &viewerExtraImage{
			fixed: fixed,
			scale: 1.0,
		}

		// Disable click-to-open so we can handle panning.
		v.Embed.SetOpenURL(nil)
		v.Embed.NotifyImage(func() {
			v.scaleFit()
		})

		var mouseX, mouseY float64

		motionCtrl := gtk.NewEventControllerMotion()
		motionCtrl.ConnectMotion(func(x, y float64) {
			mouseX = x
			mouseY = y
		})

		scrollCtrl := gtk.NewEventControllerScroll(gtk.EventControllerScrollVertical)
		scrollCtrl.SetPropagationPhase(gtk.PhaseCapture)
		scrollCtrl.ConnectScroll(func(_, dy float64) bool {
			mod := scrollCtrl.CurrentEventState()
			if (mod & gdk.ControlMask) != 0 {
				// One discrete scroll up is -1.0, and we want to scale maybe
				// 1.1x in, so we scale the value.
				if dy > 0 {
					// scroll down
					v.scale(dy*+(1-0.1), mouseX, mouseY)
				} else {
					// scroll up
					v.scale(dy*-(1+0.1), mouseX, mouseY)
				}

				return true
			}
			return false
		})

		// Treat this specially, otherwise Scroll will eat up the events.
		v.AddController(scrollCtrl)

		// Keep track of the scroll begin coordinates so we can get the offset
		// properly.
		var originX, originY float64

		dragCtrl := gtk.NewGestureDrag()
		dragCtrl.ConnectDragBegin(func(x, y float64) {
			originX = v.hadj.Value()
			originY = v.vadj.Value()
		})
		dragCtrl.ConnectDragUpdate(func(offsetX, offsetY float64) {
			v.hadj.SetValue(originX - offsetX)
			v.vadj.SetValue(originY - offsetY)
		})

		v.Scroll.AddController(dragCtrl)
		v.Scroll.SetChild(fixed)

	case EmbedTypeGIFV, EmbedTypeVideo:
		// Don't allow pinch-to-zoom, but at least fill the embed.
		v.Embed.SetVExpand(true)
		v.Embed.SetHExpand(true)
		v.Embed.SetVAlign(gtk.AlignFill)
		v.Embed.SetHAlign(gtk.AlignFill)

		v.Scroll.SetChild(v.Embed)
		v.Scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyNever)
	default:
		panic("unsupported embed type")
	}

	return &v
}

func newActionButton(target gtk.Widgetter, text, icon, action string) *gtk.Button {
	button := gtk.NewButtonFromIconName(icon)
	button.AddCSSClass(icon)
	button.SetTooltipText(text)
	button.ConnectClicked(func() {
		base := gtk.BaseWidget(target)
		base.ActivateAction(action, nil)
	})
	return button
}

// AddButton adds a header button into the Viewer.
func (v *Viewer) AddButton(pack gtk.PositionType, button *gtk.Button) {
	header := v.Dialog.HeaderBar()
	switch pack {
	case gtk.PosTop, gtk.PosLeft:
		header.PackStart(button)
	case gtk.PosBottom, gtk.PosRight:
		header.PackEnd(button)
	}
}

func (v *Viewer) close() { v.Dialog.Close() }

func (v *Viewer) download() {

}

func (v *Viewer) copyURL() {
	url := v.Embed.URL()

	display := gdk.DisplayGetDefault()

	clipboard := display.Clipboard()
	clipboard.SetText(url)

	popover := gtk.NewPopover()
	popover.SetParent(v.Button.CopyURL)
	popover.SetPosition(gtk.PosBottom)
	popover.SetChild(gtk.NewLabel("Copied URL!"))
	popover.Popup()

	glib.TimeoutSecondsAdd(3, func() {
		popover.Popdown()
		glib.TimeoutSecondsAdd(3, popover.Unparent)
	})
}

func (v *Viewer) openOriginal() {
	app.OpenURI(v.ctx, v.Embed.URL())
}

func (v *Viewer) scaleFit() {
	extra, ok := v.extra.(*viewerExtraImage)
	if !ok {
		return
	}

	viewportAlloc := v.Scroll.Allocation()

	vpw := viewportAlloc.Width()
	vph := viewportAlloc.Height()

	w, h := v.Embed.Size()

	newW, newH := imgutil.MaxSize(w, h, vpw, vph)
	v.Embed.SetSizeRequest(newW, newH)

	// Calculate the current scale. The aspect ratio might be diffrent, so we
	// get the smallest one.
	wscale := float64(vpw) / float64(w)
	hscale := float64(vph) / float64(h)
	extra.scale = math.Min(wscale, hscale)

	// Recenter the image.
	posX, posY := imageOrigin(
		float64(newW),
		float64(newH),
		float64(vpw),
		float64(vph))
	extra.fixed.Move(v.Embed, posX, posY)
}

func (v *Viewer) scale(mult, originX, originY float64) {
	extra, ok := v.extra.(*viewerExtraImage)
	if !ok {
		return
	}

	wInt, hInt := v.Embed.Size()
	if wInt == 0 || hInt == 0 {
		return
	}

	w := float64(wInt)
	h := float64(hInt)

	extra.scale *= float64(mult)
	w *= extra.scale
	h *= extra.scale

	v.Embed.SetSizeRequest(round(w), round(h))

	// // Calculate the new scroll values. We do this by taking the origin and
	// // multiplying it by the same scaling offset.
	// scrollX := v.hadj.Value() * extra.scale
	// scrollY := v.vadj.Value() * extra.scale

	// v.hadj.SetValue(scrollX)
	// v.vadj.SetValue(scrollY)

	viewportAlloc := v.Scroll.Allocation()

	posX, posY := imageOrigin(
		w, h,
		float64(viewportAlloc.Width()),
		float64(viewportAlloc.Height()))
	extra.fixed.Move(v.Embed, posX, posY)
}

// posX/Y: Default to (0, 0) origin and let ScrolledWindow handle moving.
func imageOrigin(w, h, vpw, vph float64) (posX, posY float64) {
	// Center the dimensions if they're smaller than the parent viewport.
	if vpw > w {
		posX = (vpw - w) / 2
	}
	if vph > h {
		posY = (vph - h) / 2
	}
	return
}

func round(v float64) int {
	return int(math.Round(v))
}
