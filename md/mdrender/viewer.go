package mdrender

import (
	"context"

	"github.com/diamondburned/chatkit/md/block"
	"github.com/yuin/goldmark/ast"
)

// MarkdownViewer extends a block.Viewer to view Markdown. A Markdown viewer is
// immutable.
type MarkdownViewer struct {
	*block.Viewer
}

// NewMarkdownViewer creates a new MarkdownViewer.
func NewMarkdownViewer(ctx context.Context, src []byte, n ast.Node, opts ...OptionFunc) *MarkdownViewer {
	v := block.NewViewer(ctx)
	r := NewRenderer(src, v.State(), opts...)
	r.Render(ctx, n)

	return &MarkdownViewer{
		Viewer: v,
	}
}
