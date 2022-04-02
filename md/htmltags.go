package md

import (
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
)

// Tags contains the tag table mapping most Matrix HTML tags to GTK TextTags.
var Tags = textutil.TextTagsMap{
	// https://www.w3schools.com/cssref/css_default_values.asp
	"h1":     htag(1.75),
	"h2":     htag(1.50),
	"h3":     htag(1.17),
	"h4":     htag(1.00),
	"h5":     htag(0.83),
	"h6":     htag(0.67),
	"em":     {"style": pango.StyleItalic},
	"i":      {"style": pango.StyleItalic},
	"strong": {"weight": pango.WeightBold},
	"b":      {"weight": pango.WeightBold},
	"u":      {"underline": pango.UnderlineSingle},
	"strike": {"strikethrough": true},
	"del":    {"strikethrough": true},
	"sup":    {"rise": +6000, "scale": 0.7},
	"sub":    {"rise": -2000, "scale": 0.7},
	"code": {
		"family":         "Monospace",
		"insert-hyphens": false,
	},
	"caption": {
		"weight": pango.WeightLight,
		"style":  pango.StyleItalic,
		"scale":  0.8,
	},
	"li": {
		"left-margin": 24, // px
	},
	"blockquote": {
		"foreground":  "#789922",
		"left-margin": 12, // px
	},

	// Not an actual HTML tag.
	"htmltag": {
		"family":     "Monospace",
		"foreground": "#808080",
	},

	// Meta tags.
	"_invisible": {"editable": false, "invisible": true},
	"_immutable": {"editable": false},
	"_emoji":     {"scale": EmojiScale},
	"_image":     {"rise": -2 * pango.SCALE},
	"_nohyphens": {"insert-hyphens": false},
}

func htag(scale float64) textutil.TextTag {
	return textutil.TextTag{
		"scale":  scale,
		"weight": pango.WeightBold,
	}
}
