package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"go.lepovirta.org/otk/internal/gitsync"
	"go.lepovirta.org/otk/internal/osenv"
)

func main() {
	var osEnv osenv.OsEnv
	var core gitsync.Core

	osEnv.FromRealEnv()
	if err := core.Init(osEnv); err != nil {
		handleError(err)
	}

	if err := core.Run(context.Background()); err != nil {
		handleError(err)
	}
}

func handleError(err error) {
	if err == flag.ErrHelp {
		os.Exit(1)
	}
	slog.Error("fatal error", slog.Any("error", err))
	os.Exit(1)
}
