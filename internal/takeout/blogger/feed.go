package blogger

import (
	"encoding/xml"
	"html"
	"time"

	"bloggerout/internal/virtualfs"
)

// XML structure for the Blogger feed
type feed struct {
	XMLName xml.Name `xml:"feed"`
	Title   string   `xml:"title"`
	Entries []entry  `xml:"entry"`
}

// XML structure for each entry in the feed
type entry struct {
	ID         string    `xml:"id"`
	ParentID   string    `xml:"parent"`
	Type       string    `xml:"type"`
	AuthorName string    `xml:"author>name"`
	Published  time.Time `xml:"published"`
	Updated    time.Time `xml:"updated"`
	Title      string    `xml:"title"`
	Content    string    `xml:"content"`
	Status     string    `xml:"status"`
	URL        string    `xml:"filename"` // This is the URL of the post
	Categories []string  `xml:"-"`        // We'll populate this with a special UnmarshalXML
}

// Custom unmarshaling for the entry struct to handle the categories
func (e *entry) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type Alias entry // Create an alias to avoid recursion
	aux := &struct {
		Categories []struct {
			Term string `xml:"term,attr"`
		} `xml:"category"`
		*Alias
	}{
		Alias: (*Alias)(e),
	}

	if err := d.DecodeElement(aux, &start); err != nil {
		return err
	}

	// Extract the `term` attributes into the `Categories` field
	for _, cat := range aux.Categories {
		e.Categories = append(e.Categories, cat.Term)
	}

	return nil
}

func processFeedAtom(blog *Blog, vfs virtualfs.FileSystem, path string) (map[string]Post, error) {
	f, err := vfs.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var feed feed
	if err := xml.NewDecoder(f).Decode(&feed); err != nil {
		return nil, err
	}

	// Process posts
	posts := make(map[string]Post, len(feed.Entries))
	for _, entry := range feed.Entries {
		if entry.Type == "POST" {
			posts[entry.ID] = Post{
				id:         entry.ID,
				Title:      html.UnescapeString(entry.Title),
				Content:    html.UnescapeString(entry.Content),
				Date:       entry.Published,
				Updated:    entry.Updated,
				Author:     html.UnescapeString(entry.AuthorName),
				Categories: entry.Categories,
				URL:        blog.BaseURL + entry.URL,
				Draft:      entry.Status != "LIVE",
			}
		}
	}

	// Process Post's comments
	for _, entry := range feed.Entries {
		if entry.Type == "COMMENT" {
			p := posts[entry.ParentID]
			p.Comments = append(p.Comments, Comment{
				Date:   entry.Published,
				Author: entry.AuthorName,
				Text:   html.UnescapeString(entry.Content),
			})
			posts[entry.ParentID] = p
		}
	}
	return posts, nil
}
