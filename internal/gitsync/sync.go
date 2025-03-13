package internal

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5"
	gitconf "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/jpallari/otk/internal/git-sync/config"
	"github.com/rs/zerolog"
)

const (
	refBranchPrefix = "refs/heads/"
	refTagPrefix    = "refs/tags/"
	refSpecFetch    = "refs/heads/*:refs/heads/*"
)

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
		"%s in git repo '%s' (url: %s): %w",
		this.Reason,
		this.RepoId,
		this.RepoURL,
		this.Cause,
	)
}

type GitSync struct {
	repoConfigs      map[string]config.Repository
	link             *config.Link
	repo             *git.Repository
	fetchOptions     git.FetchOptions
	pushOptions      map[string]git.PushOptions
	sourceId         string
	sourceRepoConfig *config.Repository
	branchMatcher    matcher
	tagMatcher       matcher
}

func (this *GitSync) sourceRepoError(reason string, cause error) *GitRepoError {
	return &GitRepoError{
		RepoId:  this.sourceId,
		RepoURL: this.sourceRepoConfig.URL,
		Reason:  reason,
		Cause:   cause,
	}
}

func (this *GitSync) Init(
	ctx context.Context,
	repoConfigs map[string]config.Repository,
	link *config.Link,
) (err error) {
	var ok bool
	this.repoConfigs = repoConfigs
	this.link = link

	// Source
	this.sourceId = this.link.Source
	*this.sourceRepoConfig, ok = this.repoConfigs[this.sourceId]
	if !ok {
		err = fmt.Errorf("no configuration found for repo '%s'", this.sourceId)
		return
	}

	// Logging
	log := zerolog.Ctx(ctx).With().
		Str("sourceId", this.sourceId).
		Str("sourceUrl", this.sourceRepoConfig.URL).
		Logger()
	ctx = log.WithContext(ctx)

	// Source authentication
	var sourceAuth transport.AuthMethod
	sourceAuth, err = configToAuth(this.sourceRepoConfig)
	if err != nil {
		err = this.sourceRepoError("failed to configure auth", err)
		return
	}
	this.fetchOptions = git.FetchOptions{
		RemoteURL:  this.sourceRepoConfig.URL,
		RemoteName: this.sourceId,
		Auth:       sourceAuth,
		Force:      true,
		RefSpecs:   []gitconf.RefSpec{gitconf.RefSpec(refSpecFetch)},
	}

	// Clone
	cloneOptions := git.CloneOptions{
		URL:          this.sourceRepoConfig.URL,
		RemoteName:   this.sourceId,
		Mirror:       true,
		SingleBranch: true,
		Auth:         sourceAuth,
	}
	if this.sourceRepoConfig.InMemory {
		log.Debug().Msgf("Cloning '%s' to memory", this.sourceRepoConfig.URL)
		this.repo, err = git.CloneContext(ctx, memory.NewStorage(), nil, &cloneOptions)
	} else {
		path := this.sourceRepoConfig.LocalPath
		if path == "" {
			log.Debug().Msgf("Preparing temp directory for '%s'", this.sourceRepoConfig.URL)
			path, err = os.MkdirTemp("", fmt.Sprintf("git-sync-%s", this.sourceId))
			if err != nil {
				err = this.sourceRepoError("failed to create temporary directory", err)
				return
			}
		}

		log.Debug().Msgf("Cloning '%s' to %s", this.sourceRepoConfig.URL, path)
		this.repo, err = git.PlainCloneContext(ctx, path, true, &cloneOptions)
		if err == git.ErrRepositoryAlreadyExists {
			log.Debug().Msgf("Repo exists at %s. Opening.", path)
			this.repo, err = git.PlainOpen(path)
			if err != nil {
				err = this.sourceRepoError(fmt.Sprintf("failed to open path %s", path), err)
				return
			}
			err = prepareRemote(this.repo, this.sourceId, this.sourceRepoConfig, &log)
			if err != nil {
				err = this.sourceRepoError("failed to prepare remote", err)
			}
		}
	}
	if err != nil {
		err = this.sourceRepoError("failed to clone repo", err)
		return
	}

	// Configure targets
	targetPushOptions := make(map[string]git.PushOptions, len(link.Targets))
	for _, targetId := range link.Targets {
		targetRepoConfig, ok := repoConfigs[targetId]
		if !ok {
			err = fmt.Errorf("no configuration found for repo '%s'", targetId)
			return
		}
		log := log.With().
			Str("targetId", targetId).
			Str("targetUrl", targetRepoConfig.URL).
			Logger()

		var authMethod transport.AuthMethod
		authMethod, err = configToAuth(&targetRepoConfig)
		if err != nil {
			err = &GitRepoError{
				RepoId:  targetId,
				RepoURL: targetRepoConfig.URL,
				Reason:  "failed to configure auth",
				Cause:   err,
			}
			return
		}
		targetPushOptions[targetId] = git.PushOptions{
			RemoteName: targetId,
			RemoteURL:  targetRepoConfig.URL,
			Auth:       authMethod,
			Force:      true,
		}
		err = prepareRemote(this.repo, targetId, &targetRepoConfig, &log)
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

	// Configure matchers
	if err = this.branchMatcher.Init(this.link.Branches); err != nil {
		return this.sourceRepoError("failed to build branch matchers", err)
	}
	if err = this.tagMatcher.Init(this.link.Tags); err != nil {
		return this.sourceRepoError("failed to build tag matchers", err)
	}

	return nil
}

func (this *GitSync) Run() error {
	sourceRemote, err := this.repo.Remote(this.sourceId)
	if err != nil {
		return this.sourceRepoError("failed to query remote", err)
	}

	branches, tags, err := this.getBranchesAndTags(sourceRemote)
	if err != nil {
		return this.sourceRepoError("failed to fetch branches and tags", err)
	}

	err = sourceRemote.Fetch(&this.fetchOptions)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return this.sourceRepoError("failed to fetch from remote", err)
	}

	// push branches to new remote
	// push tags to new remote
	return nil
}

