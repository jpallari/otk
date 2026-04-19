package gitsync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
	fsutil "github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	gitconf "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-git/go-git/v5/storage/memory"
	"go.lepovirta.org/otk/internal/gitsync/config"
	"go.lepovirta.org/otk/internal/logging"
	"go.lepovirta.org/otk/internal/matcher"
	"go.lepovirta.org/otk/internal/osenv"
)

const (
	refPrefixBranch = "refs/heads/"
	refPrefixTag    = "refs/tags/"
	refSpecFetch    = "refs/heads/*:refs/heads/*"
)

type GitSync struct {
	repoConfigs      map[string]config.Repository
	mapping          *config.SyncMapping
	repo             *git.Repository
	fetchOptions     git.FetchOptions
	listOptions      git.ListOptions
	pushOptions      map[string]git.PushOptions
	sourceRepoConfig *config.Repository
	tempDirPath      string
}

func (gs *GitSync) sourceRepoError(reason string, cause error) *GitRepoError {
	return &GitRepoError{
		RepoId:  gs.mapping.Source,
		RepoURL: gs.sourceRepoConfig.URL,
		Reason:  reason,
		Cause:   cause,
	}
}

func (gs *GitSync) getLogger(ctx context.Context) *slog.Logger {
	logArgs := make([]any, 0, 2)
	if gs.sourceRepoConfig != nil && gs.sourceRepoConfig.URL != "" {
		logArgs = append(logArgs, slog.String("sourceUrl", gs.sourceRepoConfig.URL))
	}
	if gs.mapping != nil && gs.mapping.Source != "" {
		logArgs = append(logArgs, slog.String("sourceId", gs.mapping.Source))
	}
	return logging.FromContext(ctx).With(logArgs...)
}

func (gs *GitSync) Init(
	ctx context.Context,
	osEnv *osenv.OsEnv,
	repoConfigs map[string]config.Repository,
	mapping *config.SyncMapping,
) (err error) {
	err = gs.init(ctx, osEnv, repoConfigs, mapping)
	if err != nil {
		log := gs.getLogger(ctx)
		log.ErrorContext(ctx, "init failed", slog.Any("error", err))
	}
	return
}

