package cli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/soyacen/envmerge/internal/resolver"
)

func treeCmd() *cli.Command {
	return &cli.Command{
		Name:      "tree",
		Usage:     "Show the template dependency tree",
		ArgsUsage: "<template>",
		Flags:     mergeFlags(),
		Action: func(ctx context.Context, c *cli.Command) error {
			input, err := singleInput(c)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return cli.Exit("", exitUsage)
			}
			eng := engineFrom(c)
			res, rerr := eng.Render(ctx, input)
			if rerr != nil {
				printDiagnostic(rerr)
				return cli.Exit("", exitTemplate)
			}

			baseDir := filepath.Dir(res.Root.AbsPath)
			renderTree(stdout, res.Root, baseDir)
			fmt.Fprintf(stdout, "\nTemplates: %d\nVariables: %d\nOverrides: %d\n",
				1+len(res.Dependencies), len(res.Variables), len(res.Overrides))
			return nil
		},
	}
}

func renderTree(w io.Writer, root *resolver.ResolvedDocument, baseDir string) {
	fmt.Fprintln(w, displayPath(root.AbsPath, baseDir))
	renderChildren(w, root, "", baseDir, map[string]bool{root.ID: true})
}

func renderChildren(w io.Writer, node *resolver.ResolvedDocument, prefix, baseDir string, expanded map[string]bool) {
	for i, child := range node.Includes {
		last := i == len(node.Includes)-1
		connector := "├── "
		continuePrefix := "│   "
		if last {
			connector = "└── "
			continuePrefix = "    "
		}
		fmt.Fprintln(w, prefix+connector+displayPath(child.AbsPath, baseDir))
		if expanded[child.ID] {
			continue // shared dependency: show the reference, do not re-descend
		}
		expanded[child.ID] = true
		renderChildren(w, child, prefix+continuePrefix, baseDir, expanded)
	}
}

func displayPath(abs, baseDir string) string {
	if rel, err := filepath.Rel(baseDir, abs); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return abs
}
