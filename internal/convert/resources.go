package convert

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path"
	"strings"

	"bloggerout/internal/takeout/resources"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
)

// resource represents an image or a video to be rendered
type resource struct {
	Mime     string              // mime type
	Resource *resources.Resource // resource link in the takeout
	Source   string              // source in takeout or the web URL
	Name     string              // base's name
	Caption  string              // caption
	data     []byte              // data when encoded in the url
}

// renderImage renders an image with a caption
// render inline image if the link starts with "data:image/png;base64,..."
// check if the image is in the referenced takeouts, copy the image on the post
// otherwise download get the remote image and add a note
func (pc *postConverter) renderImage(ctx context.Context, w converter.Writer, link string, caption string) error {
	var img *resource
	var err error
	if strings.HasPrefix(link, "data:") {
		img, err = decodeInlineImage(ctx, link)
		if err != nil {
			pc.log(w, ERROR, fmt.Sprintf("can't decode inline image: %s", err))
			return nil
		}
	} else {
		u, err := url.Parse(link)
		if err != nil {
			pc.log(w, ERROR, fmt.Sprintf("can't parse image link: %.200s", link))
		}
		imageName := path.Base(u.Path)
		imageName = strings.ReplaceAll(imageName, "+", " ")
		r := pc.data.Resources.SearchByBaseAndDate(imageName, pc.hp.Date)
		img = &resource{
			Source:   link,
			Name:     imageName,
			Resource: r,
		}
	}

	// don't render duplicated images in the same post
	if _, exists := pc.resources[img.Name]; exists {
		return nil
	}
	img.Caption = caption
	pc.resources[img.Name] = img // remember we have processed this image already

	_, err = pc.pfs.Stat(img.Name)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		pc.log(w, ERROR, fmt.Sprintf("can't stat image: %s", err))
		return err
	}

	if err != nil && errors.Is(err, os.ErrNotExist) {
		// the image is not yet in the post folder

		if img.Resource == nil {
			// Not in the takeout, try to download from the internet
			if img.data != nil {
				err := pc.copyFromURL(ctx, img)
				if err != nil {
					slog.Error("can't copy image from url", "image", img.Name, "error", err)
					return err
				}
			} else {
				err = pc.ImageDownload(ctx, img)
				if err != nil {
					// Render the error in the post, nothing else
					pc.log(w, CONTENT_LOST, "can't download image: "+err.Error())
					return nil
				}
				// The image has been downloaded from the internet, mention it in the post.
				pc.log(w, MISSING_ORIGINAL, "Image not found in the takeout archive, remote image copied to the post: "+img.Name)
			}
		} else {
			// The image is found in the takeout
			err := pc.copyFromTakeout(ctx, img)
			if err != nil {
				slog.Error("can't copy image from takeout", "image", img.Name, "error", err)
				return err
			}
		}
	}
	// write the figure shortcode
	sb := strings.Builder{}
	sb.WriteString("{{< figure")
	sb.WriteString(" src=")
	sb.WriteString(safeAttribute(img.Name))
	sb.WriteString(" alt=")
	sb.WriteString(fmt.Sprintf("%q", img.Name))
	if img.Caption != "" {
		sb.WriteString(" caption=")
		sb.WriteString(safeAttribute(img.Caption))
	}
	sb.WriteString(" >}}\n")
	w.WriteString(sb.String())

	return nil
}

// decodeInlineImage decodes an inline image and returns the image with the image
// name set with the SHA1 of the content.
//
// decode the image type: "data:image/png;base64,iVBORw0KGgoAAAANSUh...."
func decodeInlineImage(_ context.Context, link string) (*resource, error) {
	source, found := strings.CutPrefix(link, "data:image/")
	if !found {
		return nil, fmt.Errorf("invalid data URL: %.200s", link)
	}

	var ext string
	ext, source, found = strings.Cut(source, ";")
	if !found {
		return nil, fmt.Errorf("invalid data URL: %.200s", link)
	}

	var encoding string
	encoding, source, found = strings.Cut(source, ",")
	if !found || encoding != "base64" {
		return nil, fmt.Errorf("invalid data URL: %.200s", link)
	}

	img := &resource{}
	hasher := sha1.New()
	hasher.Write([]byte(source))
	hash := hasher.Sum(nil)
	img.Name = fmt.Sprintf("%x.%s", hash, ext)
	img.Source = img.Name

	// decode the image into img.data
	var err error
	dec := base64.NewDecoder(base64.StdEncoding, strings.NewReader(source))
	img.data, err = io.ReadAll(dec)
	if err != nil {
		return nil, fmt.Errorf("invalid data URL: %.200s", link)
	}
	return img, nil
}

// // submit the check link task
// func (pc *postConverter) pushCheckLink(ctx context.Context, link string) error {
// 	u, err := url.Parse(link)
// 	if err != nil {
// 		return err
// 	}

