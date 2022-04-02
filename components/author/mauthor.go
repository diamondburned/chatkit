// Package author handles rendering usernames.
package author

import (
	"fmt"
	"html"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
)

type markupOpts struct {
	textTag textutil.TextTag
	color   string
	suffix  string
	at      bool
	shade   bool
	minimal bool
}

// MarkupMod is a function type that Markup can take multiples of. It
// changes subtle behaviors of the Markup function, such as the color hasher
// used.
type MarkupMod func(opts *markupOpts)

// WithMinimal renders the markup without additional information, such as
// pronouns.
func WithMinimal() MarkupMod {
	return func(opts *markupOpts) {
		opts.minimal = true
	}
}

// WithShade renders the markup with a background shade.
func WithShade() MarkupMod {
	return func(opts *markupOpts) {
		opts.shade = true
	}
}

// WithMention makes the renderer prefix an at ("@") symbol.
func WithMention() MarkupMod {
	return func(opts *markupOpts) {
		opts.at = true
	}
}

// WithTextTagAttr sets the given attribute into the text tag used for the
// author. It is only useful for Text.
func WithTextTagAttr(attr textutil.TextTag) MarkupMod {
	return func(opts *markupOpts) {
		opts.textTag = attr
	}
}

// WithSuffix adds a small grey suffix string into the output string if the
// Minimal flag is not present.
func WithSuffix(suffix string) MarkupMod {
	return WithSuffixMarkup(html.EscapeString(suffix))
}

// WithSuffixMarkup is like WithSuffix, except the input is taken as valid
// markup.
func WithSuffixMarkup(suffix string) MarkupMod {
	return func(opts *markupOpts) {
		opts.suffix = suffix
	}
}

// WithColor sets the color of the rendered output.
func WithColor(color string) MarkupMod {
	return func(opts *markupOpts) {
		opts.color = color
	}
}

func mkopts(mods []MarkupMod) markupOpts {
	opts := markupOpts{}
	for _, mod := range mods {
		mod(&opts)
	}
	return opts
}

// Markup renders the markup string for the given user inside the given room.
// The markup format follows the Pango markup format.
//
// If the given room ID string is empty, then certain information are skipped.
// If it's non-empty, then the state will be used to fetch additional
// information.
func Markup(name string, mods ...MarkupMod) string {
	opts := mkopts(mods)

	if opts.at && !strings.HasPrefix(name, "@") {
		name = "@" + name
	}

	b := strings.Builder{}
	b.Grow(512)
	if opts.color != "" {
		if opts.shade {
			b.WriteString(fmt.Sprintf(
				`<span color="%s" bgcolor="%[1]s33">%s</span>`,
				opts.color, html.EscapeString(name),
			))
		} else {
			b.WriteString(fmt.Sprintf(
				`<span color="%s">%s</span>`,
				opts.color, html.EscapeString(name),
			))
		}
	} else {
		b.WriteString(name)
	}

	if opts.minimal {
		return b.String()
	}

	if opts.suffix != "" {
		b.WriteByte(' ')
		b.WriteString(fmt.Sprintf(
			`<span fgalpha="75%%" size="small">%s</span>`,
			string(opts.suffix),
		))
	}

	return b.String()
}

// Text renders the author's name into a rich text buffer. The written string is
// always minimal. The inserted tags have the "_mauthor" prefix.
func Text(iter *gtk.TextIter, name string, mods ...MarkupMod) {
	opts := mkopts(mods)

	if opts.at && !strings.HasPrefix(name, "@") {
		name = "@" + name
	} else if name == "" {
		return
	}

	start := iter.Offset()

	buf := iter.Buffer()
	buf.Insert(iter, name)

	if opts.color != "" {
		startIter := buf.IterAtOffset(start)

		tags := buf.TagTable()

		tag := tags.Lookup("_mauthor_" + opts.color)
		if tag == nil {
			attrs := textutil.TextTag{
				"foreground": opts.color,
			}
			if opts.shade {
				attrs["background"] = opts.color + "33" // alpha
			}
			if opts.textTag != nil {
				for k, v := range opts.textTag {
					attrs[k] = v
				}
			}
			tag = attrs.Tag("_mauthor_" + opts.color)
			tags.Add(tag)
		}

		buf.ApplyTag(tag, startIter, iter)
	}

	if opts.suffix != "" {
		buf.InsertMarkup(iter, " "+opts.suffix)
	}
}