func (this *GitSync) getBranchesAndTags(
	remote *git.Remote,
) (branches []string, tags []string, err error) {
	var refs []*plumbing.Reference

	refs, err = remote.List(&git.ListOptions{})
	if err != nil {
		err = fmt.Errorf("failed to list refs for remote '%s': %w", this.sourceId, err)
	}

	branches = make([]string, 0, 1000)
	tags = make([]string, 0, 1000)

	for _, ref := range refs {
		refName := ref.Name().String()
		if strings.HasPrefix(refName, refBranchPrefix) {
			branch := strings.TrimPrefix(refName, refBranchPrefix)
			if this.branchMatcher.Match(branch) {
				branches = append(branches, branch)
			}
		} else if strings.HasPrefix(refName, refTagPrefix) {
			tag := strings.TrimPrefix(refName, refTagPrefix)
			if this.tagMatcher.Match(tag) {
				tags = append(tags, tag)
			}
		}
	}

	return
}

func prepareRemote(
	repo *git.Repository,
	remoteId string,
	repoConfig *config.Repository,
	log *zerolog.Logger,
) error {
	log.Debug().Msgf("Querying remote '%s'", remoteId)
	remote, err := repo.Remote(remoteId)

	if err != nil && err != git.ErrRemoteNotFound {
		return fmt.Errorf("failed to get remote '%s': %w", remoteId, err)
	}
	if err == nil {
		if slices.Contains(remote.Config().URLs, repoConfig.URL) {
			log.Debug().Msgf("Remote '%s' configured already", remoteId)
			return nil
		}
		log.Debug().Msgf("Deleting remote '%s' before reconfiguring it", remoteId)
		if err := repo.DeleteRemote(remoteId); err != nil {
			return fmt.Errorf("failed to delete remote '%s': %w", remoteId, err)
		}
	}

	log.Debug().Msgf("Creating remote '%s'", remoteId)
	_, err = repo.CreateRemote(&gitconf.RemoteConfig{
		Name:   remoteId,
		URLs:   []string{repoConfig.URL},
		Mirror: true,
	})
	if err != nil {
		return fmt.Errorf("failed to create remote '%s'", remoteId, err)
	}

	return nil
}

func configToAuth(
	repoConfig *config.Repository,
) (transport.AuthMethod, error) {
	if repoConfig.HttpToken != "" {
		return &http.TokenAuth{
			Token: repoConfig.HttpToken,
		}, nil
	}
	if repoConfig.HttpCredentials.Username != "" && repoConfig.HttpCredentials.Password != "" {
		return &http.BasicAuth{
			Username: repoConfig.HttpCredentials.Username,
			Password: repoConfig.HttpCredentials.Password,
		}, nil
	}
	if repoConfig.SshCredentials.UseAgent {
		return ssh.NewSSHAgentAuth(repoConfig.SshCredentials.Username)
	}
	if repoConfig.SshCredentials.KeyPath != "" {
		return ssh.NewPublicKeysFromFile(
			repoConfig.SshCredentials.Username,
			repoConfig.SshCredentials.KeyPath,
			repoConfig.SshCredentials.KeyPassword,
		)
	}
	return nil, nil
}
