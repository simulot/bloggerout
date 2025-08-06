package convert

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"bloggerout/internal/filename"
	"bloggerout/internal/takeout/blogger"

	"github.com/JohannesKaufmann/dom"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"golang.org/x/net/html"
	"gopkg.in/yaml.v3"
)

type postConverter struct {
	*blogConverter

	resources map[string]*resource
	hp        HugoPost

	url        string
	errors     map[string]int
	checkImage bool
	comments   []blogger.Comment
	post       blogger.Post
	path       string   // Path to the post directory
	pfs        *os.Root // post's file root
	mdc        *converter.Converter
}

func (bc *blogConverter) newPostConverter(ctx context.Context, post blogger.Post) error {
	pc := &postConverter{
		blogConverter: bc,
		post:          post,
		errors:        make(map[string]int),
		resources:     make(map[string]*resource),
	}
	return pc.convertPost(ctx, post)
}

type HugoPost struct {
	Blog    string
	Title   string
	Date    time.Time
	Draft   bool
	Tags    []string
	Params  map[string]string
	content string
}

func (pc *postConverter) convertPost(ctx context.Context, p blogger.Post) error {
	pc.hp = HugoPost{
		Blog:  pc.blog,
		Title: p.Title,
		Date:  p.Date,
		Draft: p.Draft,
		Tags:  p.Categories,
		Params: map[string]string{
			"author": p.Author,
		},
	}
	pc.url = p.URL

	if pc.hp.Title == "" {
		pc.hp.Title = pc.hp.Date.Format("2006-01-02") + " - Untitled"
	}

	destPath, err := prepareFileName(pc.postPathTmpl, map[string]any{
		"Blog":   filename.Sanitize(pc.hp.Blog),
		"Date":   pc.hp.Date,
		"Author": filename.Sanitize(pc.hp.Params["author"]),
	})
	if err != nil {
		return fmt.Errorf("can't prepare post file name: %w", err)
	}

	// Post path
	destPath = path.Join(destPath, pc.hp.Date.Format("2006-01-02")+" "+filename.Sanitize(pc.hp.Title))

	_, err = pc.rfs.Stat(destPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("can't check if Hugo post file exists: %w", err)
		}
		err = mkDirAll(pc.rfs, destPath)
		if err != nil {
			return fmt.Errorf("can't create Hugo post directory: %w", err)
		}
	}

	pc.pfs, err = pc.rfs.OpenRoot(destPath)
	if err != nil {
		return fmt.Errorf("can't create Hugo post directory: %w", err)
	}

	mdc := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
		),
	)

	// handlers for Blogger tags
	mdc.Register.RendererFor("table", converter.TagTypeInline, pc.tableHandler, converter.PriorityEarly)
	mdc.Register.RendererFor("a", converter.TagTypeInline, pc.anchorImgHandler, converter.PriorityEarly)
	mdc.Register.RendererFor("img", converter.TagTypeInline, pc.imageHandler, converter.PriorityEarly)
	mdc.Register.RendererFor("iframe", converter.TagTypeBlock, pc.iFrameHandler, converter.PriorityEarly)
	mdc.Register.RendererFor("object", converter.TagTypeBlock, pc.videoObjectHandler, converter.PriorityEarly)

	// capture tags not handled by the other handlers
	mdc.Register.RendererFor("a", converter.TagTypeInline, pc.anchorHandler, converter.PriorityEarly+10)

	// let's the magic happening
	pc.hp.content, err = mdc.ConvertString(p.Content, converter.WithContext(ctx))
	if err != nil {
		return err
	}

	for _, c := range p.Comments {
		text, err := mdc.ConvertString(c.Text, converter.WithContext(ctx))
		if err != nil {
			return err
		}
		pc.comments = append(pc.comments, blogger.Comment{
			Author: c.Author,
			Date:   c.Date,
			Text:   text,
		})
	}

	err = pc.renderPost()
	if err != nil {
		return nil
	}

	defer slog.Info("post converted", "blog", pc.hp.Blog, "date", pc.hp.Date, "title", pc.hp.Title)
	return nil
}