func (gs *GitSync) init(
	ctx context.Context,
	osEnv *osenv.OsEnv,
	repoConfigs map[string]config.Repository,
	mapping *config.SyncMapping,
) (err error) {
	var ok bool
	gs.repoConfigs = repoConfigs
	gs.mapping = mapping

	// Use custom HTTP client
	httpClient := http.Client{
		Transport: osEnv.HttpTransport,
		Timeout:   2 * time.Minute,
	}
	client.InstallProtocol("https", githttp.NewClient(&httpClient))

	// Source
	sourceRepoConfig, ok := gs.repoConfigs[gs.mapping.Source]
	if !ok {
		err = fmt.Errorf("no configuration found for repo '%s'", gs.mapping.Source)
		return
	}
	gs.sourceRepoConfig = &sourceRepoConfig

	// getLogger depends on the above fields, so we can't call it earlier
	log := gs.getLogger(ctx)

	// Source authentication
	var sourceAuth transport.AuthMethod
	sourceAuth, err = configToAuth(osEnv.Fs, gs.sourceRepoConfig, log)
	if err != nil {
		err = gs.sourceRepoError("failed to configure auth", err)
		return
	}
	gs.fetchOptions = git.FetchOptions{
		RemoteURL:  gs.sourceRepoConfig.URL,
		RemoteName: gs.mapping.Source,
		Auth:       sourceAuth,
		Force:      true,
		RefSpecs:   []gitconf.RefSpec{gitconf.RefSpec(refSpecFetch)},
	}
	gs.listOptions = git.ListOptions{
		Auth: sourceAuth,
	}

	// Repo storage
	var storer storage.Storer
	var path string
	if gs.sourceRepoConfig.InMemory {
		path = ""
		storer = memory.NewStorage()
	} else {
		path = gs.sourceRepoConfig.LocalPath
		if path == "" {
			log.DebugContext(ctx, "Preparing temp directory", slog.String("url", gs.sourceRepoConfig.URL))
			path, err = fsutil.TempDir(osEnv.Fs, "", fmt.Sprintf("%s-%s", config.AppName, gs.mapping.Source))
			if err != nil {
				err = gs.sourceRepoError("failed to create temporary directory", err)
				return
			}
			gs.tempDirPath = path // stored for clean-up later
		}

		var pathFs billy.Filesystem
		pathFs, err = osEnv.Fs.Chroot(path)
		if err != nil {
			err = gs.sourceRepoError(fmt.Sprintf("failed to chroot path '%s'", path), err)
			return
		}
		storer = filesystem.NewStorage(pathFs, cache.NewObjectLRUDefault())
	}

	// Initialize repo
	log.DebugContext(ctx, "initializing repo", slog.String("gitPath", path))
	gs.repo, err = git.Init(storer, nil)
	if err != nil && err != git.ErrRepositoryAlreadyExists {
		err = gs.sourceRepoError("failed to initialize repo", err)
		return
	}
	if err == git.ErrRepositoryAlreadyExists {
		log.DebugContext(ctx, "opening repo", slog.String("path", path))
		gs.repo, err = git.Open(storer, nil)
		if err != nil {
			err = gs.sourceRepoError(fmt.Sprintf("failed to open path %s", path), err)
			return
		}
	}

	// Prepare source remote
	if gs.sourceRepoConfig.URL == "" {
		log.InfoContext(ctx, "no remote specified, fetch will be skipped", slog.String("source", gs.mapping.Source))
	} else {
		err = prepareRemote(ctx, gs.repo, gs.mapping.Source, gs.sourceRepoConfig, log)
		if err != nil {
			err = gs.sourceRepoError("failed to prepare remote", err)
			return
		}
	}

	// Configure targets
	gs.pushOptions = make(map[string]git.PushOptions, len(mapping.Targets))
	for _, targetId := range mapping.Targets {
		targetRepoConfig, ok := gs.repoConfigs[targetId]
		if !ok {
			err = fmt.Errorf("no configuration found for repo '%s'", targetId)
			return
		}
		log := log.With(
			slog.String("targetId", targetId),
			slog.String("targetUrl", targetRepoConfig.URL),
		)

		var authMethod transport.AuthMethod
		authMethod, err = configToAuth(osEnv.Fs, &targetRepoConfig, log)
		if err != nil {
			err = &GitRepoError{
				RepoId:  targetId,
				RepoURL: targetRepoConfig.URL,
				Reason:  "failed to configure auth",
				Cause:   err,
			}
			return
		}
		gs.pushOptions[targetId] = git.PushOptions{
			RemoteName: targetId,
			RemoteURL:  targetRepoConfig.URL,
			Auth:       authMethod,
			Force:      true,
			Atomic:     false,
		}
		err = prepareRemote(ctx, gs.repo, targetId, &targetRepoConfig, log)
		if err != nil {
			err = &GitRepoError{
				RepoId:  targetId,
				RepoURL: targetRepoConfig.URL,
				Reason:  "failed to set up remote",
				Cause:   err,
			}
			return
		}
	}

	return nil
}

func (gs *GitSync) Clean(fs billy.Filesystem) error {
	if gs.tempDirPath != "" {
		err := fsutil.RemoveAll(fs, gs.tempDirPath)
		if err != nil {
			return fmt.Errorf(
				"failed to clean up temp directory '%s': %w",
				gs.tempDirPath, err,
			)
		}
	}
	return nil
}

func (gs *GitSync) RunInLoop(ctx context.Context) error {
	log := gs.getLogger(ctx)
	ctx = logging.AddToContext(ctx, log)
	timer := time.NewTimer(0)

	for {
		select {
		case <-timer.C:
			if err := gs.RunOnce(ctx); err != nil {
				log.ErrorContext(ctx, "sync failed", slog.Any("error", err))
			}
			interval := gs.mapping.Interval.Duration
			if interval <= 0 {
				interval = time.Hour
			}
			log.DebugContext(ctx, "next sync", slog.Duration("interval", interval))
			timer.Reset(gs.mapping.Interval.Duration)
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil
		}
	}
}

