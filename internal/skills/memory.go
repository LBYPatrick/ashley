package skills

import (
	"bytes"
	"io/fs"
	"path"
	"time"
)

// WithDefinition overlays one unsaved definition for preview and validation.
func WithDefinition(base fs.FS, name string, data []byte) fs.FS {
	return Overlay{User: definitionFS{name: "skills/" + name + ".jsonc", data: data}, Base: base}
}

type definitionFS struct {
	name string
	data []byte
}

func (f definitionFS) Open(name string) (fs.File, error) {
	if name != f.name {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return &memoryFile{Reader: bytes.NewReader(f.data), info: memoryInfo{name: path.Base(name), size: int64(len(f.data))}}, nil
}

type memoryFile struct {
	*bytes.Reader
	info memoryInfo
}

func (f *memoryFile) Close() error               { return nil }
func (f *memoryFile) Stat() (fs.FileInfo, error) { return f.info, nil }

type memoryInfo struct {
	name string
	size int64
}

func (i memoryInfo) Name() string       { return i.name }
func (i memoryInfo) Size() int64        { return i.size }
func (i memoryInfo) Mode() fs.FileMode  { return 0444 }
func (i memoryInfo) ModTime() time.Time { return time.Time{} }
func (i memoryInfo) IsDir() bool        { return false }
func (i memoryInfo) Sys() any           { return nil }
