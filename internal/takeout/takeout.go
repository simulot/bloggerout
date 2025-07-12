package takeout

import (
	"context"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"html"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"bloggerout/internal/virtualfs"
)

type Takeout struct {
	Blogger *BloggerTakeout
	YouTube *YouTubeTakeout
	vfs     virtualfs.FileSystem
}

// ReadTakeout read the Blogger Takeout file or folder.
// depending on the CLI invokation, a directory or a glob of zip files can be provided.
// the function will determine the type of input and process it accordingly.
func ReadTakeout(ctx context.Context, inputPaths []string) (*Takeout, error) {
	if len(inputPaths) == 0 {
		return nil, fmt.Errorf("no input paths provided")
	}

	// got something like the result of a glob, check if all are zip files
	allZip := true
	for _, path := range inputPaths {
		if strings.ToLower(filepath.Ext(path)) != ".zip" {
			allZip = false
			break
		}
	}

	if allZip {
		vfs, err := virtualfs.NewZipFileSystem(inputPaths...)
		if err != nil {
			return nil, err
		}
		return processDirectory(ctx, vfs)
	} else {
		if len(inputPaths) != 1 {
			return nil, fmt.Errorf("only one input path is supported")
		}
		s, err := os.Stat(inputPaths[0])
		if err != nil {
			return nil, err
		}
		if s.IsDir() {
			// Process directory
			vfs, err := virtualfs.NewOSFileSystem(inputPaths[0])
			if err != nil {
				return nil, err
			}
			return processDirectory(ctx, vfs)
		}
	}
	return nil, fmt.Errorf("unsupported file type: %s", inputPaths[0])
}

func processDirectory(ctx context.Context, vfs virtualfs.FileSystem) (*BloggerTakeout, error) {
	to := &BloggerTakeout{
		Blogs:  make(map[string]Blog),
		Albums: make(map[string]Album),
		vfs:    vfs,
	}

	blogger := false
	fs.WalkDir(vfs, ".", func(path string, d fs.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err != nil {
				return err
			}
			if d.IsDir() {
				dir, base := filepath.Split(path)
				dir = filepath.Base(dir)
				switch filepath.Base(base) {
				case "Blogger":
					blogger = true
					return nil // Continue processing subdirectories of Blogger
				default:
					if !blogger {
						return nil
					}
					switch dir {
					case "Blogs":
						err := to.processBlog(ctx, vfs, path)
						if err != nil {
							slog.Error("failed to process the blog", "path", path, "error", err)
						}
					case "Albums":
						err := to.processAlbum(ctx, vfs, path)
						if err != nil {
							slog.Error("failed to process the album", "path", path, "error", err)
						}
					default:
						return nil
					}
				}
			}
		}
		// ignoring anything else
		return nil
	})
	return to, nil
}

func (to *BloggerTakeout) processBlog(ctx context.Context, vfs virtualfs.FileSystem, path string) error {
	files, err := vfs.ReadDir(path)
	if err != nil {
		return err
	}

	blog := Blog{}
	for _, file := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if file.IsDir() {
				continue
			}

			switch file.Name() {
			case "settings.csv":
				blog.Title, blog.Description, err = processSettings(vfs, filepath.Join(path, file.Name()))
				if err != nil {
					slog.Error("failed to process settings.csv", "path", file.Name(), "error", err)
				}
			case "feed.atom":
				blog.Posts, err = processFeed(vfs, filepath.Join(path, file.Name()))
				if err != nil {
					slog.Error("failed to process feed.atom", "path", file.Name(), "error", err)
				}
			default:
				slog.Debug("skipping file", "path", file.Name())
			}

		}
	}
	to.Blogs[filepath.Base(path)] = blog
	return nil
}

func processSettings(vfs virtualfs.FileSystem, path string) (name string, desciption string, err error) {
	f, err := vfs.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	csvReader := csv.NewReader(f)
	headers, err := csvReader.Read()
	if err != nil {
		return "", "", err
	}
	fields, err := csvReader.Read()
	if err != nil {
		return "", "", err
	}

	for i, field := range headers {
		switch field {
		case "blog_name":
			name = fields[i]
		case "blog_description":
			desciption = fields[i]
		}
	}
	return
}

func processFeed(vfs virtualfs.FileSystem, path string) ([]Post, error) {
	f, err := vfs.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var feed Feed
	if err := xml.NewDecoder(f).Decode(&feed); err != nil {
		return nil, err
	}
	posts := make([]Post, len(feed.Entries))
	for i, entry := range feed.Entries {
		posts[i] = Post{
			Title:   html.UnescapeString(entry.Title),
			Content: html.UnescapeString(entry.Content),
			Date:    entry.Published,
			Author:  html.UnescapeString(entry.AuthorName),
			Draft:   entry.Status != "LIVE",
		}
	}
	return posts, nil
}

func (to *BloggerTakeout) processAlbum(ctx context.Context, vfs virtualfs.FileSystem, path string) error {
	files, err := vfs.ReadDir(path)
	if err != nil {
		return err
	}

	album := Album{
		Title: filepath.Base(path),
	}
	for _, file := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if file.IsDir() {
				continue
			}
			if filepath.Ext(file.Name()) != ".json" {
				album.Content = append(album.Content, filepath.Join(path, file.Name()))
				continue
			}
		}
	}
	to.Albums[album.Title] = album
	return nil
}
