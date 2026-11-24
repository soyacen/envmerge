package cli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

func validateCmd() *cli.Command {
	flags := withFlags(mergeFlags(),
		&cli.StringFlag{Name: "dir", Usage: "directory to scan for templates"},
		&cli.StringFlag{Name: "pattern", Value: "*.env.example", Usage: "glob pattern used with --dir"},
		&cli.BoolFlag{Name: "recursive", Usage: "recurse into subdirectories when scanning --dir"},
	)
	return &cli.Command{
		Name:      "validate",
		Usage:     "Check template syntax and dependency graph",
		ArgsUsage: "[template ...]",
		Flags:     flags,
		Action: func(ctx context.Context, c *cli.Command) error {
			eng := engineFrom(c)
			inputs, err := resolveInputs(c)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return cli.Exit("", exitUsage)
			}
			failed := false
			for _, input := range inputs {
				if err := eng.Validate(ctx, input); err != nil {
					if !quiet(c) {
						fmt.Fprintf(stderr, "✗ %s\n", input)
					}
					printDiagnostic(err)
					failed = true
					continue
				}
				if quiet(c) {
					continue
				}
				fmt.Fprintf(stdout, "%s\n", input)
				fmt.Fprintln(stdout, "✓ Template syntax valid")
				fmt.Fprintln(stdout, "✓ All included files exist")
				fmt.Fprintln(stdout, "✓ No circular dependencies")
				fmt.Fprintln(stdout, "✓ Environment variables valid")
			}
			if failed {
				return cli.Exit("", exitTemplate)
			}
			return nil
		},
	}
}
