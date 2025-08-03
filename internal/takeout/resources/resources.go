package resources

import (
	"io/fs"
	"path"
	"sort"
	"time"

	"bloggerout/internal/virtualfs"
)

type ResourceMetadata struct {
	SizeBytes         int64     // "1641075"
	Filename          string    // "IMG_0123.jpg"
	CreationTimestamp time.Time // "1751181639509" --> to time.Time
	MimeType          string    // "image/*"
}

type Resource struct {
	vfs       virtualfs.FileSystem // virtual file system for the resource
	dirEntry  fs.DirEntry          // file information
	container string               // Resource belongs to this container (ex Album name, Blog name, YT channel)
	path      string               // path to the resource in the virtual file system, the direntry give the base name
	metadata  ResourceMetadata     // metadata of the resource
}

func (r *Resource) Open() (fs.File, error) {
	return r.vfs.Open(path.Join(r.path, r.dirEntry.Name()))
}

func (r Resource) Filename() string {
	return r.metadata.Filename
}

func (r Resource) CreationTimestamp() time.Time {
	return r.metadata.CreationTimestamp
}

func (r Resource) MimeType() string {
	return r.metadata.MimeType
}

func (r Resource) SizeBytes() int64 {
	return r.metadata.SizeBytes
}

type Resources struct {
	byPath      map[string]*Resource   // by path of the resource
	byBase      map[string][]*Resource // by base name of the resource
	byContainer map[string][]*Resource // by container name of the resource (ex Album name, Blog name, YT channel)
}

func New() *Resources {
	return &Resources{
		byPath:      make(map[string]*Resource),
		byBase:      make(map[string][]*Resource),
		byContainer: make(map[string][]*Resource),
	}
}

func (rs *Resources) Add(vfs virtualfs.FileSystem, container string, filePath string, entry fs.DirEntry, metadata *ResourceMetadata) *Resource {
	base := entry.Name()
	r := &Resource{vfs: vfs, dirEntry: entry, path: filePath, container: container}
	if metadata != nil {
		r.metadata = *metadata
	} else {
		r.metadata.Filename = base
		if info, err := entry.Info(); err == nil {
			r.metadata.CreationTimestamp = info.ModTime()
			r.metadata.SizeBytes = info.Size()
		}
	}
	l := rs.byBase[base]
	rs.byBase[base] = append(l, r)
	l = rs.byContainer[container]
	rs.byContainer[container] = append(l, r)
	rs.byPath[path.Join(filePath, base)] = r
	return r
}

func (rs *Resources) SearchByBase(base string) []*Resource {
	return rs.byBase[base]
}

func (rs *Resources) SearchInContainer(container string, base string) *Resource {
	l := rs.byContainer[container]
	for _, r := range l {
		if r.dirEntry.Name() == base {
			return r
		}
	}
	return nil
}

func (rs *Resources) SearchByBaseAndDate(base string, date time.Time) *Resource {
	if len(rs.byBase[base]) == 0 {
		return nil
	}
	l := make([]*Resource, len(rs.byBase[base]))
	copy(l, rs.byBase[base])
	sort.Slice(l, func(i, j int) bool {
		di := date.Sub(l[i].metadata.CreationTimestamp).Abs()
		dj := date.Sub(l[j].metadata.CreationTimestamp).Abs()
		return di < dj
	})
	return l[0]
}

func (rs *Resources) SearchByPath(filePath string) *Resource {
	return rs.byPath[filePath]
}
