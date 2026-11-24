// Package loader reads template files from an abstracted file system and
// resolves paths to a canonical identity used for cycle and sharing detection.
package loader

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/soyacen/envmerge/internal/diagnostic"
)

// Loader loads raw template bytes for a path.
type Loader interface {
	Load(ctx context.Context, path string) ([]byte, error)
}

// FSLoader reads from any io/fs.FS (an os.DirFS, a fstest.MapFS, etc.). It is
// the default loader for tests and sandboxed tooling.
type FSLoader struct {
	FS   fs.FS
	Root string // absolute directory the FS is mounted at, for identity paths
}

// NewFSLoader returns an FSLoader backed by os.DirFS rooted at an absolute dir.
func NewFSLoader(dir string) (*FSLoader, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	return &FSLoader{FS: os.DirFS(abs), Root: abs}, nil
}

// Load reads the file at path (absolute or Root-relative) from the FS.
func (l *FSLoader) Load(ctx context.Context, path string) ([]byte, error) {
	rel := l.toRelative(path)
	if !fs.ValidPath(filepath.ToSlash(rel)) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, wrapNotFound(path, err)
		}
		return data, nil
	}
	data, err := fs.ReadFile(l.FS, filepath.ToSlash(rel))
	if err != nil {
		return nil, wrapNotFound(path, err)
	}
	return data, nil
}

func (l *FSLoader) toRelative(path string) string {
	if filepath.IsAbs(path) {
		r, err := filepath.Rel(l.Root, path)
		if err == nil {
			return r
		}
		return path
	}
	return path
}

func wrapNotFound(path string, err error) error {
	if os.IsNotExist(err) || err == fs.ErrNotExist {
		return diagnostic.New(diagnostic.ErrFileNotFound, "included template does not exist: "+path)
	}
	if os.IsPermission(err) {
		return diagnostic.New(diagnostic.ErrPermissionDenied, "permission denied: "+path)
	}
	return err
}

// OSLoader reads directly from the real file system. It is the default loader
// for the CLI, where templates live anywhere on disk and absolute paths matter.
type OSLoader struct{}

// Load reads the file at path from disk.
func (OSLoader) Load(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, wrapNotFound(path, err)
	}
	return data, nil
}