func (gs *GitSync) RunOnce(ctx context.Context) error {
	var branches, tags []string
	var err error
	log := gs.getLogger(ctx)
	ctx = logging.AddToContext(ctx, log)

	if gs.fetchOptions.RemoteURL == "" {
		// Local branches and tags
		branches, tags, err = gs.getLocalBranchesAndTags()
		if err != nil {
			return gs.sourceRepoError("failed to query local", err)
		}
	} else {
		// Remote branches and tags
		log.DebugContext(ctx, "get source remote")
		sourceRemote, err := gs.repo.Remote(gs.mapping.Source)
		if err != nil {
			return gs.sourceRepoError("failed to query remote", err)
		}

		log.DebugContext(ctx, "fetch latest commits for source remote")
		err = sourceRemote.FetchContext(ctx, &gs.fetchOptions)
		if err != nil && err != git.NoErrAlreadyUpToDate {
			return gs.sourceRepoError("failed to fetch from remote", err)
		}

		log.DebugContext(ctx, "get branches and tags for source remote")
		branches, tags, err = gs.getRemoteBranchesAndTags(ctx, sourceRemote)
		if err != nil {
			return gs.sourceRepoError("failed to fetch branches and tags", err)
		}
		if len(branches) == 0 && len(tags) == 0 {
			return nil
		}
	}

	refSpecs := make([]gitconf.RefSpec, 0, len(branches)+len(tags))
	for _, branch := range branches {
		refSpecs = append(refSpecs, refSpecForBranchUpdate(branch))
	}
	for _, tag := range tags {
		refSpecs = append(refSpecs, refSpecForTagUpdate(tag))
	}

	errs := make(
		[]error,
		0,
		len(gs.pushOptions)*len(branches)+len(gs.pushOptions)*len(tags),
	)
	for targetId, targetOptions := range gs.pushOptions {
		targetRepoConfig := gs.repoConfigs[targetId]
		log := log.With(
			slog.String("targetId", targetId),
			slog.String("targetUrl", targetRepoConfig.URL),
		)
		targetOptions.RefSpecs = refSpecs

		log.DebugContext(ctx, "push to remote target")
		err = gs.repo.PushContext(ctx, &targetOptions)
		if err != nil && err != git.NoErrAlreadyUpToDate {
			log.ErrorContext(ctx, "failed to push to remote", slog.Any("error", err))
			err = &GitRepoError{
				RepoId:  targetId,
				RepoURL: targetRepoConfig.URL,
				Reason:  "failed to push to remote",
				Cause:   err,
			}
			errs = append(errs, err)
		} else if err == git.NoErrAlreadyUpToDate {
			log.DebugContext(ctx, "remote already up-to-date")
		} else {
			log.InfoContext(ctx, "remote update succeeded")
		}
	}

	return errors.Join(errs...)
}

func (gs *GitSync) getLocalBranchesAndTags() (
	branches []string,
	tags []string,
	err error,
) {
	branches = make([]string, 0, 1000)
	tags = make([]string, 0, 1000)
	var refIter storer.ReferenceIter

	refIter, err = gs.repo.Branches()
	if err != nil {
		err = fmt.Errorf("local branch iterator error: %w", err)
		return
	}

	_ = refIter.ForEach(func(ref *plumbing.Reference) error {
		branch := ref.Name().Short()
		if matchAny(gs.mapping.Branches, branch) {
			branches = append(branches, branch)
		}
		return nil
	})

	refIter, err = gs.repo.Tags()
	if err != nil {
		err = fmt.Errorf("local tags iterator error: %w", err)
		return
	}

	_ = refIter.ForEach(func(ref *plumbing.Reference) error {
		tag := ref.Name().Short()
		if matchAny(gs.mapping.Tags, tag) {
			tags = append(tags, tag)
		}
		return nil
	})

	return
}

