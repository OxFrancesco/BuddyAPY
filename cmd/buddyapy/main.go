package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"buddyapy/internal/app"
	"buddyapy/internal/tui"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr, app.DefaultConfig()); err != nil {
		fmt.Fprintf(os.Stderr, "buddyapy: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, cfg app.Config) error {
	if len(args) > 0 && args[0] == "tui" {
		return tui.Run(ctx, args[1:], stdin, stdout, stderr, cfg)
	}

	return app.Run(ctx, args, stdout, stderr, cfg)
}
