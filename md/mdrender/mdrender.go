package mdrender

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"github.com/diamondburned/chatkit/md/block"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"libdb.so/ctxt"
)

// RendererFunc is a map of callbacks for handling each ast.Node.
type RendererFunc func(ctx context.Context, r *Renderer, n ast.Node) ast.WalkStatus

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

// WithState adds a new container state to the given context.
func WithState(ctx context.Context, state *block.ContainerState) context.Context {
	return ctxt.With(ctx, state)
}

// Renderer is a rendering instance.
type Renderer struct {
	state     *block.ContainerState
	renderers map[ast.NodeKind]RendererFunc
	fallbackR RendererFunc
	src       []byte
}

// NewRenderer creates a new renderer.
func NewRenderer(src []byte, state *block.ContainerState, opts ...OptionFunc) *Renderer {
	r := Renderer{
		src:   src,
		state: state,
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

// State returns the container state associated with the given context, or it
// returns the default state.
func (r *Renderer) State(ctx context.Context) *block.ContainerState {
	state, ok := ctxt.From[*block.ContainerState](ctx)
	if ok {
		return state
	}
	return r.state
}

// Source returns the source bytes slice.
func (r *Renderer) Source() []byte {
	return r.src
}

// Render renders n recursively.
func (r *Renderer) Render(ctx context.Context, n ast.Node) ast.WalkStatus {
	return r.RenderSiblings(ctx, n)
}

// RenderSiblings renders all siblings in n and returns SkipChildren if
// everything is successfully rendered.
func (r *Renderer) RenderSiblings(ctx context.Context, first ast.Node) ast.WalkStatus {
	for n := first; n != nil; n = n.NextSibling() {
		switch r.RenderOnce(ctx, n) {
		case ast.WalkContinue:
			if r.RenderChildren(ctx, n) == ast.WalkStop {
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
func (r *Renderer) RenderChildren(ctx context.Context, n ast.Node) ast.WalkStatus {
	return r.RenderSiblings(ctx, n.FirstChild())
}

// RenderChildrenWithTag calls RenderChildren wrapped within the given tag.
func (r *Renderer) RenderChildrenWithTag(ctx context.Context, n ast.Node, tagName string) ast.WalkStatus {
	status := ast.WalkContinue

	text := r.State(ctx).TextBlock()
	text.TagNameBounded(tagName, func() {
		status = r.RenderChildren(ctx, n)
	})

	return status
}

// RenderOnce renders a single node.
func (r *Renderer) RenderOnce(ctx context.Context, n ast.Node) ast.WalkStatus {
	f, ok := r.renderers[n.Kind()]
	if ok {
		return f(ctx, r, n)
	}

	switch n := n.(type) {
	case *ast.String:
		text := r.State(ctx).TextBlock()
		text.Insert(string(n.Value))

	case *ast.Text:
		text := r.State(ctx).TextBlock()
		text.Insert(string(n.Segment.Value(r.src)))

		switch {
		case n.HardLineBreak():
			text.InsertNewLines(2)
		case n.SoftLineBreak():
			text.InsertNewLines(1)
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

		return r.RenderChildrenWithTag(ctx, n, tagName)

	case *ast.Heading:
		// h1 ~ h6
		if n.Level >= 1 && n.Level <= 6 {
			text := r.State(ctx).TextBlock()
			text.EndLine(2)
			return r.RenderChildrenWithTag(ctx, n, "h"+strconv.Itoa(n.Level))
		}

	case *ast.CodeSpan:
		return r.RenderChildrenWithTag(ctx, n, "code")

	case *ast.Link:
		text := r.State(ctx).TextBlock()
		text.ConnectLinkHandler()

		if string(n.Title) != "" {
			text.Insert(string(n.Title))
		}

		startIx := text.Iter.Offset()
		status := r.RenderChildren(ctx, n)

		start := text.Iter.Copy()
		start.SetOffset(startIx)
		end := text.Iter

		text.ApplyLink(string(n.Destination), start, end)
		return status

	case *ast.AutoLink:
		text := r.State(ctx).TextBlock()
		text.ConnectLinkHandler()

		startIx := text.Iter.Offset()
		text.Insert(string(n.URL(r.src)))

		start := text.Iter.Copy()
		start.SetOffset(startIx)
		end := text.Iter

		text.ApplyLink(string(n.URL(r.src)), start, end)
		return ast.WalkContinue

	case *ast.List:
		if n.IsOrdered() {
			ctx = ctxt.With(ctx, &block.ListIndex{
				Index:     n.Start,
				Unordered: false,
			})
		} else {
			ctx = ctxt.With(ctx, &block.ListIndex{
				Unordered: true,
			})
		}
		r.RenderChildren(ctx, n)
		return ast.WalkSkipChildren

	case *ast.ListItem:
		// Ensure the ListItemBlock is never reused for other nodes.
		defer r.State(ctx).FinalizeBlock()

		listIx, ok := ctxt.From[*block.ListIndex](ctx)
		if !ok {
			// ListItem outside of List? Don't handle.
			return ast.WalkContinue
		}

		listIx.Level = n.Offset
		slog.Debug(
			"rendering list item using mdrender",
			"index", listIx.Index,
			"level", listIx.Level,
			"unordered", listIx.Unordered)

		listItem := block.NewListItemBlock(r.State(ctx), *listIx)
		r.State(ctx).Append(listItem)

		// TODO: prevent children from having block-level elements.
		r.RenderChildren(ctx, n)

		listIx.Index++
		return ast.WalkSkipChildren

	case *ast.Paragraph:
		// Fix stupid assumptions about HTML.
		if n.ChildCount() == 1 {
			if _, ok := n.FirstChild().(*ast.FencedCodeBlock); ok {
				break
			}
		}

		text := r.State(ctx).TextBlock()
		text.EndLine(2)

	case *ast.FencedCodeBlock:
		lines := n.Lines()
		len := lines.Len()
		if len == 0 {
			return ast.WalkContinue
		}

		code := block.NewCodeBlock(r.State(ctx))
		code.TextBlock().TagNameBounded("code", func() {
			r.InsertSegments(code.TextBlock(), lines)
		})
		code.Highlight(string(n.Language(r.src)))

		r.State(ctx).Append(code)
		r.State(ctx).FinalizeBlock() // no more code from here on
		return ast.WalkSkipChildren

	case *ast.Blockquote:
		quote := block.NewBlockquote(r.State(ctx))
		r.State(ctx).Append(quote)
		return r.RenderChildren(WithState(ctx, quote.State), n)

	default:
		if r.fallbackR != nil {
			return r.fallbackR(ctx, r, n)
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

		value := string(seg.Value(r.src))
		// Replace CRLF to LF, just in case the messages contain this. This will
		// screw up syntax highlighting alignment.
		value = strings.ReplaceAll(value, "\r\n", "\n")

		text.Insert(value)
	}
}
