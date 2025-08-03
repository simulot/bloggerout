package blogger

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"time"

	"bloggerout/internal/takeout/resources"
	"bloggerout/internal/virtualfs"
)

// BloggerTakeout represents the structure of the Blogger Takeout data.
type BloggerTakeout struct {
	vfs        virtualfs.FileSystem
	ressources *resources.Resources
	Blogs      map[string]*Blog // all blogs by Blog name
}

func New(vfs virtualfs.FileSystem, ressources *resources.Resources) *BloggerTakeout {
	return &BloggerTakeout{
		vfs:        vfs,
		ressources: ressources,
		Blogs:      make(map[string]*Blog),
	}
}

// Blog represents a single blog extracted from takeout
type Blog struct {
	Title       string // file settings.csv -> blog_name
	Description string // file settings.csv -> blog_description
	Domain      string // file settings.csv -> blog_publishing_mode
	SubDomain   string // file settings.csv -> blog_subdomain
	BaseURL     string
	Posts       map[string]Post // map by id, from file feed.atom
}

// BlogPost represents a single blog post extracted from the feed.atom file.
type Post struct {
	id         string // blogger ID to discriminate duplicated posts
	Title      string
	Content    string
	Date       time.Time
	Updated    time.Time // we keep only the last updated version
	Author     string
	Categories []string
	Comments   []Comment
	URL        string
	Draft      bool
}

type Comment struct {
	Date   time.Time
	Author string
	Text   string
}

func (to *BloggerTakeout) Scan(ctx context.Context, path string) (*BloggerTakeout, error) {
	dirs, err := fs.ReadDir(to.vfs, path)
	if err != nil {
		return nil, err
	}

	for _, dir := range dirs {
		if dir.IsDir() {
			switch dir.Name() {
			case "Albums/":
				err = to.scanAlbums(ctx, path+"/Albums")
			case "Blogs/":
				err = to.scanBlogs(ctx, path+"/Blogs")
			}
			if err != nil {
				return nil, err
			}
		}
	}

	return to, nil
}

// scanBlogs scans the blogs in the directory
func (to *BloggerTakeout) scanBlogs(_ context.Context, filePath string) error {
	blogs, err := fs.ReadDir(to.vfs, filePath)
	if err != nil {
		return err
	}
	for _, b := range blogs {
		if !b.IsDir() {
			// should not happen
			continue
		}
		blog, err := readSettingsCSV(to.vfs, path.Join(filePath, b.Name(), "settings.csv"))
		if err != nil {
			return err
		}

		blog.Posts, err = processFeedAtom(blog, to.vfs, path.Join(filePath, b.Name(), "feed.atom"))
		if err != nil {
			return err
		}
		to.Blogs[blog.Title] = blog

		// search images and videos
		files, err := fs.ReadDir(to.vfs, path.Join(filePath, b.Name()))
		if err != nil {
			return err
		}

		for _, file := range files {
			if file.IsDir() {
				// should not happen
				continue
			}
			switch file.Name() {
			case "settings.csv", "feed.atom":
				continue
			default:
				to.ressources.Add(to.vfs, blog.Title, path.Join(filePath, b.Name()), file, nil)
			}
		}
	}
	return nil
}

func readSettingsCSV(vfs virtualfs.FileSystem, path string) (*Blog, error) {
	f, err := vfs.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	csvReader := csv.NewReader(f)
	headers, err := csvReader.Read()
	if err != nil {
		return nil, err
	}
	fields, err := csvReader.Read()
	if err != nil {
		return nil, err
	}

	blog := &Blog{
		Posts: make(map[string]Post),
	}

	for i, field := range headers {
		switch field {
		case "blog_name":
			blog.Title = fields[i]
		case "blog_description":
			blog.Description = fields[i]
		case "blog_publishing_mode":
			if fields[i] == "BLOGSPOT" {
				blog.Domain = "blogspot.com"
			}
		case "blog_subdomain":
			blog.SubDomain = fields[i]
		}
	}
	if blog.Domain != "" {
		blog.BaseURL = "https://" + blog.SubDomain + "." + blog.Domain
	}

	return blog, nil
}

func (to *BloggerTakeout) scanAlbums(ctx context.Context, filePath string) error {
	albums, err := to.vfs.ReadDir(filePath)
	if err != nil {
		return err
	}

	for _, a := range albums {
		if !a.IsDir() {
			continue
		}

		files, err := to.vfs.ReadDir(path.Join(filePath, a.Name()))
		if err != nil {
			return err
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}
			ext := strings.ToLower(path.Ext(file.Name()))
			if ext == ".json" {
				continue
			}

			meteDataName := file.Name() + ".json"
			// read the metadata.json file
			md, err := readMetadataJSON(to.vfs, path.Join(filePath, a.Name(), meteDataName))
			if err != nil {
				md = nil
			}

			// anything else is a resource
			to.ressources.Add(to.vfs, a.Name(), path.Join(filePath, a.Name()), file, md)
		}
	}
	return nil
}

func readMetadataJSON(vfs virtualfs.FileSystem, path string) (*resources.ResourceMetadata, error) {
	file, err := vfs.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var raw struct {
		// UploadStatus        string `json:"uploadStatus"`        // "FULL_QUALITY"
		SizeBytes           string `json:"sizeBytes"`           // "1641075"
		Filename            string `json:"filename"`            // "IMG_0123.jpg"
		CreationTimestampMs string `json:"creationTimestampMs"` // "1751181639509"
		// HasOriginalBytes    string `json:"hasOriginalBytes"`    // "YES"
		// ContentVersion      string `json:"contentVersion"`      // "124564"
		MimeType string `json:"mimeType"` // "image/*"
	}

	err = json.NewDecoder(file).Decode(&raw)
	if err != nil {
		return nil, err
	}

	md := &resources.ResourceMetadata{
		Filename: raw.Filename,
		MimeType: raw.MimeType,
	}
	md.SizeBytes, _ = strconv.ParseInt(raw.SizeBytes, 10, 64)
	creationTimestampMs, _ := strconv.ParseInt(raw.CreationTimestampMs, 10, 64)
	md.CreationTimestamp = time.UnixMilli(creationTimestampMs)
	return md, nil
}
