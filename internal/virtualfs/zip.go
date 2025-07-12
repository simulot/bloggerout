package virtualfs

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

// ZipFileSystem implements FileSystem for zip archives.
type ZipFileSystem struct {
	osFiles    []*os.File    // List of underlying os.Files
	zipReaders []*zip.Reader // List of zip.Reader for each os.File
	entries    zipEntries    // sorted list of zip entries, all zip parts merged
}

// NewZipFileSystem creates a new FileSystem from a zip or multiple zip files.
func NewZipFileSystem(zipFiles ...string) (FileSystem, error) {
	var err error
	if len(zipFiles) == 1 {
		zipFiles, err = filepath.Glob(zipFiles[0])
		if err != nil {
			return nil, err
		}
	}

	if len(zipFiles) == 0 {
		return nil, fmt.Errorf("no files found matching %s", zipFiles)
	}

	zfs := &ZipFileSystem{
		osFiles:    make([]*os.File, len(zipFiles)),
		zipReaders: make([]*zip.Reader, len(zipFiles)),
	}

	list := map[string]zipEntry{}

	for i, zipFile := range zipFiles {
		zfs.osFiles[i], err = os.Open(zipFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open zip file %s: %v", zipFile, err)
		}
		fileInfo, err := zfs.osFiles[i].Stat()
		if err != nil {
			return nil, fmt.Errorf("failed to stat zip file %s: %v", zipFile, err)
		}

		zfs.zipReaders[i], err = zip.NewReader(zfs.osFiles[i], fileInfo.Size())
		if err != nil {
			return nil, fmt.Errorf("failed to create zip reader for %s: %v", zipFile, err)
		}
		for _, file := range zfs.zipReaders[i].File {
			if _, present := list[file.Name]; present {
				return nil, fmt.Errorf("duplicate file name %s in zip files", file.Name)
			}
			list[file.Name] = zipEntry{
				// zReader: zfs.zipReaders[i],
				f: file,
			}
		}
	}

	// Merge and sort the entries
	zfs.entries = make(zipEntries, len(list))
	i := 0
	for _, entry := range list {
		zfs.entries[i] = entry
		i++
	}
	sort.Sort(zfs.entries)

	return zfs, nil
}

func (zfs *ZipFileSystem) Close() error {
	var err error
	for _, file := range zfs.osFiles {
		err = errors.Join(err, file.Close())
	}
	return err
}

func (zfs *ZipFileSystem) stat(path string) (int, zipEntry, error) {
	if len(zfs.entries) == 0 {
		return 0, zipEntry{}, fmt.Errorf("no files in zip")
	}
	if path == "" {
		return 0, zfs.entries[0], nil
	}

	i, found := slices.BinarySearchFunc(
		zfs.entries,
		zipEntry{}, // dummy entry to compare against
		func(e, t zipEntry) int {
			return strings.Compare(e.f.Name, path)
		},
	)
	// If found, return the entry
	if found {
		return i, zfs.entries[i], nil
	}

	// Not found, but if there is an entry with a path that starts with the given path, we return a directory
	if i < len(zfs.entries) && strings.HasPrefix(zfs.entries[i].f.Name, path) {
		return i, zipEntry{name: path}, nil
	}

	// Not found
	return 0, zipEntry{}, fmt.Errorf("file %s not found in zip", path)
}

func (zfs *ZipFileSystem) Stat(path string) (fs.FileInfo, error) {
	if path == "." {
		return zipEntry{name: "."}, nil
	}
	if path == "" {
		path = ""
	}
	_, fi, err := zfs.stat(path)
	if err != nil {
		return nil, err
	}
	return fi, err
}

func (zfs *ZipFileSystem) Open(path string) (fs.File, error) {
	_, fi, err := zfs.stat(path)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("Open: %s is a directory", path)
	}
	f, err := fi.f.Open()
	return &zipEntry{f: fi.f, r: f}, err
}

func (zfs *ZipFileSystem) ReadDir(path string) ([]fs.DirEntry, error) {
	if path == "." {
		path = ""
	}
	i, fi, err := zfs.stat(path)
	if err != nil {
		return nil, err
	}
	if path != "" && !fi.IsDir() {
		return nil, fmt.Errorf("ReadDir: %s is not a directory", path)
	}

	var list []fs.DirEntry
	if len(path) > 0 && !strings.HasSuffix(path, "/") {
		path = path + "/" // ensure we have a trailing slash
	}
	prevname := ""
	for _, e := range zfs.entries[i:] {
		if !strings.HasPrefix(e.f.Name, path) {
			break // not in the same directory anymore
		}
		name := e.f.Name[len(path):] // local name
		file := e
		if i := strings.IndexRune(name, '/'); i >= 0 {
			name = name[0 : i+1] // keep local directory name only
		}
		if name != prevname {
			if strings.HasSuffix(name, "/") || file.IsDir() {
				list = append(list, zipEntry{name: name})
				prevname = name
				continue
			}
			list = append(list, file)
			prevname = name
		}
	}

	return list, nil
}

// zipEntry is a wrapper around zip.File that implements fs.File.
type zipEntry struct {
	// zReader *zip.Reader
	f    *zip.File
	r    io.Reader
	name string // name if is directory
}

func (z *zipEntry) Stat() (fs.FileInfo, error) {
	return z.f.FileInfo(), nil
}

func (z *zipEntry) Read(p []byte) (int, error) {
	return z.r.Read(p)
}

func (z *zipEntry) Close() error {
	return nil
}

func (z zipEntry) Name() string {
	if z.f == nil {
		return z.name // directory name
	}
	return path.Base(z.f.Name)
}

func (z zipEntry) Size() int64 {
	if z.f == nil {
		return 0 // directory size is 0
	}
	return int64(z.f.UncompressedSize64)
}

func (z zipEntry) Mode() fs.FileMode {
	if z.f == nil {
		return fs.ModeDir
	}
	return fs.FileMode(z.f.Mode())
}

func (z zipEntry) ModTime() time.Time {
	if z.f == nil {
		return time.Time{} // directory mod time is zero
	}
	return z.f.Modified
}

func (z zipEntry) IsDir() bool {
	return z.f == nil
}

func (z zipEntry) Sys() interface{} {
	return nil
}

// implement fs.DirEntry
func (z zipEntry) Info() (fs.FileInfo, error) {
	return z, nil
}

func (z zipEntry) Type() fs.FileMode {
	return z.Mode().Type()
}

// zipEntries is a slice of zipEntry that implements sort.Interface.
type zipEntries []zipEntry

func (z zipEntries) Len() int           { return len(z) }
func (z zipEntries) Less(i, j int) bool { return z[i].f.Name < z[j].f.Name }
func (z zipEntries) Swap(i, j int)      { z[i], z[j] = z[j], z[i] }
