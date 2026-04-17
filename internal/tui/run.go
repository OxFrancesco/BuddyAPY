package tui

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"

	"buddyapy/internal/app"
)

func Run(ctx context.Context, args []string, input io.Reader, output, stderr io.Writer, cfg app.Config) error {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printUsage(output)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}

	model := newModel(ctx, app.ResolveAPI(cfg), app.ResolveNow(cfg))

	options := []tea.ProgramOption{
		tea.WithContext(ctx),
	}
	if input != nil {
		options = append(options, tea.WithInput(input))
	}
	if output != nil {
		options = append(options, tea.WithOutput(output))
	}

	program := tea.NewProgram(model, options...)
	_, err := program.Run()
	return err
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: buddyapy tui")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Interactive yield explorer for DefiLlama pools.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Keybindings:")
	fmt.Fprintln(w, "  Tab / Shift+Tab   Cycle panes")
	fmt.Fprintln(w, "  Enter             Edit filter or jump into details")
	fmt.Fprintln(w, "  Space             Toggle focused boolean filter")
	fmt.Fprintln(w, "  /                 Jump to the symbol filter")
	fmt.Fprintln(w, "  c / C             Clear one filter / clear all filters")
	fmt.Fprintln(w, "  r                 Refresh live data and rerun search")
	fmt.Fprintln(w, "  q                 Quit")
}
