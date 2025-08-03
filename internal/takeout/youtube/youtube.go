package youtube

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"bloggerout/internal/takeout/resources"
	"bloggerout/internal/virtualfs"

	"github.com/simulot/TakeoutLocalization/go/localization"
)

// https://developers.google.com/data-portability/schema-reference/youtube

type Video struct {
	ID        string
	ChannelID string
	Title     string              // Title as found in the metadata
	Resource  *resources.Resource // The link to the resource in the virtual file system
	FileName  string
}

type Channel struct {
	ID     string
	Title  string
	Videos map[string]*Video // Map of video IDs to Video structs
}

type YouTubeTakeout struct {
	vfs           virtualfs.FileSystem
	loc           *localization.Products
	resources     *resources.Resources
	channels      map[string]Channel
	videosByID    map[string]*Video // Map of video IDs to Video structs
	videosByTitle map[string]*Video // Map of video titles to Video structs
}

func New(vfs virtualfs.FileSystem, resources *resources.Resources, loc *localization.Products) *YouTubeTakeout {
	return &YouTubeTakeout{
		vfs:           vfs,
		resources:     resources,
		channels:      make(map[string]Channel),
		videosByID:    make(map[string]*Video),
		videosByTitle: make(map[string]*Video),
		loc:           loc,
	}
}

func (to *YouTubeTakeout) Scan(ctx context.Context, filePath string, globalizedPath string) (*YouTubeTakeout, error) {
	dirs, err := fs.ReadDir(to.vfs, filePath)
	if err != nil {
		return nil, err
	}

	// Map the takeouts directories with their global names
	localizedPaths := map[string]string{}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		key, _ := to.loc.Globalize(path.Join(globalizedPath, strings.TrimSuffix(d.Name(), "/")))
		switch key {
		case "channels":
			localizedPaths["channels"] = path.Join(filePath, d.Name())
		case "video metadata":
			localizedPaths["video metadata"] = path.Join(filePath, d.Name())
		case "videos":
			localizedPaths["videos"] = path.Join(filePath, d.Name())
		}
	}

	// Process localized folders in a specific order

	err = to.scanChannels(ctx, to.vfs, localizedPaths["channels"], path.Join(globalizedPath, "channels"))
	if err != nil {
		return nil, err
	}
	err = to.scanVideoMetadata(ctx, to.vfs, localizedPaths["video metadata"], path.Join(globalizedPath, "video metadata"))
	if err != nil {
		return nil, err
	}
	err = to.scanVideos(ctx, to.vfs, localizedPaths["videos"])
	if err != nil {
		return nil, err
	}
	return to, nil
}

func (to *YouTubeTakeout) scanChannels(ctx context.Context, vfs virtualfs.FileSystem, filePath string, globaliziedPath string) error {
	if filePath == "" {
		return fmt.Errorf("no 'channels' folder found in the YouTube Takeout. Check localization")
	}

	files, err := fs.ReadDir(vfs, filePath)
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		key, node := to.loc.Globalize(path.Join(globaliziedPath, f.Name()))
		if key == "channel.csv" {
			err = readCSV(ctx, vfs, path.Join(filePath, f.Name()), node, func(cols map[string]int, r []string) {
				c := Channel{
					ID:     r[cols["Channel ID"]],
					Title:  r[cols["Channel Title (Original)"]],
					Videos: make(map[string]*Video),
				}
				to.channels[c.ID] = c
			})
		}
	}
	return nil
}

func (to *YouTubeTakeout) scanVideoMetadata(ctx context.Context, vfs virtualfs.FileSystem, filePath string, globaliziedPath string) error {
	if filePath == "" {
		return fmt.Errorf("no 'video metadata' folder found in the YouTube Takeout. Check localization")
	}
	// List all channels in the channels folde
	files, err := fs.ReadDir(vfs, filePath)
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		key, n := to.loc.Globalize(path.Join(globaliziedPath, f.Name()))
		if key == "videos.csv" {
			err = readCSV(ctx, vfs, path.Join(filePath, f.Name()), n, func(cols map[string]int, r []string) {
				v := &Video{
					ID:        r[cols["Video ID"]],
					ChannelID: r[cols["Channel ID"]],
					Title:     r[cols["Video Title (Original)"]],
				}
				if c, ok := to.channels[v.ChannelID]; ok {
					c.Videos[v.ID] = v
				}
				to.videosByID[v.ID] = v
				to.videosByTitle[v.Title] = v
			})
		}

	}
	return nil
}

func (to *YouTubeTakeout) scanVideos(ctx context.Context, vfs virtualfs.FileSystem, filePath string) error {
	if filePath == "" {
		return fmt.Errorf("no 'videos' folder found in the YouTube Takeout. Check localization")
	}
	files, err := fs.ReadDir(vfs, filePath)
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		title := strings.TrimSuffix(f.Name(), path.Ext(f.Name()))
		if v, ok := to.videosByTitle[title]; ok {
			v.FileName = path.Join(filePath, f.Name())
			r := to.resources.Add(to.vfs, v.ChannelID, filePath, f, nil)
			v.Resource = r
			to.videosByID[v.ID] = v
		}
	}
	return nil
}

func (to *YouTubeTakeout) SearchByID(ctx context.Context, id string) *Video {
	return to.videosByID[id]
}
