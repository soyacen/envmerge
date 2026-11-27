package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/soyacen/envmerge/internal/diagnostic"
)

func checkCmd() *cli.Command {
	flags := withFlags(mergeFlags(),
		&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "explicit generated file to compare against"},
		&cli.StringFlag{Name: "dir", Usage: "directory to scan for templates"},
		&cli.StringFlag{Name: "pattern", Value: "*.env.example", Usage: "glob pattern used with --dir"},
		&cli.BoolFlag{Name: "recursive", Usage: "recurse into subdirectories when scanning --dir"},
	)
	return &cli.Command{
		Name:      "check",
		Usage:     "Verify generated files match their templates (CI guard)",
		ArgsUsage: "[template ...]",
		Description: "Recompute each template in memory and compare with the existing\n" +
			"output file. Exit code 4 signals a mismatch or a missing output file.",
		Flags: flags,
		Action: func(ctx context.Context, c *cli.Command) error {
			eng := engineFrom(c)
			inputs, err := resolveInputs(c)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return cli.Exit("", exitUsage)
			}

			mismatch := false
			for _, input := range inputs {
				res, cerr := eng.Render(ctx, input)
				if cerr != nil {
					printDiagnostic(cerr)
					return cli.Exit("", exitTemplate)
				}
				target := c.String("output")
				if target == "" {
					target = res.OutputPath
				}
				existing, rerr := os.ReadFile(target)
				if rerr != nil {
					if !quiet(c) {
						fmt.Fprintf(stderr, "✗ %s: output file missing: %s\n", input, target)
					}
					mismatch = true
					continue
				}
				if !bytes.Equal(existing, res.Content) {
					if !quiet(c) {
						fmt.Fprintf(stderr, "✗ %s: %s is out of date\n", input, target)
						fmt.Fprintf(stderr, "  run `envmerge generate --force %s` to refresh\n", input)
					}
					mismatch = true
				} else if !quiet(c) {
					fmt.Fprintf(stdout, "✓ %s: %s is up to date\n", input, target)
				}
			}
			if mismatch {
				return cli.Exit(diagnostic.ErrOutputMismatch.Error(), exitCheckDiff)
			}
			return nil
		},
	}
}
