package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"time"
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

func (d *Downloader) DownloadFile(ctx context.Context, url string, dest string) error {
	dir := path.Dir(dest)
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		return err
	}

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

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
