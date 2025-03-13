package gitsynctesting

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	"go.lepovirta.org/otk/internal/gitsync"
	"go.lepovirta.org/otk/internal/osenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/crypto/ssh"
)

const (
	sshKeyComment  = "gitsynctesting"
	sshKeyPassword = "supersecretpassword"
	sshKeyPath     = "ssh-key"
	gitAuthorName  = "Git Sync"
	gitAuthorEmail = "gitsync@example.org"
)

var timestamp = time.Date(2025, 4, 27, 15, 46, 0, 0, time.FixedZone("", 0))

var testData = []struct {
	fs       billy.Filesystem
	repoName string
	targets  []struct {
		fs       billy.Filesystem
		repoName string
	}
	commits []commit
}{
	{
		fs:       memfs.New(),
		repoName: "test-s1",
		targets: []struct {
			fs       billy.Filesystem
			repoName string
		}{
			{
				repoName: "test-t1",
				fs:       memfs.New(),
			},
		},
		commits: []commit{
			{
				message: "s1-1",
				files: map[string]string{
					"README1.txt": "source 1 commit 1",
				},
				timestamp: timestamp,
			},
			{
				message: "s1-2",
				files: map[string]string{
					"README1.txt": "source 1 commit 2",
				},
				timestamp: timestamp.Add(24 * time.Hour),
				tags:      []string{"v1.0"},
			},
			{
				message: "s1-3",
				files: map[string]string{
					"README1.txt": "source 1 commit 3",
				},
				timestamp: timestamp.Add(48 * time.Hour),
				tags:      []string{"v1.1"},
			},
		},
	},
	{
		fs:       memfs.New(),
		repoName: "test-s2",
		targets: []struct {
			fs       billy.Filesystem
			repoName string
		}{
			{
				repoName: "test-t2",
				fs:       memfs.New(),
			},
			{
				repoName: "test-t3",
				fs:       memfs.New(),
			},
		},
		commits: []commit{
			{
				message: "s2-1",
				files: map[string]string{
					"README2.txt": "source 2 commit 1",
				},
				timestamp: timestamp,
			},
			{
				message: "s2-2",
				files: map[string]string{
					"README2.txt": "source 2 commit 2",
				},
				timestamp: timestamp.Add(24 * time.Hour),
			},
		},
	},
	{
		fs:       memfs.New(),
		repoName: "test-s3",
		targets: []struct {
			fs       billy.Filesystem
			repoName string
		}{
			{
				repoName: "test-t4",
				fs:       memfs.New(),
			},
		},
		commits: []commit{
			{
				message: "s3-1",
				files: map[string]string{
					"README3.txt": "source 3 commit 1",
				},
				timestamp: timestamp,
			},
		},
	},
}

func TestIntegration(t *testing.T) {
	require := require.New(t)
	ctx := t.Context()

	// Setup OS env
	osEnv := osenv.OsEnv{
		Args: []string{
			"otk-gitsync",
			"-config", "config.json",
			"-run",
			"-once",
		},
		Fs:     memfs.New(),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	// Generate SSH key pair
	sshPublicKey, sshPrivateKey, err := generateSshKeyPair([]byte(sshKeyPassword), sshKeyComment)
	require.NoError(err, "ssh key gen")
	require.NoError(util.WriteFile(osEnv.Fs, sshKeyPath, sshPrivateKey, 0o600), "ssh key write")

	// Copy config to env
	testDir := currentDir()
	require.NoError(copyFileFromOsFs(
		osEnv.Fs,
		filepath.Join(testDir, "gitsync.config.json"),
		"config.json",
	))

	// Collect repo names
	repoNames := make([]string, 0, 100)
	for _, data := range testData {
		repoNames = append(repoNames, data.repoName)
		for _, target := range data.targets {
			repoNames = append(repoNames, target.repoName)
		}
	}

	// Start SSH server container
	var sshHost string
	var sshPort int
	{
		containerDir := filepath.Join(testDir, "..", "git-server-ssh")
		req := testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:   containerDir,
				KeepImage: true,
			},
			ExposedPorts: []string{"22/tcp"},
			WaitingFor:   wait.ForExposedPort(),
			Env: map[string]string{
				"SSH_AUTHORIZED_KEY": string(sshPublicKey),
				"GIT_REPOS":          strings.Join(repoNames, " "),
			},
		}
		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
		testcontainers.CleanupContainer(t, container)
		require.NoError(err, "git-server start")
		sshHost, err = container.Host(ctx)
		require.NoError(err)
		sshContainerPort, err := container.MappedPort(ctx, "22/tcp")
		require.NoError(err, "git-server port")
		sshPort = sshContainerPort.Int()
	}

	// SSH connection details from env vars
	osEnv.EnvVars.FromMap(map[string]string{
		"GITSYNC_SSH_HOST":         sshHost,
		"GITSYNC_SSH_PORT":         strconv.Itoa(sshPort),
		"GITSYNC_SSH_KEY_PASSWORD": sshKeyPassword,
		"GITSYNC_LOG_LEVEL":        "debug",
	})

	// Prepare test git repos
	for _, data := range testData {
		repo, err := setupTestRepository(ctx, data.fs, data.repoName, sshHost, sshPort)
		require.NoErrorf(err, "git init %s", data.repoName)
		err = pushCommits(ctx, repo, sshPrivateKey, data.commits)
		require.NoErrorf(err, "git commit %s", data.repoName)
	}

	// Run gitsync
	var core gitsync.Core
	require.NoErrorf(core.Init(osEnv), "core init")
	require.NoErrorf(core.Run(ctx), "core run")

	// Fetch targets and verify commits
	for _, data := range testData {
		for _, target := range data.targets {
			repo, err := setupTestRepository(ctx, target.fs, target.repoName, sshHost, sshPort)
			require.NoErrorf(err, "git init %s", target.repoName)
			err = fetchRepo(ctx, repo, sshPrivateKey)
			require.NoErrorf(err, "git pull %s", data.repoName)
			err = verifyCommits(t, repo, data.commits)
			require.NoErrorf(err, "git verify %s", data.repoName)
		}
	}
}

