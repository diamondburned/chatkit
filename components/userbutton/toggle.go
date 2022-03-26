package userbutton

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
)

// Toggle is a toggle button showing the user avatar. It shows a PopoverMenu
// when clicked.
type Toggle struct {
	*gtk.ToggleButton
	MenuItems []gtkutil.PopoverMenuItem

	avatar *onlineimage.Avatar
	ctx    context.Context

	menuFn    func() []gtkutil.PopoverMenuItem
	popoverFn func(*gtk.PopoverMenu)
}

var toggleCSS = cssutil.Applier("userbutton-toggle", `
	.userbutton-toggle {
		border-radius: 999px;
		border:  none;
		margin:  0;
		padding: 2px;
	}
	.userbutton-toggle:checked {
		background-color: @theme_selected_bg_color;
	}
`)

const AvatarSize = 32

// NewToggle creates a new Toggle instance. It takes parameters similar to
// NewPopover.
func NewToggle(ctx context.Context, provider imgutil.Provider) *Toggle {
	t := Toggle{ctx: ctx}

	t.avatar = onlineimage.NewAvatar(ctx, provider, AvatarSize)

	t.ToggleButton = gtk.NewToggleButton()
	t.SetChild(t.avatar)
	t.ConnectClicked(func() {
		if t.menuFn == nil {
			t.SetActive(false)
			return
		}

		popover := gtkutil.NewPopoverMenuCustom(nil, gtk.PosBottom, t.menuFn())
		popover.ConnectHide(func() { t.SetActive(false) })

		if t.popoverFn != nil {
			t.popoverFn(popover)
		}

		gtkutil.PopupFinally(popover)
	})
	toggleCSS(t)

	return &t
}

// SetName sets the tooltip hover and the avatar initials.
func (t *Toggle) SetName(name string) {
	t.avatar.SetInitials(name)
	t.SetTooltipText(name)
}

// SetAvatar sets the avatar URL.
func (t *Toggle) SetAvatar(url string) {
	t.avatar.SetFromURL(url)
}

// SetMenuFunc sets the menu function. The function is invoked everytime the
// PopoverMenu is created.
func (t *Toggle) SetMenuFunc(f func() []gtkutil.PopoverMenuItem) {
	t.menuFn = f
}

// SetPopoverFunc sets the function to be called when a Popover is spawned.
func (t *Toggle) SetPopoverFunc(f func(*gtk.PopoverMenu)) {
	t.popoverFn = f
}
