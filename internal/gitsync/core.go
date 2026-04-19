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

func (c *Core) Init(osEnv osenv.OsEnv) error {
	c.osEnv = osEnv

	var logConfig logging.Config
	logConfig.FromEnv(config.AppName, c.osEnv.EnvVars)
	logConfig.SetupGlobal(config.AppName, c.osEnv.Stderr)

	if err := c.cliFlags.Parse(
		c.osEnv.EnvVars,
		c.osEnv.Args,
		c.osEnv.Stderr,
	); err != nil {
		if err == flag.ErrHelp {
			return err
		}
		return fmt.Errorf("failed to parse CLI flags: %w", err)
	}

	if err := parseConfig(c.osEnv, &c.cliFlags, &c.cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}
	return nil
}

func (c *Core) Run(ctx context.Context) error {
	log := logging.FromContext(ctx)

	if !c.cliFlags.Run {
		log.DebugContext(ctx, "run dry-run")
		return c.dryRun()
	}

	if c.cliFlags.Once {
		log.DebugContext(ctx, "run once")
		return c.runOnce(ctx)
	}
	log.DebugContext(ctx, "run in a loop")
	return c.runLoop(ctx)
}

func (c *Core) dryRun() error {
	return dryRun(c.osEnv.Stdout, &c.cfg)
}

func (c *Core) runOnce(ctx context.Context) error {
	ctx, sigCancel := sighandle.CancelOnSignals(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer sigCancel()

	log := logging.FromContext(ctx)

	cleanUp := func(gitSync *GitSync) {
		if err := gitSync.Clean(c.osEnv.Fs); err != nil {
			log.ErrorContext(ctx, "cleanup failed", slog.Any("error", err))
		}
	}

	errs := make([]error, 0, len(c.cfg.Mappings))
	var gitSync GitSync
	for _, mapping := range c.cfg.Mappings {
		if err := gitSync.Init(ctx, &c.osEnv, c.cfg.Repositories, &mapping); err != nil {
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

func (c *Core) runLoop(ctx context.Context) error {
	ctx, sigCancel := sighandle.CancelOnSignals(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer sigCancel()

	log := logging.FromContext(ctx)

	cleanUp := func(gitSync *GitSync) {
		if err := gitSync.Clean(c.osEnv.Fs); err != nil {
			log.ErrorContext(ctx, "cleanup failed", slog.Any("error", err))
		}
	}

	gitSyncs := make([]GitSync, len(c.cfg.Mappings))
	for i, mapping := range c.cfg.Mappings {
		gitSync := &gitSyncs[i]
		if err := gitSync.Init(ctx, &c.osEnv, c.cfg.Repositories, &mapping); err != nil {
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