func (gs *GitSync) getRemoteBranchesAndTags(
	ctx context.Context,
	remote *git.Remote,
) (branches []string, tags []string, err error) {
	var refs []*plumbing.Reference
	log := logging.FromContext(ctx)

	log.DebugContext(ctx, "listing refs")
	refs, err = remote.ListContext(ctx, &gs.listOptions)
	if err == transport.ErrEmptyRemoteRepository {
		log.DebugContext(ctx, "remote is empty")
		return nil, nil, nil
	}
	if err != nil {
		err = fmt.Errorf("failed to list refs for remote '%s': %w", gs.mapping.Source, err)
		return
	}

	branches = make([]string, 0, 1000)
	tags = make([]string, 0, 1000)

	for _, ref := range refs {
		refName := ref.Name().String()
		if after, ok := strings.CutPrefix(refName, refPrefixBranch); ok {
			branch := after
			if matchAny(gs.mapping.Branches, branch) {
				branches = append(branches, branch)
			}
		} else if after, ok := strings.CutPrefix(refName, refPrefixTag); ok {
			tag := after
			if matchAny(gs.mapping.Tags, tag) {
				tags = append(tags, tag)
			}
		}
	}

	log.DebugContext(
		ctx,
		"found branches and tags",
		slog.Int("branchCount", len(branches)),
		slog.Int("tagCount", len(tags)),
		slog.String("branches", strings.Join(branches, ", ")),
		slog.String("tags", strings.Join(tags, ", ")),
	)
	return
}

func matchAny(matchers []matcher.M, s string) bool {
	for _, m := range matchers {
		if m.MatchString(s) {
			return true
		}
	}
	return false
}

func prepareRemote(
	ctx context.Context,
	repo *git.Repository,
	remoteId string,
	repoConfig *config.Repository,
	log *slog.Logger,
) error {
	log = log.With(slog.String("remoteId", remoteId))
	log.DebugContext(ctx, "querying remote")
	remote, err := repo.Remote(remoteId)

	if err != nil && err != git.ErrRemoteNotFound {
		return fmt.Errorf("failed to get remote '%s': %w", remoteId, err)
	}
	if err == nil {
		if slices.Contains(remote.Config().URLs, repoConfig.URL) {
			log.DebugContext(ctx, "remote configured already")
			return nil
		}
		log.DebugContext(ctx, "deleting remote before reconfiguring")
		if err := repo.DeleteRemote(remoteId); err != nil {
			return fmt.Errorf("failed to delete remote '%s': %w", remoteId, err)
		}
	}

	log.DebugContext(ctx, "creating remote")
	_, err = repo.CreateRemote(&gitconf.RemoteConfig{
		Name:   remoteId,
		URLs:   []string{repoConfig.URL},
		Mirror: true,
	})
	if err != nil {
		return fmt.Errorf("failed to create remote '%s': %w", remoteId, err)
	}

	return nil
}

func refSpecForBranchUpdate(branch string) gitconf.RefSpec {
	return gitconf.RefSpec(fmt.Sprintf(
		"+%s%s:%s%s",
		refPrefixBranch,
		branch,
		refPrefixBranch,
		branch,
	))
}

func refSpecForTagUpdate(tag string) gitconf.RefSpec {
	return gitconf.RefSpec(fmt.Sprintf(
		"+%s%s:%s%s",
		refPrefixTag,
		tag,
		refPrefixTag,
		tag,
	))
}

type GitRepoError struct {
	RepoId  string
	RepoURL string
	Reason  string
	Cause   error
}

func (e *GitRepoError) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf(
			"%s in '%s' (url: %s)",
			e.Reason,
			e.RepoId,
			e.RepoURL,
		)
	}
	return fmt.Sprintf(
		"%s in git repo '%s' (url: %s): %s",
		e.Reason,
		e.RepoId,
		e.RepoURL,
		e.Cause.Error(),
	)
}
