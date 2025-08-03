package blogger

import (
	"encoding/xml"
	"os"
	"testing"
)

func TestParseFeed(t *testing.T) {
	// Open the feed.atom file
	file, err := os.Open("./data/Takeout/Blogger/Blogs/Blog Experience/feed.atom")
	if err != nil {
		t.Fatalf("failed to open feed.atom file: %v", err)
	}
	defer file.Close()

	// Parse the XML
	var feed feed
	decoder := xml.NewDecoder(file)
	err = decoder.Decode(&feed)
	if err != nil {
		t.Fatalf("failed to parse feed.atom file: %v", err)
	}

	// Validate the parsed data
	if feed.Title == "" {
		t.Errorf("expected feed title to be non-empty")
	}

	if len(feed.Entries) == 0 {
		t.Errorf("expected feed to have entries")
	}

	for _, entry := range feed.Entries {
		if entry.AuthorName == "" {
			t.Errorf("expected AuthorName author name to be non-empty")
		}
		if entry.Published.IsZero() {
			t.Errorf("expected entry published date to be non-empty")
		}
		if entry.Updated.IsZero() {
			t.Errorf("expected entry updated date to be non-empty")
		}
		// if entry.Title == "" && entry.Type == "POST" {
		// 	t.Errorf("expected entry title to be non-empty")
		// }
		if entry.Content == "" {
			t.Errorf("expected entry content to be non-empty")
		}

		if len(entry.Categories) == 0 {
			t.Errorf("expected entry to have categories")
		}
	}
}
