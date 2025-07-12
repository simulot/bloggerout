package convert

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"bloggerout/internal/filename"
	"bloggerout/internal/takeout"

	"github.com/JohannesKaufmann/dom"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"golang.org/x/net/html"
	"gopkg.in/yaml.v3"
)

type postConverter struct {
	c   *Convert
	mdc *converter.Converter
	hp  *HugoPost
}

type HugoPost struct {
	Blog    string
	Title   string
	Date    string
	Draft   bool
	Tags    []string
	content string
	Author  string
}

func (c *Convert) convertPost(_ context.Context, blog string, p takeout.Post) error {
	hp := HugoPost{
		Blog:  blog,
		Title: fmt.Sprintf("%q", p.Title),
		Date:  p.Date.Format("2006-01-02"),
		Draft: p.Draft,
		Tags:  p.Categories,
	}
	if hp.Title == "" {
		hp.Title = hp.Date + " - Untitled"
	}
	mdc := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
		),
	)
	pc := &postConverter{
		c:   c,
		mdc: mdc,
		hp:  &hp,
	}

	mdc.Register.RendererFor("table", converter.TagTypeInline, pc.renderTable, converter.PriorityEarly)
	mdc.Register.RendererFor("a", converter.TagTypeInline, pc.renderAnchor, converter.PriorityEarly+1)

	content, err := mdc.ConvertString(p.Content)
	if err != nil {
		return err
	}

	hp.content = content

	err = pc.renderPost()

	return err
}

// renderAnchor revisits the default <a>
// - detect blogger's images and render them as markdown images pointing to local files
// - keep normal links as is

func (p *postConverter) renderAnchor(ctx converter.Context, w converter.Writer, node *html.Node) converter.RenderStatus {
	// Blogger images are nested into a <a> tag with with an url starting with

	img := dom.FindFirstNode(node, func(node *html.Node) bool {
		return dom.NodeName(node) == "img"
	})

	if img != nil {
		src := dom.GetAttributeOr(img, "src", "")
		image := image{
			Source: src,
			Blog:   p.hp.Blog,
			Name:   path.Base(src),
		}
		err := p.renderImage(ctx, w, image)
		if err != nil {
			fmt.Printf("Can't render image: %s\n", err.Error())
			return converter.RenderTryNext
		}
		return converter.RenderSuccess
	}
	return converter.RenderTryNext
}

// renderTable renders an image placed in table with a caption

func (p *postConverter) renderTable(ctx converter.Context, w converter.Writer, node *html.Node) converter.RenderStatus {
	// check if the table is of the class "tr-caption-container"
	if !dom.HasClass(node, "tr-caption-container") {
		return converter.RenderTryNext
	}

	img := dom.FindFirstNode(node, func(node *html.Node) bool {
		return dom.NodeName(node) == "img"
	})
	if img != nil {
		image := image{
			Blog: p.hp.Blog,
		}

		caption := dom.FindFirstNode(node, func(node *html.Node) bool {
			return dom.NodeName(node) == "td" && dom.HasClass(node, "tr-caption")
		})
		if caption != nil {
			image.Caption = dom.CollectText(caption)
		}

		src := dom.GetAttributeOr(img, "src", "")
		image.Source = src
		image.Name = path.Base(src)
		err := p.renderImage(ctx, w, image)
		if err != nil {
			fmt.Printf("Can't render image: %s\n", err.Error())
			return converter.RenderTryNext
		}
		return converter.RenderSuccess
	}
	return converter.RenderTryNext
}

// var renderImg = template.Must(template.New("image").Parse(`{{define "img"}}<img src="{{.Source}}" alt="{{.Name}}">{{end}}
// {{if .Caption}}<figure>{{template "img" . }}<figcaption>{{.Caption}}</figcaption></figure>{{else}}{{template "img" .}}{{end}}
// `))

var renderImg = template.Must(template.New("image").Parse(`![{{.Name}}]({{.StaticName}})
`))

func (p *postConverter) renderImage(ctx context.Context, w converter.Writer, img image) error {
	base := path.Base(img.Source)
	base = strings.ReplaceAll(base, "+", " ") // TODO check why this doesn't work
	postContext := struct {
		Blog   string // blog name
		Title  string // post title
		Date   string // post date
		Author string
	}{
		Blog:   filename.Sanitize(p.hp.Blog),
		Title:  filename.Sanitize(p.hp.Title),
		Author: filename.Sanitize(p.hp.Author),
		Date:   p.hp.Date,
	}
	sb := strings.Builder{}
	err := p.c.imagePathTmpl.Execute(&sb, postContext)
	if err != nil {
		return err
	}

	img.StaticName = path.Join(sb.String()+"/", base)
	img.Name = base

	err = p.c.pushImage(ctx, img)
	if err != nil {
		return err
	}

	// for the rendering, remove /static
	sb.Reset()
	img.StaticName = strings.TrimPrefix(img.StaticName, "/static")
	err = renderImg.Execute(&sb, img)
	if err != nil {
		return fmt.Errorf("can't render image: %w", err)
	}
	w.WriteString("\n{{% center %}}\n")
	w.WriteString(sb.String())
	if img.Caption != "" {
		w.WriteString("*")
		w.WriteString(img.Caption)
		w.WriteString("*")
	}
	w.WriteString("\n{{% /center %}}\n")
	return nil
}

// image represents an image to be rendered
type image struct {
	Blog       string
	Source     string // image source in takeout or the web URL
	StaticName string // filename in the static directory
	Name       string // image base's name
	Caption    string // image caption
}

func (p *postConverter) renderPost() error {
	var sb strings.Builder
	sb.Reset()
	// Hugo image path is defined by the imagePathTmpl template
	// It is rendered with the postContext struct as the data

	postContext := struct {
		Blog   string // blog name
		Title  string // post title
		Date   string // post date
		Author string
	}{
		Blog:   filename.Sanitize(p.hp.Blog),
		Title:  filename.Sanitize(p.hp.Title),
		Author: filename.Sanitize(p.hp.Author),
		Date:   p.hp.Date,
	}

	err := p.c.hugoPathTmpl.Execute(&sb, postContext)
	if err != nil {
		return fmt.Errorf("can't render blog path: %w", err)
	}
	dstName := sb.String()

	sb.Reset()
	err = p.c.postPathTmpl.Execute(&sb, postContext)
	if err != nil {
		return fmt.Errorf("can't render image path: %w", err)
	}

	dstName = path.Join(dstName, sb.String(), p.hp.Date+"-"+postContext.Title+".md")

	dir := filepath.Dir(dstName)
	err = os.MkdirAll(dir, 0o755)
	if err != nil {
		return fmt.Errorf("can't create Hugo post directory: %w", err)
	}

	f, err := os.Create(dstName)
	if err != nil {
		return err
	}
	defer f.Close()

	err = p.hp.Write(f)
	return err
}

// const hugoTmpl = `---
// title: "{{ .Title }}"
// date: "{{ .Date }}"
// draft: {{ .Draft }}
// {{- if .Tags }}
// tags: [{{ range $i, $tag := .Tags }}{{if $i}}, {{end}}"{{$tag}}"{{end}}]
// {{- end }}
// ---
// {{.Content}}
// `

// var hugo = template.Must(template.New("hugo").Parse(hugoTmpl))

func (hp HugoPost) Write(w io.Writer) error {
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
	return err
}
