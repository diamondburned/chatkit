package avatarbadge

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

// AvatarBadge wraps a badge and an arbitrary widget that the badge is laid on
// top of.
type AvatarBadge struct {
	*gtk.Overlay
	Badge *Badge
	child gtk.Widgetter
}

// Badge is the image badge.
type Badge struct{}
