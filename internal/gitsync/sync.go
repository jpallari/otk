package gitsync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-git/go-git/v5/storage/memory"
	"go.lepovirta.org/otk/internal/gitsync/config"
	"go.lepovirta.org/otk/internal/logging"
	"go.lepovirta.org/otk/internal/matcher"
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

func (this *GitSync) sourceRepoError(reason string, cause error) *GitRepoError {
	return &GitRepoError{
		RepoId:  this.mapping.Source,
		RepoURL: this.sourceRepoConfig.URL,
		Reason:  reason,
		Cause:   cause,
	}
}

func (this *GitSync) getLogger(ctx context.Context) *slog.Logger {
	logArgs := make([]any, 0, 2)
	if this.sourceRepoConfig != nil && this.sourceRepoConfig.URL != "" {
		logArgs = append(logArgs, slog.String("sourceUrl", this.sourceRepoConfig.URL))
	}
	if this.mapping != nil && this.mapping.Source != "" {
		logArgs = append(logArgs, slog.String("sourceId", this.mapping.Source))
	}
	return logging.FromContext(ctx).With(logArgs...)
}

func (this *GitSync) Init(
	ctx context.Context,
	fs billy.Filesystem,
	repoConfigs map[string]config.Repository,
	mapping *config.SyncMapping,
) (err error) {
	err = this.init(ctx, fs, repoConfigs, mapping)
	if err != nil {
		log := this.getLogger(ctx)
		log.ErrorContext(ctx, "init failed", slog.Any("error", err))
	}
	return
}

