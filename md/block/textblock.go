package block

import (
	"log"
	"strings"

	"github.com/diamondburned/chatkit/md"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
)

var textBlockCSS = cssutil.Applier("md-textblock", `
	textview.md-textblock,
	textview.md-textblock text {
		background-color: transparent;
		color: @theme_fg_color;
	}
`)

// NewDefaultTextView creates a new TextView that TextBlock uses.
func NewDefaultTextView(Buffer *gtk.TextBuffer) *gtk.TextView {
	tview := gtk.NewTextViewWithBuffer(Buffer)
	tview.SetEditable(false)
	tview.SetCursorVisible(false)
	tview.SetVExpand(true)
	tview.SetHExpand(true)
	tview.SetWrapMode(gtk.WrapWordChar)

	textBlockCSS(tview)
	textutil.SetTabSize(tview)

	return tview
}

type TextBlock struct {
	*gtk.TextView
	// Iter is the text block's internal iterator. Use this while walking to
	// insert texts. All methods that act on the block will also use this
	// iterator.
	Iter *gtk.TextIter
	// Buffer is the TextView's TextBuffer.
	Buffer *gtk.TextBuffer

	state *ContainerState
}

var _ TextWidgetBlock = (*TextBlock)(nil)

// NewTextBlock creates a new TextBlock.
func NewTextBlock(state *ContainerState) *TextBlock {
	tbuf := gtk.NewTextBuffer(state.Viewer.TagTable())
	view := NewDefaultTextView(tbuf)
	return NewTextBlockFromView(view, state)
}

// NewTextBlockFromView creates a new TextBlock from the given TextView.
func NewTextBlockFromView(view *gtk.TextView, state *ContainerState) *TextBlock {
	text := TextBlock{
		Buffer: view.Buffer(),
		state:  state,
	}

	text.Iter = text.Buffer.StartIter()
	text.TextView = view

	text.Buffer.SetEnableUndo(false)
	text.AddCSSClass("md-textblock")
	return &text
}

// TextBlock returns itself. It impleemnts the TextWidgetChild interface.
func (b *TextBlock) TextBlock() *TextBlock { return b }

// ConnectLinkHandler connects the hyperlink handler into the TextBlock. Call
// this method if the TextBlock has a link. Only the first call will bind the
// handler.
func (b *TextBlock) ConnectLinkHandler() {
	md.BindLinkHandler(b.TextView, func(url string) { app.OpenURI(b.state.Context(), url) })
}

// TrailingNewLines counts the number of trailing new lines up to 2.
func (b *TextBlock) TrailingNewLines() int {
	if !b.IsNewLine() {
		return 0
	}

	seeker := b.Iter.Copy()

	for i := 0; i < 2; i++ {
		if !seeker.BackwardChar() || rune(seeker.Char()) != '\n' {
			return i
		}
	}

	return 2
}

// IsNewLine returns true if the iterator is currently on a new line.
func (b *TextBlock) IsNewLine() bool {
	if !b.Iter.BackwardChar() {
		// empty Bufferfer, so consider yes
		return true
	}

	// take the character, then undo the backward immediately
	char := rune(b.Iter.Char())
	b.Iter.ForwardChar()

	return char == '\n'
}

// EndLine ensures that the given amount of new lines will be put before the
// iterator. It accounts for existing new lines in the Bufferfer.
func (b *TextBlock) EndLine(amount int) {
	// Only add more lines if we're not at the start of the text block.
	if b.Iter.Offset() > 0 {
		b.InsertNewLines(amount - b.TrailingNewLines())
	}
}

// InsertNewLines inserts n new lines without checking for existing new lines.
// Most users should use EndLine instead. If n < 1, then no insertion is done.
func (b *TextBlock) InsertNewLines(n int) {
	if n < 1 {
		return
	}
	b.Buffer.Insert(b.Iter, strings.Repeat("\n", n))
}

// ApplyLink applies tags denoting a hyperlink.
func (b *TextBlock) ApplyLink(url string, start, end *gtk.TextIter) {
	b.Buffer.ApplyTag(b.EmptyTag(md.URLTagName(url)), start, end)
	b.Buffer.ApplyTag(textutil.LinkTags().FromTable(b.state.TagTable(), "a"), start, end)
	b.ConnectLinkHandler()
}

// EmptyTag gets an existing tag or creates a new empty one with the given name.
func (b *TextBlock) EmptyTag(tagName string) *gtk.TextTag {
	return emptyTag(b.state.Viewer.TagTable(), tagName)
}

func emptyTag(table *gtk.TextTagTable, tagName string) *gtk.TextTag {
	if tag := table.Lookup(tagName); tag != nil {
		return tag
	}

	tag := gtk.NewTextTag(tagName)
	if !table.Add(tag) {
		log.Panicf("failed to add new tag %q", tagName)
	}

	return tag
}

// Tag returns a tag from the md.Tags table. One is added if it's not already in
// the shared TagsTable.
func (b *TextBlock) Tag(tagName string) *gtk.TextTag {
	return md.Tags.FromTable(b.state.Viewer.TagTable(), tagName)
}

// TagNameBounded wraps around TagBounded and HTMLTag.
func (b *TextBlock) TagNameBounded(tagName string, f func()) {
	b.TagBounded(b.Tag(tagName), f)
}

// TagBounded saves the current offset and calls f, expecting the function to
// use s.iter. Then, the tag with the given name is applied on top.
func (b *TextBlock) TagBounded(tag *gtk.TextTag, f func()) {
	start := b.Iter.Offset()
	f()
	startIter := b.Buffer.IterAtOffset(start)
	b.Buffer.ApplyTag(tag, startIter, b.Iter)
}

// Insert inserts text into the buffer at the current iterator position.
func (b *TextBlock) Insert(text string) {
	b.Buffer.Insert(b.Iter, text)
}
