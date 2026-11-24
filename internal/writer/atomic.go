// Package writer persists generated dotenv files atomically so a failed or
// interrupted generation never leaves a half-written or clobbered target.
package writer

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/soyacen/envmerge/internal/diagnostic"
)

// Writer persists generated content to a path.
type Writer interface {
	Write(ctx context.Context, path string, content []byte, force bool) error
}

// Atomic writes via a same-directory temp file, fsync and an atomic rename.
type Atomic struct{}

// New returns an atomic writer.
func New() *Atomic { return &Atomic{} }

// Write generates the target file atomically. When force is false an existing
// target is never overwritten (ERR_OUTPUT_EXISTS), protecting hand-edited
// secrets. The temp file and the result use 0600 permissions on POSIX systems.
func (*Atomic) Write(ctx context.Context, path string, content []byte, force bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return wrapWrite(path, err)
	}

	if !force {
		if _, err := os.Stat(path); err == nil {
			return diagnostic.New(diagnostic.ErrOutputExists, "output file already exists: "+path)
		}
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".envmerge-*.tmp")
	if err != nil {
		return wrapWrite(path, err)
	}
	tmpName := tmp.Name()
	cleanup := func() { tmp.Close(); os.Remove(tmpName) }

	if _, err := tmp.Write(content); err != nil {
		cleanup()
		return wrapWrite(path, err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return wrapWrite(path, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return wrapWrite(path, err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		os.Remove(tmpName)
		return wrapWrite(path, err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		if errors.Is(err, os.ErrPermission) {
			return diagnostic.New(diagnostic.ErrPermissionDenied, "permission denied writing "+path)
		}
		return wrapWrite(path, err)
	}
	return nil
}

func wrapWrite(path string, err error) error {
	return diagnostic.New(diagnostic.ErrWriteFailed, "failed to write "+path+": "+err.Error())
}