func (this *GitSync) init(
	ctx context.Context,
	fs billy.Filesystem,
	repoConfigs map[string]config.Repository,
	mapping *config.SyncMapping,
) (err error) {
	var ok bool
	this.repoConfigs = repoConfigs
	this.mapping = mapping

	// Source
	sourceRepoConfig, ok := this.repoConfigs[this.mapping.Source]
	if !ok {
		err = fmt.Errorf("no configuration found for repo '%s'", this.mapping.Source)
		return
	}
	this.sourceRepoConfig = &sourceRepoConfig

	// getLogger depends on the above fields, so we can't call it earlier
	log := this.getLogger(ctx)

	// Source authentication
	var sourceAuth transport.AuthMethod
	sourceAuth, err = configToAuth(fs, this.sourceRepoConfig, log)
	if err != nil {
		err = this.sourceRepoError("failed to configure auth", err)
		return
	}
	this.fetchOptions = git.FetchOptions{
		RemoteURL:  this.sourceRepoConfig.URL,
		RemoteName: this.mapping.Source,
		Auth:       sourceAuth,
		Force:      true,
		RefSpecs:   []gitconf.RefSpec{gitconf.RefSpec(refSpecFetch)},
	}
	this.listOptions = git.ListOptions{
		Auth: sourceAuth,
	}

	// Repo storage
	var storer storage.Storer
	var path string
	if this.sourceRepoConfig.InMemory {
		path = ""
		storer = memory.NewStorage()
	} else {
		path = this.sourceRepoConfig.LocalPath
		if path == "" {
			log.DebugContext(ctx, "Preparing temp directory", slog.String("url", this.sourceRepoConfig.URL))
			path, err = fsutil.TempDir(fs, "", fmt.Sprintf("%s-%s", config.AppName, this.mapping.Source))
			if err != nil {
				err = this.sourceRepoError("failed to create temporary directory", err)
				return
			}
			this.tempDirPath = path // stored for clean-up later
		}

		var pathFs billy.Filesystem
		pathFs, err = fs.Chroot(path)
		if err != nil {
			err = this.sourceRepoError(fmt.Sprintf("failed to chroot path '%s'", path), err)
			return
		}
		storer = filesystem.NewStorage(pathFs, cache.NewObjectLRUDefault())
	}

	// Initialize repo
	log.DebugContext(ctx, "initializing repo", slog.String("gitPath", path))
	this.repo, err = git.Init(storer, nil)
	if err != nil && err != git.ErrRepositoryAlreadyExists {
		err = this.sourceRepoError("failed to initialize repo", err)
		return
	}
	if err == git.ErrRepositoryAlreadyExists {
		log.DebugContext(ctx, "opening repo", slog.String("path", path))
		this.repo, err = git.Open(storer, nil)
		if err != nil {
			err = this.sourceRepoError(fmt.Sprintf("failed to open path %s", path), err)
			return
		}
	}

	// Prepare source remote
	if this.sourceRepoConfig.URL == "" {
		log.InfoContext(ctx, "no remote specified, fetch will be skipped", slog.String("source", this.mapping.Source))
	} else {
		err = prepareRemote(ctx, this.repo, this.mapping.Source, this.sourceRepoConfig, log)
		if err != nil {
			err = this.sourceRepoError("failed to prepare remote", err)
			return
		}
	}

	// Configure targets
	this.pushOptions = make(map[string]git.PushOptions, len(mapping.Targets))
	for _, targetId := range mapping.Targets {
		targetRepoConfig, ok := this.repoConfigs[targetId]
		if !ok {
			err = fmt.Errorf("no configuration found for repo '%s'", targetId)
			return
		}
		log := log.With(
			slog.String("targetId", targetId),
			slog.String("targetUrl", targetRepoConfig.URL),
		)

		var authMethod transport.AuthMethod
		authMethod, err = configToAuth(fs, &targetRepoConfig, log)
		if err != nil {
			err = &GitRepoError{
				RepoId:  targetId,
				RepoURL: targetRepoConfig.URL,
				Reason:  "failed to configure auth",
				Cause:   err,
			}
			return
		}
		this.pushOptions[targetId] = git.PushOptions{
			RemoteName: targetId,
			RemoteURL:  targetRepoConfig.URL,
			Auth:       authMethod,
			Force:      true,
			Atomic:     false,
		}
		err = prepareRemote(ctx, this.repo, targetId, &targetRepoConfig, log)
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

func (this *GitSync) Clean(fs billy.Filesystem) error {
	if this.tempDirPath != "" {
		err := fsutil.RemoveAll(fs, this.tempDirPath)
		if err != nil {
			return fmt.Errorf(
				"failed to clean up temp directory '%s': %w",
				this.tempDirPath, err,
			)
		}
	}
	return nil
}

func (this *GitSync) RunInLoop(ctx context.Context) error {
	log := this.getLogger(ctx)
	ctx = logging.AddToContext(ctx, log)
	timer := time.NewTimer(0)

	for {
		select {
		case <-timer.C:
			if err := this.RunOnce(ctx); err != nil {
				log.ErrorContext(ctx, "sync failed", slog.Any("error", err))
			}
			interval := this.mapping.Interval.Duration
			if interval <= 0 {
				interval = time.Hour
			}
			log.DebugContext(ctx, "next sync", slog.Duration("interval", interval))
			timer.Reset(this.mapping.Interval.Duration)
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil
		}
	}
}

func (this *GitSync) RunOnce(ctx context.Context) error {
	var branches, tags []string
	var err error
	log := this.getLogger(ctx)
	ctx = logging.AddToContext(ctx, log)

	if this.fetchOptions.RemoteURL == "" {
		// Local branches and tags
		branches, tags, err = this.getLocalBranchesAndTags()
		if err != nil {
			return this.sourceRepoError("failed to query local", err)
		}
	} else {
		// Remote branches and tags
		log.DebugContext(ctx, "get source remote")
		sourceRemote, err := this.repo.Remote(this.mapping.Source)
		if err != nil {
			return this.sourceRepoError("failed to query remote", err)
		}

		log.DebugContext(ctx, "fetch latest commits for source remote")
		err = sourceRemote.FetchContext(ctx, &this.fetchOptions)
		if err != nil && err != git.NoErrAlreadyUpToDate {
			return this.sourceRepoError("failed to fetch from remote", err)
		}

		log.DebugContext(ctx, "get branches and tags for source remote")
		branches, tags, err = this.getRemoteBranchesAndTags(ctx, sourceRemote)
		if err != nil {
			return this.sourceRepoError("failed to fetch branches and tags", err)
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
		len(this.pushOptions)*len(branches)+len(this.pushOptions)*len(tags),
	)
	for targetId, targetOptions := range this.pushOptions {
		targetRepoConfig := this.repoConfigs[targetId]
		log := log.With(
			slog.String("targetId", targetId),
			slog.String("targetUrl", targetRepoConfig.URL),
		)
		targetOptions.RefSpecs = refSpecs

		log.DebugContext(ctx, "push to remote target")
		err = this.repo.PushContext(ctx, &targetOptions)
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

func (this *GitSync) getLocalBranchesAndTags() (
	branches []string,
	tags []string,
	err error,
) {
	branches = make([]string, 0, 1000)
	tags = make([]string, 0, 1000)
	var refIter storer.ReferenceIter

	refIter, err = this.repo.Branches()
	if err != nil {
		err = fmt.Errorf("local branch iterator error: %w", err)
		return
	}

	refIter.ForEach(func(ref *plumbing.Reference) error {
		branch := ref.Name().Short()
		if matchAny(this.mapping.Branches, branch) {
			branches = append(branches, branch)
		}
		return nil
	})

	refIter, err = this.repo.Tags()
	if err != nil {
		err = fmt.Errorf("local tags iterator error: %w", err)
		return
	}

	refIter.ForEach(func(ref *plumbing.Reference) error {
		tag := ref.Name().Short()
		if matchAny(this.mapping.Tags, tag) {
			tags = append(tags, tag)
		}
		return nil
	})

	return
}

func (this *GitSync) getRemoteBranchesAndTags(
	ctx context.Context,
	remote *git.Remote,
) (branches []string, tags []string, err error) {
	var refs []*plumbing.Reference
	log := logging.FromContext(ctx)

	log.DebugContext(ctx, "listing refs")
	refs, err = remote.ListContext(ctx, &this.listOptions)
	if err == transport.ErrEmptyRemoteRepository {
		log.DebugContext(ctx, "remote is empty")
		return nil, nil, nil
	}
	if err != nil {
		err = fmt.Errorf("failed to list refs for remote '%s': %w", this.mapping.Source, err)
		return
	}

	branches = make([]string, 0, 1000)
	tags = make([]string, 0, 1000)

	for _, ref := range refs {
		refName := ref.Name().String()
		if strings.HasPrefix(refName, refPrefixBranch) {
			branch := strings.TrimPrefix(refName, refPrefixBranch)
			if matchAny(this.mapping.Branches, branch) {
				branches = append(branches, branch)
			}
		} else if strings.HasPrefix(refName, refPrefixTag) {
			tag := strings.TrimPrefix(refName, refPrefixTag)
			if matchAny(this.mapping.Tags, tag) {
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

func (this *GitRepoError) Error() string {
	if this.Cause == nil {
		return fmt.Sprintf(
			"%s in '%s' (url: %s)",
			this.Reason,
			this.RepoId,
			this.RepoURL,
		)
	}
	return fmt.Sprintf(
		"%s in git repo '%s' (url: %s): %s",
		this.Reason,
		this.RepoId,
		this.RepoURL,
		this.Cause.Error(),
	)
}
