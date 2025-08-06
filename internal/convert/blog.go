package convert

import (
	"context"
	"embed"
	"io"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path"
	"slices"
	"strings"
	"text/template"

	"bloggerout/internal/downloader"
	"bloggerout/internal/takeout"
	"bloggerout/internal/takeout/blogger"
	"bloggerout/internal/worker"
)

//go:embed _shortcodes/*
var shortCodesFS embed.FS

type blogConverter struct {
	*Convert
	data       *takeout.Takeout
	blog       string
	workers    *worker.WorkerPool
	downloader *downloader.Downloader
	report     HugoPost
	rfs        *os.Root
}

func newBlogConverter(ctx context.Context, c *Convert, data *takeout.Takeout, blog string) error {
	blogPath, err := prepareFileName(c.hugoPathTmpl, map[string]string{"Blog": blog})
	if err != nil {
		return err
	}

	_, err = os.Stat(blogPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		err := os.MkdirAll(blogPath, 0o755)
		if err != nil {
			return err
		}
	}

	rfs, err := os.OpenRoot(blogPath)
	if err != nil {
		return err
	}

	err = copyShortCodes(rfs, shortCodesFS, "layouts")

	bc := &blogConverter{
		Convert:    c,
		data:       data,
		blog:       blog,
		workers:    worker.NewWorkerPool(10),
		downloader: downloader.NewDownloader(),
		rfs:        rfs,
	}
	return bc.convertBlog(ctx, blog, data.Blogger.Blogs[blog])
}

func (bc *blogConverter) convertBlog(ctx context.Context, blog string, blogData *blogger.Blog) error {
	bc.workers.Start(ctx)
	defer bc.workers.Stop()

	keys := slices.Sorted(maps.Keys(blogData.Posts))
	for _, k := range keys {
		post := blogData.Posts[k]
		// if post.Title != "Paul pati Grand-Père" {
		// 	continue
		// }
		if !post.Draft {
			bc.workers.Submit(func(ctx context.Context) {
				err := bc.newPostConverter(ctx, post)
				if err != nil {
					slog.Error("Error converting post", "error", err, "post", post.Title, "date", post.Date.Format("2006-01-02"))
				}
			})
		}
	}
	return nil
}

type blogLogMessage struct {
	Kind    string // Info, Warning, Error
	Message string
}

func prepareFileName(tmpl *template.Template, pathContext any) (string, error) {
	var sb strings.Builder
	err := tmpl.Execute(&sb, pathContext)
	if err != nil {
		return "", err
	}
	fileName := strings.TrimPrefix(sb.String(), "/")
	return fileName, nil
}

func mkDirAll(rfs *os.Root, dirName string) error {
	dirName = strings.TrimPrefix(dirName, "/")
	if dirName == "" {
		return nil
	}

	parts := strings.Split(dirName, "/")
	if parts[0] == "/" {
		parts = parts[1:]
	}
	dirName = ""
	for _, part := range parts {
		dirName = path.Join(dirName, part)
		err := rfs.Mkdir(dirName, 0o755)
		if err != nil && !os.IsExist(err) {
			return err
		}
	}
	return nil
}

// copyShortCodes copies the shortcodes from the source code to the destination.
func copyShortCodes(dest *os.Root, src fs.FS, destPath string) error {
	err := dest.Mkdir(destPath, 0o755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	return fs.WalkDir(src, ".", func(name string, d fs.DirEntry, err error) error {
		destName := path.Join(destPath, name)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name != "." {
				err = dest.Mkdir(destName, 0o755)
				if err != nil && !os.IsExist(err) {
					return err
				}
			}
			return nil
		}
		// _, err = dest.Stat(destName)
		// if err == nil {
		// 	return nil
		// }

		sf, err := src.Open(name)
		if err != nil {
			return err
		}
		defer sf.Close()
		df, err := dest.Create(path.Join(destPath, name))
		if err != nil {
			return err
		}
		defer df.Close()
		_, err = io.Copy(df, sf)
		return err
	})
}
