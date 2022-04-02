package mdrender

import (
	"bytes"
	"context"
	"log"
	"strconv"
	"strings"

	"github.com/diamondburned/chatkit/md"
	"github.com/diamondburned/chatkit/md/hl"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/yuin/goldmark/ast"
)

const wysiwygPrefix = "_wysiwyg_"

func wysiwygTag(tagTable *gtk.TextTagTable, name string) *gtk.TextTag {
	tag := tagTable.Lookup(wysiwygPrefix + name)
	if tag != nil {
		return tag
	}

	tt, ok := md.Tags[name]
	if !ok {
		log.Panicln("unknown tag name", name)
		return nil
	}

	tag = tt.Tag(wysiwygPrefix + name)
	tagTable.Add(tag)

	return tag
}

type WYSIWYGOpts struct {
	// Walker is the callback that's called when the tree is walked if it isn't
	// nil. It's meant to be a simple way to extend the renderer.
	Walker func(*WYSIWYG, ast.Node) ast.WalkStatus
	// SkipHTML, if true, will not highlight HTML tags.
	SkipHTML bool
}

// wysiwyg is the What-You-See-Is-What-You-Get node walker/highlighter.
type WYSIWYG struct {
	Buffer *gtk.TextBuffer
	Tags   *gtk.TextTagTable
	Source []byte

	Head *gtk.TextIter
	Tail *gtk.TextIter

	ctx      context.Context
	opts     WYSIWYGOpts
	invisTag *gtk.TextTag
}

// NewWYSIWYG creates a new instance of WYSIWYG.
func NewWYSIWYG(ctx context.Context, buffer *gtk.TextBuffer, opts WYSIWYGOpts) *WYSIWYG {
	return &WYSIWYG{
		Buffer: buffer,
		Tags:   buffer.TagTable(),
		ctx:    ctx,
		opts:   opts,
	}
}

// RenderWYSIWYG is a convenient function.
func RenderWYSIWYG(ctx context.Context, buffer *gtk.TextBuffer) {
	w := NewWYSIWYG(ctx, buffer, WYSIWYGOpts{})
	w.Render()
}

// Render renders the WYSIWYG content using the current content inside the
// buffer. The Head and Tail iterators are revalidated.
func (w *WYSIWYG) Render() {
	w.Head, w.Tail = w.Buffer.Bounds()
	w.Source = []byte(w.Buffer.Slice(w.Head, w.Tail, true))

	removeTags := make([]*gtk.TextTag, 0, w.Tags.Size())

	w.Tags.ForEach(func(tag *gtk.TextTag) {
		if strings.HasPrefix(tag.ObjectProperty("name").(string), wysiwygPrefix) {
			removeTags = append(removeTags, tag)
		}
	})

	// Ensure that the WYSIWYG tags are all gone.
	for _, tag := range removeTags {
		w.Buffer.RemoveTag(tag, w.Head, w.Tail)
	}

	// Error is not important.
	md.ParseAndWalk(w.Source, w.walker)
}

func (w *WYSIWYG) walker(n ast.Node, enter bool) (ast.WalkStatus, error) {
	if !enter {
		return ast.WalkContinue, nil
	}

	return w.enter(n), nil
}

func (w *WYSIWYG) enter(n ast.Node) ast.WalkStatus {
	switch n := n.(type) {
	case *ast.Emphasis:
		var tag string
		switch n.Level {
		case 1:
			tag = "i"
		case 2:
			tag = "b"
		default:
			return ast.WalkContinue
		}

		w.MarkText(n, tag)
		return ast.WalkSkipChildren

	case *ast.Heading:
		// h1 ~ h6
		if n.Level >= 1 && n.Level <= 6 {
			w.MarkTextFunc(n, []string{"h" + strconv.Itoa(n.Level)},
				func(head, tail *gtk.TextIter) {
					// Seek head to the start of the line to account for the
					// hash ("#").
					head.BackwardFindChar(func(ch uint32) bool { return rune(ch) == '\n' }, nil)
				},
			)
			return ast.WalkSkipChildren
		}

	case *ast.Link:
		linkTags := textutil.LinkTags()
		w.MarkTextTags(n, linkTags.FromTable(w.Tags, "a"))
		return ast.WalkSkipChildren

	case *ast.CodeSpan:
		w.MarkText(n, "code")
		return ast.WalkSkipChildren

	case *ast.RawHTML:
		segments := n.Segments.Sliced(0, n.Segments.Len())
		for _, seg := range segments {
			w.MarkBounds(seg.Start, seg.Stop, "htmltag")
		}

	case *ast.FencedCodeBlock:
		lines := n.Lines()

		len := lines.Len()
		if len == 0 {
			return ast.WalkSkipChildren
		}

		w.MarkBounds(lines.At(0).Start, lines.At(len-1).Stop, "code")

		if lang := string(n.Language(w.Source)); lang != "" {
			// Use markBounds' head and tail iterators.
			hl.Highlight(w.ctx, w.Head, w.Tail, lang)
		}

		return ast.WalkSkipChildren

	default:
		if w.opts.Walker != nil {
			return w.opts.Walker(w, n)
		}
	}

	return ast.WalkContinue
}

