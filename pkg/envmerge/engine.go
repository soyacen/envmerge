package envmerge

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/soyacen/envmerge/internal/merger"
	"github.com/soyacen/envmerge/internal/renderer"
	"github.com/soyacen/envmerge/internal/resolver"
	"github.com/soyacen/envmerge/internal/writer"
)

// Engine wires the loader, resolver, merger, renderer and writer into a single
// reusable pipeline. It is safe for concurrent use.
type Engine struct {
	opts     Options
	resolver *resolver.Engine
	merger   *merger.Engine
	renderer renderer.Renderer
	writer   writer.Writer
}

// New builds an Engine from Options, applying defaults.
func New(opts Options) *Engine {
	o := opts.apply()
	return &Engine{
		opts:     o,
		resolver: resolver.New(o.Loader, o.MaxIncludeDepth, o.RootDir),
		merger:   merger.New(),
		renderer: renderer.New(renderer.Options{
			PreserveComments:   o.PreserveComments,
			PreserveBlankLines: o.PreserveBlankLines,
		}),
		writer: writer.New(),
	}
}

// compile runs the pipeline up to rendering and returns a full Result.
func (e *Engine) compile(ctx context.Context, inputPath string) (*Result, error) {
	root, err := e.resolver.Resolve(ctx, inputPath)
	if err != nil {
		return nil, err
	}
	merged, err := e.merger.Merge(root, e.opts.ConflictPolicy)
	if err != nil {
		return nil, err
	}
	content, err := e.renderer.Render(merged)
	if err != nil {
		return nil, err
	}
	return &Result{
		InputPath:    inputPath,
		Content:      content,
		Dependencies: resolver.Dependencies(root),
		Variables:    merged.Variables,
		Overrides:    merged.Overrides,
		Merged:       merged,
		Root:         root,
	}, nil
}

// Render compiles a template and returns the content without writing a file
// (used by --dry-run / --stdout).
func (e *Engine) Render(ctx context.Context, inputPath string) (*Result, error) {
	res, err := e.compile(ctx, inputPath)
	if err != nil {
		return nil, err
	}
	res.OutputPath = e.DeriveOutputPath(inputPath)
	return res, nil
}

// Generate compiles a template and writes it to outputPath atomically. When
// outputPath is empty the name is auto-derived from the input.
func (e *Engine) Generate(ctx context.Context, inputPath, outputPath string) (*Result, error) {
	res, err := e.compile(ctx, inputPath)
	if err != nil {
		return nil, err
	}
	if outputPath == "" {
		outputPath = e.DeriveOutputPath(inputPath)
	}
	res.OutputPath = outputPath
	if err := e.writer.Write(ctx, outputPath, res.Content, e.opts.Force); err != nil {
		return res, err
	}
	return res, nil
}

// Validate compiles a template but discards the rendered output; it returns an
// error only when the template graph or contents are invalid.
func (e *Engine) Validate(ctx context.Context, inputPath string) error {
	_, err := e.compile(ctx, inputPath)
	return err
}

// Dependencies returns the reachable include paths for a template.
func (e *Engine) Dependencies(ctx context.Context, inputPath string) ([]string, error) {
	res, err := e.compile(ctx, inputPath)
	if err != nil {
		return nil, err
	}
	return res.Dependencies, nil
}

// DeriveOutputPath removes a trailing ".example" from the file name while
// preserving any leading-dot convention (design §10.2).
func (e *Engine) DeriveOutputPath(inputPath string) string {
	dir, base := filepath.Split(inputPath)
	name := strings.TrimSuffix(base, ".example")
	return filepath.Join(dir, name)
}
