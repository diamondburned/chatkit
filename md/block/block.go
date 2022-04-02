package block

import (
	"container/list"
	"context"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// WidgetBlock describes the minimum interface of a child within the widget
// tree.
type WidgetBlock interface {
	gtk.Widgetter
}

// TextWidgetBlock is a WidgetBlock that also embeds a TextBlock. All children
// that has appendable texts should implement this.
type TextWidgetBlock interface {
	WidgetBlock
	TextBlock() *TextBlock
}

// ContainerWidgetBlock is a WidgetBlock that also embeds other WidgetBlocks. It
// should be implemented by embedding
type ContainerWidgetBlock interface {
	WidgetBlock
	State() *ContainerState
}

// ContainerState is the state of a single level of a Markdown node boxed inside
// a container of widgets.
type ContainerState struct {
	*gtk.Box
	// Viewer is the top-level Markdown viewer. It is the same for all new
	// ContainerStates created underneath the same Viewer.
	Viewer *Viewer

	// internal state
	list    *list.List
	current *list.Element
}

// newContainerState is used internally.
func newContainerState(viewer *Viewer, parent *gtk.Box) *ContainerState {
	s := ContainerState{Viewer: viewer}
	return s.WithParent(parent)
}

// WithParent creates a new ContainerState with the given parent widget and a
// new list. The Viewer is retained.
func (s *ContainerState) WithParent(parent *gtk.Box) *ContainerState {
	return &ContainerState{
		Box:    parent,
		Viewer: s.Viewer,
		list:   list.New(),
	}
}

// Context returns the Viewer's context.
func (s *ContainerState) Context() context.Context { return s.Viewer.ctx }

// TagTable returns the Viewer's TagTable.
func (s *ContainerState) TagTable() *gtk.TextTagTable {
	return s.Viewer.TagTable()
}

// Current returns the current WidgetBlock instance.
func (s *ContainerState) Current() WidgetBlock {
	if s.current != nil {
		return s.current.Value.(WidgetBlock)
	}
	return nil
}

// text returns the textBlock that is within any writable block.

// TextBlock returns either the current widget if it's a Text widget (*TextBlock
// or TextWidgetBlock), or it creates a new *TextBlock.
func (s *ContainerState) TextBlock() *TextBlock {
	switch text := s.Current().(type) {
	case *TextBlock:
		return text
	case TextWidgetBlock:
		return text.TextBlock()
	default:
		block := NewTextBlock(s)
		s.Append(block)
		return block
	}
}

// FinalizeBlock finalizes the current block. Any later use of the state must
// create a new block.
func (s *ContainerState) FinalizeBlock() {
	s.current = nil
}

// Append appends another block into the container state. The appended block is
// marked as the current block and will be returned by Current.
func (s *ContainerState) Append(block WidgetBlock) {
	s.current = s.list.PushBack(block)
	s.Box.Append(block)
}
