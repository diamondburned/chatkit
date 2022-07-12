package md

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
)

// BindLinkHandler binds input handlers for triggering hyperlinks within the
// TextView. If BindLinkHandler is called on the same TextView again, then it
// does nothing. The function checks this by checking for the .gmd-hyperlinked
// class.
func BindLinkHandler(tview *gtk.TextView, onURL func(string)) {
	if tview.HasCSSClass("md-hyperlinked") {
		return
	}
	tview.AddCSSClass("md-hyperlinked")

	linkTags := textutil.LinkTags()

	checkURL := func(x, y float64) *EmbeddedURL {
		bx, by := tview.WindowToBufferCoords(gtk.TextWindowWidget, int(x), int(y))
		it, ok := tview.IterAtLocation(bx, by)
		if !ok {
			return nil
		}

		for _, tags := range it.Tags() {
			tagName := tags.ObjectProperty("name").(string)

			if !strings.HasPrefix(tagName, urlTagPrefix) {
				continue
			}

			u, ok := ParseEmbeddedURL(strings.TrimPrefix(tagName, urlTagPrefix))
			if ok {
				return &u
			}
		}

		return nil
	}

	buf := tview.Buffer()
	table := buf.TagTable()

	click := gtk.NewGestureClick()
	click.SetButton(1)
	click.SetExclusive(true)
	click.ConnectAfter("pressed", func(nPress int, x, y float64) {
		if nPress != 1 {
			return
		}

		if u := checkURL(x, y); u != nil {
			onURL(u.URL)

			tag := linkTags.FromBuffer(buf, "a:visited")
			buf.ApplyTag(tag, buf.IterAtOffset(u.From), buf.IterAtOffset(u.To))
		}
	})

	var (
		lastURL *EmbeddedURL
		lastTag *gtk.TextTag
	)

	unhover := func() {
		if lastURL != nil {
			buf.RemoveTag(lastTag, buf.IterAtOffset(lastURL.From), buf.IterAtOffset(lastURL.To))
			lastURL = nil
			lastTag = nil
		}
	}

	motion := gtk.NewEventControllerMotion()
	motion.ConnectLeave(func() {
		unhover()
	})
	motion.ConnectMotion(func(x, y float64) {
		u := checkURL(x, y)
		if u == lastURL {
			return
		}

		unhover()

		if u != nil {
			hover := linkTags.FromTable(table, "a:hover")
			buf.ApplyTag(hover, buf.IterAtOffset(u.From), buf.IterAtOffset(u.To))

			lastURL = u
			lastTag = hover
		}
	})

	tview.AddController(click)
	tview.AddController(motion)
}

// urlTagPrefix is the prefix for tag names that identify a hyperlinked URL.
const urlTagPrefix = "link:"

// EmbeddedURL is a type that describes a URL and its bounds within a text
// buffer.
type EmbeddedURL struct {
	From int    `json:"1"`
	To   int    `json:"2"`
	URL  string `json:"u"`
}

// URLTagName creates a new URL tag name from the given URL.
func URLTagName(start, end *gtk.TextIter, url string) string {
	return urlTagPrefix + embedURL(start.Offset(), end.Offset(), url)
}

func embedURL(x, y int, url string) string {
	b, err := json.Marshal(EmbeddedURL{x, y, url})
	if err != nil {
		log.Panicln("bug: error marshaling embeddedURL:", err)
	}

	return string(b)
}

// ParseEmbeddedURL parses the inlined data into an embedded URL structure.
func ParseEmbeddedURL(data string) (EmbeddedURL, bool) {
	var d EmbeddedURL

	if err := json.Unmarshal([]byte(data), &d); err != nil {
		log.Println("error parsing internal embedded URL:", err)
		return d, false
	}

	return d, true
}
