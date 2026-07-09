package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sine-io/propulse/internal/infrastructure/config"
	applogger "github.com/sine-io/propulse/internal/infrastructure/logger"
	"github.com/sine-io/propulse/internal/platform/app"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(os.Stdout, app.Usage)
		return 0
	}

	mode, err := app.NormalizeMode(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, app.Usage)
		return 1
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	cfg.Mode = mode

	log := applogger.New(cfg.Log)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, mode, cfg, log); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	return 0
}
