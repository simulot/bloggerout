package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type Downloader struct {
	client http.Client
}

func NewDownloader() *Downloader {
	// Safe HTTP client settings
	client := http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: false,
		},
	}
	return &Downloader{
		client: client,
	}
}

func (d *Downloader) DownloadFile(ctx context.Context, url string, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file: %s, status: %s", url, resp.Status)
	}

	if strings.Contains(resp.Header.Get("Content-Type"), "image/") {
		_, err = io.Copy(w, resp.Body)
		return err
	}

	if strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		url, err := getImgSrc(resp.Body)
		if err != nil {
			return err
		}
		return d.DownloadFile(ctx, url, w)
	}
	return fmt.Errorf("unsupported content type: %s", resp.Header.Get("Content-Type"))
}

func getImgSrc(r io.Reader) (string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", err
	}
	var url string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			for _, a := range n.Attr {
				if a.Key == "src" {
					url = a.Val
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	if url == "" {
		return "", fmt.Errorf("no img src found")
	}
	return url, nil
}

func (d *Downloader) CheckLink(ctx context.Context, url string, yeld func(url string, status int, err error)) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		yeld(url, 0, err)
		return
	}
	resp, err := d.client.Do(req)
	if err != nil {
		yeld(url, 0, err)
		return
	}
	defer resp.Body.Close()
	yeld(url, resp.StatusCode, nil)
}
