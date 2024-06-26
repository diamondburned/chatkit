// Package md provides Markdown helper functions as well as styling.
package md

import (
	"unicode"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	markutil "github.com/yuin/goldmark/util"
	"libdb.so/go-emoji"
)

// Parser is the default Markdown parser.
var Parser = parser.NewParser(
	parser.WithInlineParsers(
		markutil.Prioritized(parser.NewLinkParser(), 0),
		markutil.Prioritized(parser.NewAutoLinkParser(), 1),
		markutil.Prioritized(parser.NewEmphasisParser(), 2),
		markutil.Prioritized(parser.NewCodeSpanParser(), 3),
		markutil.Prioritized(parser.NewRawHTMLParser(), 4),
	),
	parser.WithBlockParsers(
		markutil.Prioritized(parser.NewParagraphParser(), 0),
		markutil.Prioritized(parser.NewBlockquoteParser(), 1),
		markutil.Prioritized(parser.NewATXHeadingParser(), 2),
		markutil.Prioritized(parser.NewFencedCodeBlockParser(), 3),
		markutil.Prioritized(parser.NewThematicBreakParser(), 4), // <hr>
	),
)

// Renderer is the default Markdown renderer.
var Renderer = html.NewRenderer(
	html.WithHardWraps(),
	html.WithUnsafe(),
)

// Converter is the default converter that outputs HTML.
var Converter = goldmark.New(
	goldmark.WithParser(Parser),
	goldmark.WithRenderer(
		renderer.NewRenderer(
			renderer.WithNodeRenderers(
				markutil.Prioritized(Renderer, 1000),
			),
		),
	),
)

// EmojiScale is the scale of Unicode emojis.
const EmojiScale = 2.5

// EmojiAttrs is the Pango attributes set for a label showing an emoji. It is
// kept the same as the _emoji tag in TextTags.
var EmojiAttrs = textutil.Attrs(
	pango.NewAttrScale(EmojiScale),
)

// AddWidgetAt adds a widget into the text view at the current iterator
// position.
func AddWidgetAt(text *gtk.TextView, iter *gtk.TextIter, w gtk.Widgetter) {
	anchor := text.Buffer().CreateChildAnchor(iter)
	text.AddChildAtAnchor(w, anchor)
}

// WalkChildren walks n's children nodes using the given walker.
// WalkSkipChildren is returned unless the walker fails.
func WalkChildren(n ast.Node, walker ast.Walker) ast.WalkStatus {
	for n := n.FirstChild(); n != nil; n = n.NextSibling() {
		ast.Walk(n, walker)
	}
	return ast.WalkSkipChildren
}

// ParseAndWalk parses src and walks its Markdown AST tree.
func ParseAndWalk(src []byte, w ast.Walker) error {
	n := Parser.Parse(text.NewReader(src))
	return ast.Walk(n, w)
}

// BeginImmutable begins the immutability region in the text buffer that the
// text iterator belongs to. Calling the returned callback will end the
// immutable region. Calling it is not required, but the given iterator must
// still be valid when it's called.
func BeginImmutable(pos *gtk.TextIter) (end func()) {
	ix := pos.Offset()

	return func() {
		buf := pos.Buffer()
		tbl := buf.TagTable()
		tag := Tags.FromTable(tbl, "_immutable")
		buf.ApplyTag(tag, buf.IterAtOffset(ix), pos)
	}
}

// InsertInvisible inserts an invisible string of text into the buffer. This is
// useful for inserting invisible textual data during editing.
func InsertInvisible(pos *gtk.TextIter, txt string) {
	buf := pos.Buffer()
	insertInvisible(buf, pos, txt)
}

func insertInvisible(buf *gtk.TextBuffer, pos *gtk.TextIter, txt string) {
	tbl := buf.TagTable()
	tag := Tags.FromTable(tbl, "_invisible")

	start := pos.Offset()
	buf.Insert(pos, txt)

	startIter := buf.IterAtOffset(start)
	buf.ApplyTag(tag, startIter, pos)
}

var whitespaces = [255]bool{
	' ':  true,
	'\t': true,
	'\n': true,
	'\r': true,
}

// IsUnicodeEmoji returns true if the given string only contains a Unicode
// emoji.
func IsUnicodeEmoji(v string) bool {
	for _, r := range v {
		if !unicode.IsSpace(r) && !emoji.IsEmoji(r) {
			return false
		}
	}
	return true
}
