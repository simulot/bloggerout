//go:build e2e
// +build e2e

package e2e

import (
	"io/fs"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"bloggerout/internal/virtualfs"
)

func TestZipFSonRealFiles(t *testing.T) {
	const zipPaths = "../../../../private/takeout-20250629T153322Z-*.zip"

	allFiles, err := scanZipFiles(zipPaths)
	if err != nil {
		t.Errorf("failed to scan zip files: %v", err)
		t.FailNow()
	}
	t.Logf("found %d files in zip files", len(allFiles))

	zfs, err := virtualfs.NewZipFileSystem(zipPaths)
	if err != nil {
		t.Errorf("failed to create zip file system: %v", err)
		t.FailNow()
	}

	foundFiles := []string{}
	err = fs.WalkDir(zfs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			foundFiles = append(foundFiles, path)
		}
		return nil
	})
	if err != nil {
		t.Errorf("failed to walk zip file system: %v", err)
		t.FailNow()
	}

	if !reflect.DeepEqual(allFiles, foundFiles) {
		t.Errorf("files in zip file system do not match files in zip files")
		t.FailNow()
	}
}

func scanZipFiles(path string) ([]string, error) {
	files, err := filepath.Glob(path)
	if err != nil {
		return nil, err
	}
	allFiles := []string{}
	for _, file := range files {
		c := exec.Command("unzip", "-Z1", file)
		output, err := c.CombinedOutput()
		if err != nil {
			return nil, err
		}
		entries := string(output)
		for _, l := range strings.Split(entries, "\n") {
			if l != "" {
				allFiles = append(allFiles, l)
			}
		}
	}
	sort.Strings(allFiles)
	return allFiles, nil
}
