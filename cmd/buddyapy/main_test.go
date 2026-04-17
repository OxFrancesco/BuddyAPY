package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"buddyapy/internal/app"
)

func TestRunTUIHelp(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run(context.Background(), []string{"tui", "--help"}, strings.NewReader(""), &stdout, &stderr, app.DefaultConfig())
	if err != nil {
		t.Fatalf("run(tui --help) error = %v", err)
	}

	output := stdout.String()
	for _, fragment := range []string{"Usage: buddyapy tui", "Tab", "r", "/"} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("run(tui --help) output missing %q:\n%s", fragment, output)
		}
	}
}
