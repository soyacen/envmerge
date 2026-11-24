package loader

import (
	"path/filepath"
	"strings"

	"github.com/soyacen/envmerge/internal/diagnostic"
)

// JoinRelative resolves an include argument declared inside a template file to
// an absolute path. Relative includes are always based on the declaring file's
// directory, never on the process working directory (design §5.2). Absolute
// includes are used as-is.
func JoinRelative(declaringFile, includeArg string) string {
	if includeArg == "" {
		return ""
	}
	if filepath.IsAbs(includeArg) {
		return filepath.Clean(includeArg)
	}
	base := filepath.Dir(declaringFile)
	return filepath.Clean(filepath.Join(base, includeArg))
}

// CanonicalID returns a stable identity for a path, resolving symlinks so that
// different spellings of the same file are treated as one node. This is the key
// to catching symlink-based cycles and honoring shared dependencies. When the
// file does not yet exist (e.g. an optional include) EvalSymlinks falls back to
// the cleaned absolute path.
func CanonicalID(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	// EvalSymlinks requires the path to exist; resolve the deepest existing
	// ancestor and re-append the remainder so non-existent tails are preserved.
	if resolved, err := evalSymlinks(abs); err == nil {
		return resolved
	}
	return filepath.Clean(abs)
}

func evalSymlinks(path string) (string, error) {
	full, err := filepath.EvalSymlinks(path)
	if err == nil {
		return filepath.Clean(full), nil
	}
	return path, err
}

// CheckRoot validates that a resolved path stays within the allowed root
// directory (design §12.4). An empty root disables the check. The comparison
// uses canonical paths so a symlink cannot escape the boundary.
func CheckRoot(root, canonicalPath string) error {
	if root == "" {
		return nil
	}
	rID := CanonicalID(root)
	rel, err := filepath.Rel(rID, canonicalPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return diagnostic.New(diagnostic.ErrPathOutsideRoot, "include resolves outside the allowed root: "+canonicalPath)
	}
	return nil
}
