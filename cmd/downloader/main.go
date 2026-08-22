package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"YouTubeBuild/internal/app"
	"YouTubeBuild/tui"

	tea "charm.land/bubbletea/v2"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	env := app.NewEnv()
	p := tea.NewProgram(tui.New(env, ctx), tea.WithContext(ctx))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