type commit struct {
	message   string
	files     map[string]string
	tags      []string
	timestamp time.Time
}

func setupTestRepository(
	ctx context.Context,
	worktree billy.Filesystem,
	repoName string,
	sshHost string,
	sshPort int,
) (*git.Repository, error) {
	repo, err := git.Init(memory.NewStorage(), worktree)
	if err != nil {
		return nil, fmt.Errorf("failed to init repo '%s': %w", repoName, err)
	}
	_, err = repo.CreateRemote(&gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{
			fmt.Sprintf("ssh://%s:%d/srv/git/%s.git", sshHost, sshPort, repoName),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create remote for repo '%s': %w", repoName, err)
	}
	err = repo.CreateBranch(&gitconfig.Branch{
		Name:   "main",
		Remote: "origin",
		Merge:  plumbing.Main,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create main branch for repo '%s': %w", repoName, err)
	}
	return repo, err
}

func gitSshAuth(sshPrivateKey []byte) (*gitssh.PublicKeys, error) {
	auth, err := gitssh.NewPublicKeys(
		"git",
		sshPrivateKey,
		sshKeyPassword,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create git auth: %w", err)
	}
	auth.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	return auth, nil
}

func pushCommits(
	ctx context.Context,
	repo *git.Repository,
	sshPrivateKey []byte,
	commits []commit,
) error {
	auth, err := gitSshAuth(sshPrivateKey)
	if err != nil {
		return err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	var hash plumbing.Hash
	for commitNr, commit := range commits {
		for filename, contents := range commit.files {
			if err := util.WriteFile(worktree.Filesystem, filename, []byte(contents), 0o666); err != nil {
				return fmt.Errorf(
					"failed to write file %s in commit %d: %w",
					filename, commitNr, err,
				)
			}
			if _, err := worktree.Add(filename); err != nil {
				return fmt.Errorf(
					"failed to stage file %s in commit %d: %w",
					filename, commitNr, err,
				)
			}
		}
		hash, err = worktree.Commit(commit.message, &git.CommitOptions{
			Author: &object.Signature{
				Name:  gitAuthorName,
				Email: gitAuthorEmail,
				When:  commit.timestamp,
			},
		})
		if err != nil {
			return fmt.Errorf(
				"failed to commit %d: %w", commitNr, err,
			)
		}
		for tagNr, tag := range commit.tags {
			if _, err := repo.CreateTag(tag, hash, nil); err != nil {
				return fmt.Errorf(
					"failed to create tag %s (%d) on commit %s: %w",
					tag, tagNr, hash.String(), err,
				)
			}
		}
	}

	if err := repo.Storer.SetReference(
		plumbing.NewHashReference(plumbing.Main, hash),
	); err != nil {
		return fmt.Errorf("failed to setup main branch: %w", err)
	}
	err = repo.PushContext(ctx, &git.PushOptions{
		RemoteName: "origin",
		Auth:       auth,
		RefSpecs: []gitconfig.RefSpec{
			gitconfig.RefSpec("refs/heads/*:refs/heads/*"),
			gitconfig.RefSpec("refs/tags/*:refs/tags/*"),
		},
	})
	if err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}
	return nil
}

func fetchRepo(
	ctx context.Context,
	repo *git.Repository,
	sshPrivateKey []byte,
) error {
	auth, err := gitSshAuth(sshPrivateKey)
	if err != nil {
		return err
	}
	if err := repo.FetchContext(ctx, &git.FetchOptions{
		RemoteName: "origin",
		Auth:       auth,
	}); err != nil {
		return err
	}
	return nil
}

func verifyCommits(
	t *testing.T,
	repo *git.Repository,
	commits []commit,
) error {
	assert := assert.New(t)

	originMainRefName := plumbing.NewRemoteReferenceName("origin", "main")
	originMainRef, err := repo.Reference(originMainRefName, false)
	if err != nil {
		return fmt.Errorf("failed to resolve ref '%s': %w", originMainRefName, err)
	}

	cIter, err := repo.Log(&git.LogOptions{
		From: originMainRef.Hash(),
	})
	if err != nil {
		return fmt.Errorf("failed to get log iterator for '%s': %w", originMainRef, err)
	}

	cIndex := len(commits) - 1
	if err := cIter.ForEach(func(actual *object.Commit) error {
		expected := commits[cIndex]
		assert.Equal(gitAuthorName, actual.Author.Name)
		assert.Equal(gitAuthorEmail, actual.Author.Email)
		assert.Equal(expected.message, actual.Message)
		assert.Equal(expected.timestamp, actual.Author.When)

		fIter, err := actual.Files()
		if err != nil {
			return fmt.Errorf("failed to get file iterator: %w", err)
		}
		if err := fIter.ForEach(func(actualFile *object.File) error {
			expectedContents, ok := expected.files[actualFile.Name]
			if !assert.Truef(ok, "file '%s' exists", actualFile.Name) {
				return nil
			}
			actualContents, err := actualFile.Contents()
			if err != nil {
				return fmt.Errorf("failed to get file '%s' contents: %w", actualFile.Name, err)
			}
			assert.Equal(expectedContents, actualContents)
			return nil
		}); err != nil {
			return fmt.Errorf("file iteration failed: %w", err)
		}

		cIndex -= 1
		return nil
	}); err != nil {
		return fmt.Errorf("iteration failed: %w", err)
	}
	return nil
}