func (w *WYSIWYG) tag(tagName string) *gtk.TextTag {
	return wysiwygTag(w.Tags, tagName)
}

func (w *WYSIWYG) tags(tagNames []string) []*gtk.TextTag {
	tags := make([]*gtk.TextTag, len(tagNames))
	for i, name := range tagNames {
		tags[i] = w.tag(name)
	}
	return tags
}

// BoundIsInvisible returns true if the iterators are surrounding regions of
// texts that are invisible.
func (w *WYSIWYG) BoundIsInvisible() bool {
	if w.invisTag == nil {
		w.invisTag = md.Tags.FromTable(w.Tags, "_invisible")
	}

	return w.Head.HasTag(w.invisTag) && w.Tail.HasTag(w.invisTag)
}

// MarkBounds is like MarkText, except it takes custom bounds instead of ones
// from an ast.Node.
func (w *WYSIWYG) MarkBounds(i, j int, names ...string) {
	w.SetIter(w.Head, i)
	w.SetIter(w.Tail, j)

	if w.BoundIsInvisible() {
		return
	}

	for _, name := range names {
		w.Buffer.ApplyTag(w.tag(name), w.Head, w.Tail)
	}
}

// MarkText walks n's children and marks all its ast.Texts with the given tag.
func (w *WYSIWYG) MarkText(n ast.Node, names ...string) {
	w.MarkTextFunc(n, names, nil)
}

// MarkTextTags is the tag variant of markText.
func (w *WYSIWYG) MarkTextTags(n ast.Node, tags ...*gtk.TextTag) {
	w.MarkTextTagsFunc(n, tags, nil)
}

// MarkTextFunc is similar to markText, except the caller has control over
// the head and tail iterators before the tags are applied. This is useful for
// block elements.
func (w *WYSIWYG) MarkTextFunc(n ast.Node, names []string, f func(h, t *gtk.TextIter)) {
	w.MarkTextTagsFunc(n, w.tags(names), f)
}

// MarkTextTagsFunc is the tag variant of markTextFunc.
func (w *WYSIWYG) MarkTextTagsFunc(n ast.Node, tags []*gtk.TextTag, f func(h, t *gtk.TextIter)) {
	md.WalkChildren(n, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		text, ok := n.(*ast.Text)
		if !ok {
			return ast.WalkContinue, nil
		}

		w.SetIter(w.Head, text.Segment.Start)
		w.SetIter(w.Tail, text.Segment.Stop)

		if !w.BoundIsInvisible() {
			if f != nil {
				f(w.Head, w.Tail)
			}

			for _, tag := range tags {
				w.Buffer.ApplyTag(tag, w.Head, w.Tail)
			}
		}

		return ast.WalkContinue, nil
	})
}

// SetIter sets the given iterator to the given byte offset. It is the most
// correct way to convert an ast.Node's position to the iterator's.
//
// SetIter reimplements text/url.go's autolink.
func (w *WYSIWYG) SetIter(iter *gtk.TextIter, byteOffset int) {
	SetIter(iter, w.Source, byteOffset)
}

// SetIter sets the given iterator to the given byte offset. It is the most
// correct way to convert an ast.Node's position to the iterator's.
func SetIter(iter *gtk.TextIter, src []byte, byteOffset int) {
	part := src[:byteOffset]
	lines := bytes.Count(part, []byte("\n"))

	lineAt := 0
	if lines > 0 {
		lineAt = bytes.LastIndexByte(part, '\n') + 1
	}

	lineAt = len(part) - lineAt

	iter.SetLine(lines)
	iter.SetLineIndex(lineAt)
}
