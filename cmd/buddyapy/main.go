package main

import (
	"context"
	"fmt"
	"os"

	"buddyapy/internal/app"
)

func main() {
	if err := app.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, app.DefaultConfig()); err != nil {
		fmt.Fprintf(os.Stderr, "buddyapy: %v\n", err)
		os.Exit(1)
	}
}
