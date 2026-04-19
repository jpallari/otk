package gitsync

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"syscall"

	"go.lepovirta.org/otk/internal/gitsync/config"
	"go.lepovirta.org/otk/internal/logging"
	"go.lepovirta.org/otk/internal/osenv"
	"go.lepovirta.org/otk/internal/sighandle"
	"golang.org/x/sync/errgroup"
)

type Core struct {
	osEnv    osenv.OsEnv
	cliFlags config.CliFlags
	cfg      config.Config
}

func (this *Core) Init(osEnv osenv.OsEnv) error {
	this.osEnv = osEnv

	var logConfig logging.Config
	logConfig.FromEnv(config.AppName, this.osEnv.EnvVars)
	logConfig.SetupGlobal(config.AppName, this.osEnv.Stderr)

	if err := this.cliFlags.Parse(
		this.osEnv.EnvVars,
		this.osEnv.Args,
		this.osEnv.Stderr,
	); err != nil {
		if err == flag.ErrHelp {
			return err
		}
		return fmt.Errorf("failed to parse CLI flags: %w", err)
	}

	if err := parseConfig(this.osEnv, &this.cliFlags, &this.cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}
	return nil
}

func (this *Core) Run(ctx context.Context) error {
	log := logging.FromContext(ctx)

	if !this.cliFlags.Run {
		log.DebugContext(ctx, "run dry-run")
		return this.dryRun()
	}

	if this.cliFlags.Once {
		log.DebugContext(ctx, "run once")
		return this.runOnce(ctx)
	}
	log.DebugContext(ctx, "run in a loop")
	return this.runLoop(ctx)
}

func (this *Core) dryRun() error {
	return dryRun(this.osEnv.Stdout, &this.cfg)
}

func (this *Core) runOnce(ctx context.Context) error {
	ctx, sigCancel := sighandle.CancelOnSignals(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer sigCancel()

	log := logging.FromContext(ctx)

	cleanUp := func(gitSync *GitSync) {
		if err := gitSync.Clean(this.osEnv.Fs); err != nil {
			log.ErrorContext(ctx, "cleanup failed", slog.Any("error", err))
		}
	}

	errs := make([]error, 0, len(this.cfg.Mappings))
	var gitSync GitSync
	for _, mapping := range this.cfg.Mappings {
		if err := gitSync.Init(ctx, &this.osEnv, this.cfg.Repositories, &mapping); err != nil {
			cleanUp(&gitSync)
			errs = append(errs, err)
			continue
		}
		if err := gitSync.RunOnce(ctx); err != nil {
			errs = append(errs, err)
		}
		cleanUp(&gitSync)
	}
	return errors.Join(errs...)
}

func (this *Core) runLoop(ctx context.Context) error {
	ctx, sigCancel := sighandle.CancelOnSignals(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer sigCancel()

	log := logging.FromContext(ctx)

	cleanUp := func(gitSync *GitSync) {
		if err := gitSync.Clean(this.osEnv.Fs); err != nil {
			log.ErrorContext(ctx, "cleanup failed", slog.Any("error", err))
		}
	}

	gitSyncs := make([]GitSync, len(this.cfg.Mappings))
	for i, mapping := range this.cfg.Mappings {
		gitSync := &gitSyncs[i]
		if err := gitSync.Init(ctx, &this.osEnv, this.cfg.Repositories, &mapping); err != nil {
			cleanUp(gitSync)
			return err
		}
		defer cleanUp(gitSync)
	}

	eg, ctx := errgroup.WithContext(ctx)
	for i := range gitSyncs {
		gitSync := gitSyncs[i]
		eg.Go(func() error {
			return gitSync.RunInLoop(ctx)
		})
	}

	return eg.Wait()
}
