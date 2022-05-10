package mdrender

import (
	"strconv"

	"github.com/diamondburned/chatkit/md/block"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// RendererFunc is a map of callbacks for handling each ast.Node.
type RendererFunc func(r *Renderer, n ast.Node) ast.WalkStatus

// OptionFunc is a function type for any options that modify Renderer's
// internals.
type OptionFunc func(r *Renderer)

// WithRenderer adds a new renderer.
func WithRenderer(kind ast.NodeKind, renderer RendererFunc) OptionFunc {
	return func(r *Renderer) {
		if r.renderers == nil {
			r.renderers = make(map[ast.NodeKind]RendererFunc)
		}
		r.renderers[kind] = renderer
	}
}

// WithFallbackRenderer adds a new renderer that is called on any unhandled or
// unknown node.
func WithFallbackRenderer(renderer RendererFunc) OptionFunc {
	return func(r *Renderer) {
		r.fallbackR = renderer
	}
}

// Renderer is a rendering instance.
type Renderer struct {
	State *block.ContainerState

	renderers map[ast.NodeKind]RendererFunc
	fallbackR RendererFunc
	src       []byte
}

// NewRenderer creates a new renderer.
func NewRenderer(src []byte, state *block.ContainerState, opts ...OptionFunc) *Renderer {
	r := Renderer{
		src:   src,
		State: state,
	}

	if len(opts) > 0 {
		// Preallocate just in case.
		r.renderers = make(map[ast.NodeKind]RendererFunc, len(opts))
	}

	for _, opt := range opts {
		opt(&r)
	}

	return &r
}

// Source returns the source bytes slice.
func (r *Renderer) Source() []byte {
	return r.src
}

// Render renders n recursively.
func (r *Renderer) Render(n ast.Node) ast.WalkStatus {
	return r.RenderSiblings(n)
}

// RenderSiblings renders all siblings in n and returns SkipChildren if
// everything is successfully rendered.
func (r *Renderer) RenderSiblings(first ast.Node) ast.WalkStatus {
	for n := first; n != nil; n = n.NextSibling() {
		switch r.RenderOnce(n) {
		case ast.WalkContinue:
			if r.RenderChildren(n) == ast.WalkStop {
				return ast.WalkStop
			}
		case ast.WalkSkipChildren:
			continue
		case ast.WalkStop:
			return ast.WalkStop
		}
	}

	return ast.WalkSkipChildren
}

// RenderChildren renders all of n's children.
func (r *Renderer) RenderChildren(n ast.Node) ast.WalkStatus {
	return r.RenderSiblings(n.FirstChild())
}

// RenderChildrenWithTag calls RenderChildren wrapped within the given tag.
func (r *Renderer) RenderChildrenWithTag(n ast.Node, tagName string) ast.WalkStatus {
	status := ast.WalkContinue

	text := r.State.TextBlock()
	text.TagNameBounded(tagName, func() {
		status = r.RenderChildren(n)
	})

	return status
}

// WithState creates a copy of the current renderer with the given container
// state.
func (r *Renderer) WithState(state *block.ContainerState) *Renderer {
	cpy := *r
	cpy.State = state
	return &cpy
}

// RenderOnce renders a single node.
func (r *Renderer) RenderOnce(n ast.Node) ast.WalkStatus {
	f, ok := r.renderers[n.Kind()]
	if ok {
		return f(r, n)
	}

	switch n := n.(type) {
	case *ast.String:
		text := r.State.TextBlock()
		text.Insert(string(n.Value))

	case *ast.Text:
		text := r.State.TextBlock()
		text.Insert(string(n.Segment.Value(r.src)))

		switch {
		case n.HardLineBreak():
			text.EndLine(2)
		case n.SoftLineBreak():
			text.EndLine(1)
		}

	case *ast.Emphasis:
		var tagName string
		switch n.Level {
		case 1:
			tagName = "i"
		case 2:
			tagName = "b"
		default:
			return ast.WalkContinue
		}

		return r.RenderChildrenWithTag(n, tagName)

	case *ast.Heading:
		// h1 ~ h6
		if n.Level >= 1 && n.Level <= 6 {
			return r.RenderChildrenWithTag(n, "h"+strconv.Itoa(n.Level))
		}

	case *ast.CodeSpan:
		return r.RenderChildrenWithTag(n, "code")

	case *ast.Link:
		text := r.State.TextBlock()
		text.ConnectLinkHandler()

		if string(n.Title) != "" {
			text.Insert(string(n.Title))
		}

		startIx := text.Iter.Offset()
		status := r.RenderChildren(n)

		start := text.Iter.Copy()
		start.SetOffset(startIx)
		end := text.Iter

		text.ApplyLink(string(n.Destination), start, end)
		return status

	case *ast.AutoLink:
		text := r.State.TextBlock()
		text.ConnectLinkHandler()

		startIx := text.Iter.Offset()
		text.Insert(string(n.URL(r.src)))

		start := text.Iter.Copy()
		start.SetOffset(startIx)
		end := text.Iter

		text.ApplyLink(string(n.URL(r.src)), start, end)
		return ast.WalkContinue

	case *ast.Paragraph:
		text := r.State.TextBlock()
		text.EndLine(2)

	case *ast.FencedCodeBlock:
		lines := n.Lines()
		len := lines.Len()
		if len == 0 {
			return ast.WalkContinue
		}

		code := block.NewCodeBlock(r.State)
		code.TextBlock().TagNameBounded("code", func() {
			r.InsertSegments(code.TextBlock(), lines)
		})
		code.Highlight(string(n.Language(r.src)))

		r.State.Append(code)
		r.State.FinalizeBlock() // no more code from here on
		return ast.WalkSkipChildren

	case *ast.Blockquote:
		quote := block.NewBlockquote(r.State)
		r.State.Append(quote)
		return r.WithState(quote.State).RenderChildren(n)

	default:
		if r.fallbackR != nil {
			return r.fallbackR(r, n)
		}
	}

	return ast.WalkContinue
}

// InsertSegments inserts the given text segments into the buffer.
func (r *Renderer) InsertSegments(text *block.TextBlock, segs *text.Segments) {
	// Nothing about this "segments" API makes sense. It's literally useless
	// abstraction over just slicing the god-damn byte slice (or string, for
	// that matter).
	for i := 0; i < segs.Len(); i++ {
		// Also, At() returns a value but Value() has a pointer receiver. That's
		// just really dumb.
		seg := segs.At(i)
		text.Insert(string(seg.Value(r.src)))
	}
}
