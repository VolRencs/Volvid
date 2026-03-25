package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"YouTubeBuild/tui"

	tea "charm.land/bubbletea/v2"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	p := tea.NewProgram(tui.New(), tea.WithContext(ctx))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
