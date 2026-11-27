// Package cli implements the envmerge command line interface on top of the
// pkg/envmerge SDK. Exit codes follow the design specification (section 13):
// 0 success, 1 program/filesystem, 2 CLI usage, 3 template/parse, 4 check mismatch.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/soyacen/envmerge/internal/merger"
	"github.com/soyacen/envmerge/pkg/envmerge"
)

// Version is injected at build time via -ldflags.
var Version = "dev"

var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

// Exit codes.
const (
	exitOK        = 0
	exitProgram   = 1
	exitUsage     = 2
	exitTemplate  = 3
	exitCheckDiff = 4
)

// Execute builds the command tree, runs it and returns a process exit code.
func Execute(ctx context.Context, args []string) int {
	app := newApp()
	// Suppress urfave's built-in exit/printing so that we own exit-code and
	// diagnostic formatting entirely; Run still returns the error.
	app.ExitErrHandler = func(context.Context, *cli.Command, error) {}

	err := app.Run(ctx, args)
	if err == nil {
		return exitOK
	}
	var ec cli.ExitCoder
	if errors.As(err, &ec) {
		return ec.ExitCode()
	}
	// Any error not surfaced as an ExitCoder originates from argument parsing.
	return exitUsage
}

func newApp() *cli.Command {
	app := &cli.Command{
		Name:      "envmerge",
		Usage:     "Compile dotenv templates with include-based composition",
		UsageText: "envmerge [global options] command [arguments]",
		Version:   Version,
		Description: "envmerge merges .env.example templates that reference each\n" +
			"other with `include`, producing flattened, deterministic .env files.",
		Commands: []*cli.Command{
			generateCmd(),
			validateCmd(),
			checkCmd(),
			treeCmd(),
			explainCmd(),
			diffCmd(),
			watchCmd(),
			versionCmd(),
		},
	}
	return app
}

// versionCmd prints the semantic version.
func versionCmd() *cli.Command {
	return &cli.Command{
		Name:   "version",
		Usage:  "Print the envmerge version",
		Action: func(context.Context, *cli.Command) error { fmt.Fprintln(stdout, "envmerge "+Version); return nil },
	}
}

// --- shared flag sets ---------------------------------------------------

// mergeFlags are options that influence resolution/merging and apply to every
// read/compile command.
func mergeFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "conflict", Value: "local-wins", Usage: "conflict policy: local-wins|last-wins|first-wins|error"},
		&cli.IntFlag{Name: "max-depth", Value: envmerge.DefaultMaxIncludeDepth, Usage: "maximum include recursion depth"},
		&cli.StringFlag{Name: "root", Usage: "restrict include resolution to this directory"},
		&cli.BoolFlag{Name: "verbose", Aliases: []string{"v"}, Usage: "print override diagnostics"},
		&cli.BoolFlag{Name: "quiet", Aliases: []string{"q"}, Usage: "suppress informational output"},
	}
}

func withFlags(base []cli.Flag, extra ...cli.Flag) []cli.Flag {
	out := make([]cli.Flag, 0, len(base)+len(extra))
	out = append(out, base...)
	return append(out, extra...)
}

// --- engine + option wiring --------------------------------------------

func engineFrom(c *cli.Command) *envmerge.Engine {
	return envmerge.New(optionsFrom(c))
}

// optionsFrom builds SDK options from the shared flag set.
func optionsFrom(c *cli.Command) envmerge.Options {
	policy, ok := merger.ParseConflictPolicy(c.String("conflict"))
	if !ok {
		policy = merger.ConflictLocalWins
	}
	return envmerge.Options{
		ConflictPolicy:  policy,
		MaxIncludeDepth: int(c.Int("max-depth")),
		RootDir:         c.String("root"),
		Force:           c.Bool("force"),
	}
}

func verbose(c *cli.Command) bool { return c.Bool("verbose") }
func quiet(c *cli.Command) bool   { return c.Bool("quiet") }

// --- error formatting ---------------------------------------------------

// handleTemplateError prints a structured diagnostic and returns an ExitCoder
// with exit code 3. Non-diagnostic errors are reported as program errors (1).
func handleTemplateError(err error) error {
	if err == nil {
		return nil
	}
	if code := diagnosticCode(err); code != "" {
		printDiagnostic(err)
		return cli.Exit("", exitTemplate)
	}
	fmt.Fprintln(stderr, "Error: "+err.Error())
	return cli.Exit("", exitProgram)
}
