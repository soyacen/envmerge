// Command envmerge is a CLI tool that compiles dotenv templates that reference
// each other with `include` into flat, deterministic .env files.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/soyacen/envmerge/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(cli.Execute(ctx, os.Args))
}
