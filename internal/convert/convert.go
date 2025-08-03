package convert

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/template"

	"bloggerout/internal/takeout"

	"github.com/spf13/cobra"
)

type Convert struct {
	data           *takeout.Takeout
	blogs          []string
	outPath        string
	selectPattern  string
	takeoutPath    []string
	hugoPath       string
	hugoPathTmpl   *template.Template // hugo path template
	imagePath      string             // image path flag
	imagePathTmpl  *template.Template // image path template
	postPath       string             // post path flag
	postPathTmpl   *template.Template // post template
	reportPath     string             // report path flag
	reportPathTmpl *template.Template // report template

	// workers    *worker.WorkerPool
	// downloader *downloader.Downloader
	// logMessages *logMessages
}

func ConverCommand() *cobra.Command {
	c := &Convert{}

	cmd := &cobra.Command{
		Use:   "convert",
		Short: "Convert the contents of the Blogger Takeout file to Hugo-compatible files",
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			if len(c.takeoutPath) == 0 {
				fmt.Println("Missing --takeout flag. Usage: bloggerout convert --takeout <path-to-blogger-takeout-file> [--takeout <path-to-blogger-takeout-file> ...] --hugo <path-to-hugo-blogs> [--select <pattern>] [other flags]")
				os.Exit(1)
			}
			if c.hugoPath == "" {
				fmt.Println("Missing --hugo flag. Usage: bloggerout convert --takeout <path-to-blogger-takeout-file> --takeout <path-to-blogger-takeout-file> ...] --hugo <path-to-hugo-blogs> [--select <pattern>] [other flags]")
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
			takeoutData, err := takeout.ReadTakeout(ctx, c.takeoutPath)
			if err != nil {
				fmt.Printf("Error reading takeout: %v\n", err)
				os.Exit(1)
			}
			c.data = takeoutData

			if c.selectPattern != "" {
				c.blogs = nil
				for name := range takeoutData.Blogger.Blogs {
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
			// c.imagePathTmpl, err = template.New("imagePath").Parse(c.imagePath)
			// if err != nil {
			// 	fmt.Printf("Error can't parse image path template: %v\n", err)
			// 	os.Exit(1)
			// }

			c.postPathTmpl, err = template.New("postPath").Parse(c.postPath)
			if err != nil {
				fmt.Printf("Error can't parse post path template: %v\n", err)
				os.Exit(1)
			}
			c.reportPathTmpl, err = template.New("reportPath").Parse(c.reportPath)
			if err != nil {
				fmt.Printf("Error can't parse report path template: %v\n", err)
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
	cmd.Flags().StringSliceVar(&c.takeoutPath, "takeout", nil, "Path to Takeout file, can be specified multiple times (required)")
	cmd.Flags().StringVar(&c.hugoPath, "hugo", "", "Path to Hugo blogs directory (required)")
	// cmd.Flags().StringVar(&c.imagePath, "image-path", "/static/images", "Path template for images inside hugo directory (default: /static/images)")
	cmd.Flags().StringVar(&c.postPath, "post-path", "/content/posts/{{ .Title }}/", "Path template for posts inside hugo directory (default: /content/posts/{{ .Title }}/)")
	cmd.Flags().StringVar(&c.reportPath, "report-path", "/content/reports", "Path template for posting import reports inside hugo directory (default: /content/report)")
	cmd.MarkFlagRequired("takeout")
	cmd.MarkFlagRequired("hugo")

	return cmd
}

func (c *Convert) Convert(ctx context.Context) error {
	var err error
	for _, blog := range c.blogs {
		err1 := newBlogConverter(ctx, c, c.data, blog)
		if err1 != nil {
			err = errors.Join(err, err1)
		}
	}
	return err
}
