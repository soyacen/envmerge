package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/soyacen/envmerge/pkg/envmerge"
)

func generateCmd() *cli.Command {
	flags := withFlags(mergeFlags(),
		&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "explicit output path (single input only)"},
		&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "overwrite an existing output file"},
		&cli.BoolFlag{Name: "dry-run", Usage: "preview the result on stdout without writing"},
		&cli.BoolFlag{Name: "stdout", Usage: "write the result to stdout instead of a file"},
		&cli.BoolFlag{Name: "comments", Value: true, Usage: "preserve comments"},
		&cli.BoolFlag{Name: "blank-lines", Value: true, Usage: "preserve blank-line grouping"},
		&cli.StringFlag{Name: "dir", Usage: "directory to scan for templates"},
		&cli.StringFlag{Name: "pattern", Value: "*.env.example", Usage: "glob pattern used with --dir"},
		&cli.BoolFlag{Name: "recursive", Usage: "recurse into subdirectories when scanning --dir"},
	)
	return &cli.Command{
		Name:      "generate",
		Usage:     "Generate .env files from templates",
		ArgsUsage: "[template ...]",
		Description: "Compile one or more .env.example templates. With --dir the\n" +
			"directory is scanned using --pattern instead of positional arguments.",
		Flags: flags,
		Action: func(ctx context.Context, c *cli.Command) error {
			eng := engineFrom(c)

			inputs, err := resolveInputs(c)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return cli.Exit("", exitUsage)
			}
			output := c.String("output")
			if output != "" && len(inputs) != 1 {
				fmt.Fprintln(stderr, "Error: --output requires exactly one input template")
				return cli.Exit("", exitUsage)
			}

			force := c.Bool("force")
			for _, input := range inputs {
				res, derr := generateOne(ctx, eng, c, input, output, force)
				if derr != nil {
					return derr
				}
				if res != nil {
					reportOverrides(c, res)
				}
			}
			return nil
		},
	}
}

// generateOne performs a single generation, honoring dry-run/stdout.
func generateOne(ctx context.Context, eng *envmerge.Engine, c *cli.Command, input, output string, force bool) (*envmerge.Result, error) {
	switch {
	case c.Bool("stdout"):
		res, err := eng.Render(ctx, input)
		if err != nil {
			return nil, handleTemplateError(err)
		}
		fmt.Fprint(stdout, string(res.Content))
		return res, nil

	case c.Bool("dry-run"):
		res, err := eng.Render(ctx, input)
		if err != nil {
			return nil, handleTemplateError(err)
		}
		if !quiet(c) {
			fmt.Fprintf(stderr, "# dry-run: would write %s\n", res.OutputPath)
		}
		fmt.Fprint(stdout, string(res.Content))
		return res, nil

	default:
		res, err := eng.Generate(ctx, input, output)
		if err != nil {
			// A non-nil result with an error means the failure happened at the
			// write stage (filesystem/IO), not while resolving the template.
			if res != nil {
				printDiagnostic(err)
				return nil, cli.Exit("", exitProgram)
			}
			return nil, handleTemplateError(err)
		}
		if !quiet(c) {
			fmt.Fprintf(stdout, "Generated %s -> %s\n", base(input), res.OutputPath)
		}
		return res, nil
	}
}

// resolveInputs combines positional arguments and --dir scanning.
func resolveInputs(c *cli.Command) ([]string, error) {
	positional := c.Args().Slice()
	dir := c.String("dir")

	if len(positional) > 0 && dir != "" {
		return nil, fmt.Errorf("Error: cannot combine positional inputs with --dir")
	}
	if len(positional) > 0 {
		return positional, nil
	}
	if dir != "" {
		matches, err := scanTemplates(dir, c.String("pattern"), c.Bool("recursive"))
		if err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("Error: no templates matched in %s with pattern %q", dir, c.String("pattern"))
		}
		return matches, nil
	}
	return nil, fmt.Errorf("Error: provide template arguments or use --dir")
}

// scanTemplates lists top-level (and optionally nested) files matching pattern.
func scanTemplates(dir, pattern string, recursive bool) ([]string, error) {
	var out []string
	if !recursive {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if ok, _ := filepath.Match(pattern, e.Name()); ok {
				out = append(out, filepath.Join(dir, e.Name()))
			}
		}
		return out, nil
	}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if ok, _ := filepath.Match(pattern, d.Name()); ok {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

func base(p string) string { return filepath.Base(p) }

func titleCase(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] -= 32
	}
	return string(r)
}