// 	pc.submit(ctx, func(ctx context.Context) {
// 		switch u.Scheme {
// 		case "http", "https":
// 			pc.downloader.CheckLink(ctx, link, func(url string, status int, err error) {
// 				if u.Host != "blogger.googleusercontent.com" {
// 					pc.log(w, EXTERNAL_LINK, fmt.Sprintf("External link,  HTTP Status:%d,  Link: %.200s", status, link), tag)
// 					return
// 				}
// 				pc.log("WARNING", fmt.Sprintf("Internal link,  HTTP Status:%d,  Link: %.200s", status, link), tag)
// 			})
// 		default:
// 			pc.log(w, ERROR, fmt.Sprintf("Unknown scheme:%s, Link: %.200s", u.Scheme, link), tag)
// 		}
// 	})
// 	return nil
// }

// submit an image download

func (pc *postConverter) ImageDownload(ctx context.Context, resource *resource) error {
	file, err := pc.pfs.Create(resource.Name)
	if err != nil {
		slog.Error("Failed to create file", "file", resource.Name, "error", err)
		return err
	}
	defer file.Close()
	err = pc.downloader.DownloadFile(ctx, resource.Source, file)
	if err != nil {
		slog.Error("Failed to download file", "file", resource.Name, "error", err)
		defer pc.pfs.Remove(resource.Name) // Remove the file if the download fails, to avoid leaving an empty file in the post's directory
		return err
	}
	return nil
}

func (pc *postConverter) copyFromInternet(ctx context.Context, img *resource) error {
	dest, err := pc.pfs.Create(img.Name)
	if err != nil {
		return fmt.Errorf("can't create file %s: %w", img.Name, err)
	}
	defer dest.Close()

	err = pc.downloader.DownloadFile(ctx, img.Source, dest)
	if err != nil {
		return fmt.Errorf("can't download image %s: %w", img.Name, err)
	}
	return nil
}

func (pc *postConverter) copyFromTakeout(_ context.Context, img *resource) error {
	dest, err := pc.pfs.Create(img.Name)
	if err != nil {
		return fmt.Errorf("failed to create image %s: %w", img.Name, err)
	}
	defer dest.Close()

	src, err := img.Resource.Open()
	if err != nil {
		return fmt.Errorf("can't open image %q in the takeout: %w", img.Name, err)
	}
	defer src.Close()
	_, err = io.Copy(dest, src)
	if err != nil {
		return fmt.Errorf("failed to copy image into %q: %w", img.Name, err)
	}
	return nil
}

func (pc *postConverter) copyFromURL(_ context.Context, img *resource) error {
	// Create the destination directory if it doesn't exist
	dest, err := pc.pfs.Create(img.Name)
	if err != nil {
		return fmt.Errorf("failed to create image %s: %w", img.Name, err)
	}
	defer dest.Close()

	_, err = io.Copy(dest, bytes.NewReader(img.data))
	if err != nil {
		return fmt.Errorf("failed to copy image %s: %w", img.Name, err)
	}
	return nil
}

// video represents an video to be rendered
type video struct {
	Resource *resources.Resource // image resource
	YTId     string              // youtube video ID
	Source   string              // image source in takeout or the web URL
	Name     string              // image base's name
	Caption  string              // image caption
}

// shortcode for a HTML5 video player
// https://eddmann.com/posts/building-a-video-hugo-shortcode-for-local-and-remote-content/

func (pc *postConverter) renderVideo(ctx context.Context, w converter.Writer, video video) error {
	sb := strings.Builder{}
	if video.Resource == nil {
		_, err := w.WriteString("{{< media/ytembed " + video.YTId + " >}}")
		return err
	}

	err := pc.compyVideoFromTakeout(ctx, video)
	if err != nil {
		slog.Error("failed to copy video from takeout", "error", err)
		return err
	}

	sb.WriteString("{{< media/video")
	sb.WriteString(" src=")
	sb.WriteString(safeAttribute(strings.TrimPrefix(video.Name, "static")))
	sb.WriteString(" type=")
	sb.WriteString(safeAttribute("video/" + strings.TrimPrefix(path.Ext(video.Name), ".")))

	if video.Caption != "" {
		sb.WriteString(" caption=")
		sb.WriteString(safeAttribute(video.Caption))
	}

	sb.WriteString(" >}}\n")
	w.WriteString(sb.String())

	return nil
}

func (pc *postConverter) compyVideoFromTakeout(_ context.Context, video video) error {
	// Create the destination directory if it doesn't exist
	dest, err := pc.pfs.Create(video.Name)
	if err != nil {
		return fmt.Errorf("failed to create video %s: %w", video.Name, err)
	}
	defer dest.Close()

	src, err := video.Resource.Open()
	if err != nil {
		return fmt.Errorf("can't open video %q in the takeout: %w", video.Name, err)
	}
	defer src.Close()
	_, err = io.Copy(dest, src)
	if err != nil {
		defer pc.pfs.Remove(video.Name)
		return fmt.Errorf("failed to copy image into %q: %w", video.Name, err)
	}
	return nil
}

// safeAttribute escapes a string for use in an HTML attribute
func safeAttribute(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
