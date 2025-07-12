package virtualfs

import (
	"io/fs"
	"os"
	"path/filepath"
)

// FileSystem abstracts file access from either the OS file system or zip archives.
type FileSystem interface {
	fs.FS
	fs.StatFS
	fs.ReadDirFS
}

// OSFileSystem implements FileSystem for the OS file system.
type OSFileSystem struct {
	path string
}

func NewOSFileSystem(dir string) (FileSystem, error) {
	_, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	return &OSFileSystem{
		path: dir,
	}, nil
}

func (fsys OSFileSystem) Open(path string) (fs.File, error) {
	p := filepath.Join(fsys.path, path)
	return os.Open(p)
}

func (fsys OSFileSystem) Stat(path string) (fs.FileInfo, error) {
	p := filepath.Join(fsys.path, path)
	return os.Stat(p)
}

func (fsys OSFileSystem) ReadDir(path string) ([]fs.DirEntry, error) {
	p := filepath.Join(fsys.path, path)
	return os.ReadDir(p)
}