func (pc *postConverter) log(w io.Writer, level, message string, node ...*html.Node) {
	n := pc.errors[level]
	pc.errors[level] = n + 1

	io.WriteString(w, "{{< debug \"Problem detected: ")
	io.WriteString(w, level)
	io.WriteString(w, "\" \"")
	io.WriteString(w, html.EscapeString(message))
	// if len(node) > 0 {
	// 	io.WriteString(w, "\n>")
	// 	html.Render(w, node[0])
	// }
	io.WriteString(w, "\" >}}\n")
}

const (
	MISSING_ORIGINAL = "MISSING_ORIGINAL"
	UNKNOWN_OBJECT   = "UNKNOWN_OBJECT"
	ERROR            = "ERROR"
	EXTERNAL_LINK    = "EXTERNAL_LINK"
	CONTENT_LOST     = "CONTENT_LOST"
)

func (pc *postConverter) renderPost() error {
	for e := range pc.errors {
		pc.hp.Tags = append(pc.hp.Tags, e)
	}

	dst, err := pc.pfs.Create("index.md")
	if err != nil {
		return fmt.Errorf("can't create Hugo post file: %w", err)
	}
	defer dst.Close()

	err = pc.Write(dst, pc.hp)
	if err != nil {
		return fmt.Errorf("can't write Hugo post file: %w", err)
	}

	fmt.Fprintf(dst, "\n\n[View the original post on Blogger](%s)\n\n", pc.url)

	htmlFile := "original.html.txt"
	html, err := pc.pfs.Create(htmlFile)
	if err != nil {
		return fmt.Errorf("can't create Hugo post HTML file: %w", err)
	}
	_, err = html.Write([]byte(pc.post.Content))
	defer html.Close()
	return nil
}

func (pc *postConverter) Write(w io.Writer, hp HugoPost) error {
	// Marshal front matter to YAML
	frontMatter, err := yaml.Marshal(hp)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("---\n"))
	if err != nil {
		return err
	}
	_, err = w.Write(frontMatter)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("---\n"))
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(hp.content))
	if err != nil {
		return err
	}

	if len(pc.comments) > 0 {
		_, err = io.WriteString(w, "\n\n>*Comments:*\n")
		if err != nil {
			return err
		}

		for _, c := range pc.comments {
			_, err = io.WriteString(w, ">\n")
			if err == nil {
				_, err = io.WriteString(w, "> "+c.Date.Format("2 January 2006")+": ")
			}
			if err == nil {
				if c.Author == "" {
					c.Author = "Anonymous"
				}
				_, err = io.WriteString(w, "*"+c.Author+"* wrote:\n")
			}
			if err == nil {
				_, err = io.WriteString(w, "> "+c.Text+"\n")
			}
		}
		if err != nil {
			return err
		}
	}

	return err
}

// Manage <img> tags
func (pc *postConverter) imageHandler(ctx converter.Context, w converter.Writer, node *html.Node) converter.RenderStatus {
	src := dom.GetAttributeOr(node, "src", "")

	_ = pc.renderImage(ctx, w, src, "")
	return converter.RenderSuccess
}

// Manage <a> tags
// When the href url is a picture, we convert it to an image, and ignore the inner HTML

func (pc *postConverter) anchorHandler(ctx converter.Context, w converter.Writer, node *html.Node) converter.RenderStatus {
	href := dom.GetAttributeOr(node, "href", "")

	u, err := url.Parse(href)
	if err != nil {
		pc.log(w, ERROR, fmt.Sprintf("can't parse href: %s", err.Error()), node)
		return converter.RenderSuccess
	}

	// Try to get the referenced image
	if u.Scheme == "http" || u.Scheme == "https" {
		baseName := path.Base(u.Path)
		ext := path.Ext(baseName)
		if ext == ".jpg" || ext == ".png" {
			_ = pc.renderImage(ctx, w, href, "")
			return converter.RenderSuccess
		}
	}

	// render the markdown link .
	w.WriteString(fmt.Sprintf("[%s](%s)", href, href))
	// // check the link in the background
	// pc.pushCheckLink(ctx, node, href)
	return converter.RenderSuccess
}

