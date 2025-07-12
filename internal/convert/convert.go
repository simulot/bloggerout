package convert

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"text/template"

	"bloggerout/internal/downloader"
	"bloggerout/internal/takeout"
	"bloggerout/internal/worker"

	"github.com/spf13/cobra"
)

type Convert struct {
	data          *takeout.BloggerTakeout
	blogs         []string
	outPath       string
	selectPattern string
	takeoutPath   string
	hugoPath      string
	hugoPathTmpl  *template.Template // hugo path template
	imagePath     string             // image path flag
	imagePathTmpl *template.Template // image path template
	postPath      string             // post path flag
	postPathTmpl  *template.Template // post template

	workers    *worker.WorkerPool
	downloader *downloader.Downloader
	imageChan  chan image
}

func ConverCommand() *cobra.Command {
	c := &Convert{}

	cmd := &cobra.Command{
		Use:   "convert",
		Short: "Convert the contents of the Blogger Takeout file to Hugo-compatible files",
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			if c.takeoutPath == "" {
				fmt.Println("Missing --takeout flag. Usage: bloggerout convert --takeout <path-to-blogger-takeout-file> --hugo <path-to-hugo-blogs> [--select <pattern>] [other flags]")
				os.Exit(1)
			}
			if c.hugoPath == "" {
				fmt.Println("Missing --hugo flag. Usage: bloggerout convert --takeout <path-to-blogger-takeout-file> --hugo <path-to-hugo-blogs> [--select <pattern>] [other flags]")
				os.Exit(1)
			}
			c.hugoPathTmpl, err = template.New("hugoPath").Parse(c.hugoPath)
			if err != nil {
				fmt.Println("Error parsing Hugo path template:", err)
				os.Exit(1)
				return
			}

			if c.imagePath != "" {
				c.imagePathTmpl, err = template.New("imagePath").Parse(c.imagePath)
				if err != nil {
					fmt.Println("Error parsing image path template:", err)
					os.Exit(1)
				}
			}
			if c.postPath != "" {
				c.postPathTmpl, err = template.New("postPath").Parse(c.postPath)
				if err != nil {
					fmt.Println("Error parsing post path template:", err)
					os.Exit(1)
				}
			}

			ctx := context.Background()
			takeoutData, err := takeout.ReadTakeout(ctx, []string{c.takeoutPath})
			if err != nil {
				fmt.Printf("Error reading takeout: %v\n", err)
				os.Exit(1)
			}
			c.data = takeoutData

			if c.selectPattern != "" {
				c.blogs = nil
				for name := range takeoutData.Blogs {
					if c.selectPattern == "*" {
						c.blogs = append(c.blogs, name)
						continue
					} else if strings.Contains(name, c.selectPattern) || name == c.selectPattern {
						c.blogs = append(c.blogs, name)
					}
				}
				if len(c.blogs) == 0 {
					fmt.Println("No blogs found matching the pattern.")
					os.Exit(1)
				}
			} else {
				fmt.Println("Missing the blog name pattern. Use --select * to export all blogs.")
				os.Exit(1)
			}
			c.imagePathTmpl, err = template.New("imagePath").Parse(c.imagePath)
			if err != nil {
				fmt.Printf("Error can't parse image path template: %v\n", err)
				os.Exit(1)
			}

			c.postPathTmpl, err = template.New("postPath").Parse(c.postPath)
			if err != nil {
				fmt.Printf("Error can't parse post path template: %v\n", err)
				os.Exit(1)
			}
			err = c.Convert(ctx)
			if err != nil {
				fmt.Printf("Error can't  convert the blog: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVarP(&c.selectPattern, "select", "s", "", "Blog name or pattern to select specific blogs, '*' to select all blogs")
	cmd.Flags().StringVar(&c.takeoutPath, "takeout", "", "Path to Blogger Takeout file (required)")
	cmd.Flags().StringVar(&c.hugoPath, "hugo", "", "Path to Hugo blogs directory (required)")
	cmd.Flags().StringVar(&c.imagePath, "image-path", "/static/images", "Path template for images inside hugo directory (default: /static/images)")
	cmd.Flags().StringVar(&c.postPath, "post-path", "/content/posts", "Path template for posts inside hugo directory (default: /content/posts)")
	cmd.MarkFlagRequired("takeout")
	cmd.MarkFlagRequired("hugo")

	return cmd
}

func (c *Convert) Convert(ctx context.Context) error {
	var err error
	c.workers = worker.NewWorkerPool(10)
	c.downloader = downloader.NewDownloader()

	go func(errs <-chan error) {
		for err := range errs {
			err = errors.Join(err, err)
		}
	}(c.workers.Errors())
	c.workers.Start(ctx)

	for _, blog := range c.blogs {
		err = errors.Join(err, c.converBlog(ctx, blog))
	}
	c.workers.Stop()
	return err
}

func (c *Convert) converBlog(ctx context.Context, blog string) error {
	var err error

	for i, post := range c.data.Blogs[blog].Posts {
		if !post.Draft {
			err = errors.Join(err, c.convertPost(ctx, blog, post))
			if i >= 1 {
				break
			}
		}
	}
	return err
}

func (c *Convert) pushImage(ctx context.Context, img image) error {
	// Check if the image is already present in the Hugo directory
	hugoPath, err := tmplExecInString(c.hugoPathTmpl, img)
	if err != nil {
		return fmt.Errorf("can't determine Hugo path for image %s: %w", img.Name, err)
	}

	imgPath := path.Join(hugoPath, img.StaticName)
	_, err = os.Stat(imgPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("can't check if image %s exists: %w", img.Name, err)
		}
	}
	if err == nil {
		// Image already in Hugo directory
		return nil
	}

	// Check if the image is already in the takeout
	base := path.Base(img.Source)
	_, err = c.data.StatImage(img.Blog, base)
	if err == nil {
		c.workers.Submit(func(ctx context.Context) error {
			return c.copyFromTakeout(ctx, img)
		})
		return nil
	}

	// Images not in the takeout are downloaded
	c.workers.Submit(func(ctx context.Context) error {
		err := c.downloader.DownloadFile(ctx, img.Source, imgPath)
		if err != nil {
			return fmt.Errorf("can't download image %s: %w", img.Name, err)
		}
		return nil
	})
	return nil
}

func (c *Convert) copyFromTakeout(ctx context.Context, img image) error {
	destName, err := tmplExecInString(c.hugoPathTmpl, img)
	if err != nil {
		return fmt.Errorf("can't create image path %s: %w", img.Name, err)
	}
	destName = path.Join(destName, img.Name)

	src, err := c.data.OpenImage(img.Blog, img.Name)
	if err != nil {
		return fmt.Errorf("can't open image %s: %w", img.Name, err)
	}
	defer src.Close()

	dest, err := os.Create(destName)
	if err != nil {
		return fmt.Errorf("failed to create image %s: %w", img.Name, err)
	}
	defer dest.Close()
	_, err = io.Copy(dest, src)
	return err
}

func tmplExecInString(tmpl *template.Template, data interface{}) (string, error) {
	var sb strings.Builder
	err := tmpl.Execute(&sb, data)
	if err != nil {
		return "", err
	}
	return sb.String(), nil
}
