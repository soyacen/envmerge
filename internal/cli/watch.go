package cli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/urfave/cli/v3"

	"github.com/soyacen/envmerge/pkg/envmerge"
)

func watchCmd() *cli.Command {
	flags := withFlags(mergeFlags(),
		&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "explicit output path"},
		&cli.BoolFlag{Name: "comments", Value: true, Usage: "preserve comments"},
		&cli.BoolFlag{Name: "blank-lines", Value: true, Usage: "preserve blank-line grouping"},
	)
	return &cli.Command{
		Name:      "watch",
		Usage:     "Regenerate a template whenever it or its dependencies change",
		ArgsUsage: "<template>",
		Description: "Watch the entry template and every file it includes. On any change\n" +
			"the output file is regenerated (with force) and the watch set refreshed.",
		Flags: flags,
		Action: func(ctx context.Context, c *cli.Command) error {
			input, err := singleInput(c)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return cli.Exit("", exitUsage)
			}
			return watch(ctx, c, input)
		},
	}
}

func watch(ctx context.Context, c *cli.Command, input string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintln(stderr, "Error: cannot create file watcher: "+err.Error())
		return cli.Exit("", exitProgram)
	}
	defer watcher.Close()

	// Watch with force enabled regardless of the --force flag.
	opts := optionsFrom(c)
	opts.Force = true
	eng := envmerge.New(opts)

	output := c.String("output")

	watched := map[string]bool{}
	track := func(paths []string) {
		for _, p := range paths {
			dir := filepath.Dir(p)
			if watched[dir] {
				continue
			}
			if err := watcher.Add(dir); err == nil {
				watched[dir] = true
			}
		}
	}

	regenerate := func() {
		res, err := eng.Generate(ctx, input, output)
		if err != nil {
			if res != nil {
				return // transient: file mid-write, retry on next event
			}
			printDiagnostic(err)
			return
		}
		track(append([]string{input}, res.Dependencies...))
		fmt.Fprintf(stdout, "Generated %s -> %s\n", filepath.Base(input), res.OutputPath)
	}

	// Initial generation primes the dependency set to watch.
	regenerate()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}
			// Ignore our own temporary files.
			if matched, _ := filepath.Match(".envmerge-*.tmp", filepath.Base(event.Name)); matched {
				continue
			}
			regenerate()
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintln(stderr, "watch error: "+err.Error())
		}
	}
}