// Manage <iframe> tags
// When the iframe embeds a YouTube video, try to get the video in the takeout and render it
// Otherwise, render a warning
func (pc *postConverter) iFrameHandler(ctx converter.Context, w converter.Writer, node *html.Node) converter.RenderStatus {
	src := dom.GetAttributeOr(node, "src", "")
	// Check if the src is a YouTube embed
	if strings.Contains(src, "youtube.com/embed/") {
		return pc.renderYoutubeEmbedded(ctx, w, node)
	}

	pc.log(w, EXTERNAL_LINK, fmt.Sprintf("iframe pointing to: %s", src), node)
	w.WriteString("{{< iframe \"" + src + "\" >}}")

	// pc.pushCheckLink(ctx, node, src)
	return converter.RenderTryNext
}

// Manage <object> tags
// The object reference is an internal blogger ID, but the takeout doesn't contain the object.
// Issue an error.
func (pc *postConverter) videoObjectHandler(ctx converter.Context, w converter.Writer, node *html.Node) converter.RenderStatus {
	pc.log(w, UNKNOWN_OBJECT, "can't handle object", node)
	return converter.RenderSuccess
}

// anchorImgHandler check blogger images nested into a <a> tag
func (pc *postConverter) anchorImgHandler(ctx converter.Context, w converter.Writer, node *html.Node) converter.RenderStatus {
	img := dom.FindFirstNode(node, func(node *html.Node) bool {
		return dom.NodeName(node) == "img"
	})

	if img != nil {
		// src := dom.GetAttributeOr(img, "src", "")
		src := dom.GetAttributeOr(node, "href", "")
		if src == "http://picasa.google.com/blogger/" {
			return converter.RenderSuccess
		}
		err := pc.renderImage(ctx, w, src, "")
		if err != nil {
			return converter.RenderTryNext
		}
		return converter.RenderSuccess
	}
	return converter.RenderTryNext
}

// Manage <table> with class "tr-caption-container" that contains an image and a caption
func (pc *postConverter) tableHandler(ctx converter.Context, w converter.Writer, node *html.Node) converter.RenderStatus {
	// check if the table implements a picasa web album
	a := dom.FindFirstNode(node, func(node *html.Node) bool {
		return dom.NodeName(node) == "a"
	})

	if a != nil {

		u, err := url.Parse(dom.GetAttributeOr(a, "href", ""))
		if err != nil {
			return converter.RenderTryNext
		}
		host := u.Hostname()
		if strings.HasPrefix(host, "picasaweb.") {
			pc.log(w, CONTENT_LOST, fmt.Sprintf("The Picasa web albums service has been dismissed by Google: (%s)", u.String()))
			return converter.RenderSuccess
		}
	}

	// check if the table is of the class "tr-caption-container"
	if !dom.HasClass(node, "tr-caption-container") {
		return converter.RenderTryNext // not a table with an image and a caption
	}

	img := dom.FindFirstNode(node, func(node *html.Node) bool {
		return dom.NodeName(node) == "img"
	})
	if img != nil {
		src := dom.GetAttributeOr(img, "src", "")
		caption := ""

		captionNode := dom.FindFirstNode(node, func(node *html.Node) bool {
			return dom.NodeName(node) == "td" && dom.HasClass(node, "tr-caption")
		})
		if captionNode != nil {
			caption = html.UnescapeString(dom.CollectText(captionNode))
		}

		_ = pc.renderImage(ctx, w, src, caption)
		return converter.RenderSuccess
	}
	return converter.RenderTryNext
}
