package convert

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"bloggerout/internal/filename"

	"github.com/JohannesKaufmann/dom"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"golang.org/x/net/html"
)

func (pc *postConverter) renderYoutubeEmbedded(ctx converter.Context, w converter.Writer, node *html.Node) converter.RenderStatus {
	// Get the src attribute
	src := dom.GetAttributeOr(node, "src", "")
	if src == "" {
		return converter.RenderTryNext
	}
	// Check if the src is a YouTube embed
	if !strings.Contains(src, "youtube.com/embed/") {
		return converter.RenderTryNext
	}

	// get the ID of the video
	//  src="https://www.youtube.com/embed/VIDEOID?feature=player_embedded"

	u, err := url.Parse(src)
	if err != nil {
		pc.log(w, ERROR, fmt.Sprintf("cannot parse video URL %s", src))
		return converter.RenderTryNext
	}
	id := path.Base(u.Path)

	// search the video in takeout data
	v := pc.data.YouTube.SearchByID(ctx, id)
	if v == nil {
		pc.log(w, MISSING_ORIGINAL, fmt.Sprintf("cannot find video %s in the given takeout data", id))
		return converter.RenderTryNext

	}

	pc.renderVideo(ctx, w, video{
		Resource: v.Resource,
		YTId:     id,
		Source:   src,
		Name:     filename.Sanitize(path.Base(v.FileName)),
		Caption:  v.Title,
	})
	return converter.RenderSuccess
}
