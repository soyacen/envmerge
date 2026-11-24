package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/soyacen/envmerge/internal/diagnostic"
)

func diffCmd() *cli.Command {
	flags := withFlags(mergeFlags(),
		&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "generated file to diff against"},
		&cli.BoolFlag{Name: "comments", Value: true, Usage: "preserve comments"},
		&cli.BoolFlag{Name: "blank-lines", Value: true, Usage: "preserve blank-line grouping"},
	)
	return &cli.Command{
		Name:      "diff",
		Usage:     "Show differences between a template and its generated file",
		ArgsUsage: "<template>",
		Flags:     flags,
		Action: func(ctx context.Context, c *cli.Command) error {
			input, err := singleInput(c)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return cli.Exit("", exitUsage)
			}
			eng := engineFrom(c)
			res, err2 := eng.Render(ctx, input)
			if err2 != nil {
				printDiagnostic(err2)
				return cli.Exit("", exitTemplate)
			}
			target := c.String("output")
			if target == "" {
				target = res.OutputPath
			}
			existing, rerr := os.ReadFile(target)
			if rerr != nil && !os.IsNotExist(rerr) {
				fmt.Fprintln(stderr, rerr)
				return cli.Exit("", exitProgram)
			}

			diff := unifiedDiff(string(existing), string(res.Content), target, "generated:"+input)
			if diff == "" {
				if !quiet(c) {
					fmt.Fprintf(stdout, "%s and template match; no changes.\n", target)
				}
				return nil
			}
			fmt.Fprint(stdout, diff)
			return cli.Exit(diagnostic.ErrOutputMismatch.Error(), exitCheckDiff)
		},
	}
}

// unifiedDiff returns an LCS-based unified diff of two texts, or "" when equal.
func unifiedDiff(a, b, aName, bName string) string {
	if a == b {
		return ""
	}
	al := strings.Split(strings.TrimRight(a, "\n"), "\n")
	bl := strings.Split(strings.TrimRight(b, "\n"), "\n")
	if a == "" {
		al = nil
	}
	if b == "" {
		bl = nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "--- %s\n+++ %s\n", aName, bName)

	i, j := 0, 0
	lcs := lcsTable(al, bl)
	for i < len(al) || j < len(bl) {
		if i < len(al) && j < len(bl) && al[i] == bl[j] {
			fmt.Fprintf(&sb, " %s\n", al[i])
			i++
			j++
		} else if j < len(bl) && (i >= len(al) || lcs[i][j+1] >= lcs[i+1][j]) {
			fmt.Fprintf(&sb, "+%s\n", bl[j])
			j++
		} else {
			fmt.Fprintf(&sb, "-%s\n", al[i])
			i++
		}
	}
	return sb.String()
}

// lcsTable builds the classic LCS dynamic-programming length table.
func lcsTable(a, b []string) [][]int {
	t := make([][]int, len(a)+1)
	for i := range t {
		t[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				t[i][j] = t[i+1][j+1] + 1
			} else if t[i+1][j] >= t[i][j+1] {
				t[i][j] = t[i+1][j]
			} else {
				t[i][j] = t[i][j+1]
			}
		}
	}
	return t
}
