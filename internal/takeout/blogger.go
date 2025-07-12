package takeout

import (
	"bloggerout/internal/virtualfs"
	"fmt"
	"io/fs"
	"path"
	"time"
)

// BloggerTakeout represents the structure of the Blogger Takeout data.
type BloggerTakeout struct {
	Blogs  map[string]Blog  // all blogs
	Albums map[string]Album // all albums, contain images
}

// Album represents a single album extracted from takeout
type Album struct {
	Title   string
	Content []string
}

// Blog represents a single blog extracted from takeout
type Blog struct {
	Title       string // file settings.csv -> blog_name
	Description string // file settings.csv -> blog_description
	Posts       []Post // file feed.atom
}

// BlogPost represents a single blog post extracted from the feed.atom file.
type Post struct {
	Title      string
	Content    string
	Date       time.Time
	Author     string
	Categories []string
	Draft      bool
}

func (to *BloggerTakeout) StatImage(vfs virtualfs.FileSystem blog string, image string) (fs.FileInfo, error) {
	album, ok := to.Albums[blog]
	if !ok {
		return nil, fmt.Errorf("album not found: %s", blog)
	}
	for _, content := range album.Content {
		if path.Base(content) == image {
			return to.vfs.Stat(content)
		}
	}
	return nil, fmt.Errorf("imaghe not found: %s", image)
}

func (to *BloggerTakeout) OpenImage(blog string, image string) (fs.File, error) {
	_, err := to.StatImage(blog, image)
	if err != nil {
		return nil, err
	}
	album, ok := to.Albums[blog]
	if !ok {
		return nil, fmt.Errorf("album not found: %s", blog)
	}

	for _, content := range album.Content {
		if path.Base(content) == image {
			return to.vfs.Open(content)
		}
	}
	return nil, fmt.Errorf("image not found: %s", image)
}
