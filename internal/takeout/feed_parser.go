package takeout

import (
	"encoding/xml"
	"time"
)

type Feed struct {
	XMLName xml.Name `xml:"feed"`
	Title   string   `xml:"title"`
	Entries []Entry  `xml:"entry"`
}

type Entry struct {
	AuthorName string    `xml:"author>name"`
	Published  time.Time `xml:"published"`
	Title      string    `xml:"title"`
	Content    string    `xml:"content"`
	Status     string    `xml:"status"`
	Categories []string  `xml:"category>term"`
}
