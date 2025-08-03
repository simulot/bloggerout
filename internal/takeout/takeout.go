package takeout

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"bloggerout/internal/takeout/blogger"
	"bloggerout/internal/takeout/resources"
	"bloggerout/internal/takeout/youtube"
	"bloggerout/internal/virtualfs"

	"github.com/simulot/TakeoutLocalization/go/localization"
)

// Takeout represents the main structure for handling data exported from various platforms.
// It includes localization data, blogger-specific data, YouTube-specific data, and a virtual file system.
type Takeout struct {
	vfs          virtualfs.FileSystem
	Localization localization.Products   // Localization data for the takeout
	Resources    *resources.Resources    // Photos and Videos contained in the takeout
	Blogger      *blogger.BloggerTakeout // The blogger data contained in the takeout
	YouTube      *youtube.YouTubeTakeout // The YouTube data contained in the takeout
}

// ReadTakeout read the Blogger Takeout file or folder.
// depending on the CLI invokation, a directory or a glob of zip files can be provided.
// the function will determine the type of input and process it accordingly.
func ReadTakeout(ctx context.Context, inputPaths []string) (*Takeout, error) {
	if len(inputPaths) == 0 {
		return nil, fmt.Errorf("no input paths provided")
	}

	zips := []string{}

	// got something like the result of a glob, check if all are zip files
	allZip := true
	for _, path := range inputPaths {
		if strings.ToLower(filepath.Ext(path)) != ".zip" {
			allZip = false
			break
		}

		paths, err := filepath.Glob(path)
		if err != nil {
			return nil, err
		}
		zips = append(zips, paths...)

	}

	var err error
	to := Takeout{
		Localization: localization.GetDefaultLocalizations(),
		Resources:    resources.New(),
	}
	if allZip {
		to.vfs, err = virtualfs.NewZipFileSystem(zips...)
		if err != nil {
			return nil, err
		}
		return to.processDirectory(ctx)
	} else {
		if len(inputPaths) != 1 {
			return nil, fmt.Errorf("only one input path is supported")
		}
		s, err := os.Stat(inputPaths[0])
		if err != nil {
			return nil, err
		}
		if s.IsDir() {
			// Process directory
			to.vfs, err = virtualfs.NewOSFileSystem(inputPaths[0])
			if err != nil {
				return nil, err
			}
			return to.processDirectory(ctx)
		}
	}
	return nil, fmt.Errorf("unsupported file type: %s", inputPaths[0])
}

// processDirectory processes the directory to find the takeout parts
func (to *Takeout) processDirectory(ctx context.Context) (*Takeout, error) {
	err := fs.WalkDir(to.vfs, ".", func(filePath string, d fs.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err != nil {
				return err
			}
			if d.IsDir() {
				if filePath == "." {
					return nil
				}
				base := path.Base(filePath)

				select {
				case <-ctx.Done():
					return ctx.Err()
				default:

					// get the key of the localized path
					key, _ := to.Localization.Globalize(base)

					switch key {
					case "Blogger":
						to.Blogger, err = blogger.New(to.vfs, to.Resources).Scan(ctx, filePath)
					case "YouTube and YouTube Music":
						to.YouTube, err = youtube.New(to.vfs, to.Resources, &to.Localization).Scan(ctx, filePath, key)
					default:
						return nil
					}
					if err == nil {
						return fs.SkipDir
					}
				}
			}
		}
		// ignoring anything else
		return nil
	})
	return to, err
}
