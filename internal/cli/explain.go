package cli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

func explainCmd() *cli.Command {
	return &cli.Command{
		Name:      "explain",
		Usage:     "Explain where a variable's final value comes from",
		ArgsUsage: "<template> <KEY>",
		Description: "Show the winning declaration and any overrides that produced a\n" +
			"variable's final value. Never prints the value itself for sensitive keys.",
		Flags: mergeFlags(),
		Action: func(ctx context.Context, c *cli.Command) error {
			args := c.Args().Slice()
			if len(args) != 2 {
				fmt.Fprintln(stderr, "Error: explain requires a template and a variable name")
				return cli.Exit("", exitUsage)
			}
			input, key := args[0], args[1]

			eng := engineFrom(c)
			res, err := eng.Render(ctx, input)
			if err != nil {
				printDiagnostic(err)
				return cli.Exit("", exitTemplate)
			}

			for _, v := range res.Variables {
				if v.Key != key {
					continue
				}
				fmt.Fprintf(stdout, "%s\n", key)
				fmt.Fprintf(stdout, "  resolved value: %s\n", valueFor(v.Key, v.Value))
				fmt.Fprintf(stdout, "  won from:       %s\n", v.WinnerPos.String())
				fmt.Fprintf(stdout, "  first seen:     %s\n", v.FirstPos.String())

				var chain []string
				for _, o := range res.Overrides {
					if o.Key == key {
						chain = append(chain, fmt.Sprintf("%s -> %s", o.Previous.String(), o.Current.String()))
					}
				}
				if len(chain) > 0 {
					fmt.Fprintln(stdout, "  override chain:")
					for _, seg := range chain {
						fmt.Fprintf(stdout, "    %s\n", seg)
					}
				} else {
					fmt.Fprintln(stdout, "  no overrides")
				}
				return nil
			}

			fmt.Fprintf(stderr, "Error: variable %q is not present in %s\n", key, input)
			return cli.Exit("", exitTemplate)
		},
	}
}

// valueFor hides values for keys that look sensitive.
func valueFor(key, value string) string {
	if isSensitive(key) {
		return "<redacted>"
	}
	return value
}
