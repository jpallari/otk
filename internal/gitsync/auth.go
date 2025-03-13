package gitsync

import (
	"fmt"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"go.lepovirta.org/otk/internal/gitsync/config"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"
)

const defaultGitUsername = "git"

func configToAuth(
	fs billy.Filesystem,
	repoConfig *config.Repository,
	log *zerolog.Logger,
) (transport.AuthMethod, error) {
	switch repoConfig.AuthMethod() {
	case config.AuthMethodNone:
		log.Debug().Msg("no auth method selected")
		return nil, nil
	case config.AuthMethodHttpToken:
		log.Debug().Msg("using http token for auth")
		return &http.TokenAuth{
			Token: repoConfig.HttpToken,
		}, nil
	case config.AuthMethodHttpCredentials:
		log.Debug().Msg("using http basic for auth")
		return &http.BasicAuth{
			Username: repoConfig.HttpCredentials.Username,
			Password: repoConfig.HttpCredentials.Password,
		}, nil
	case config.AuthMethodSshAgent:
		username := repoConfig.SshCredentials.Username
		if username == "" {
			username = defaultGitUsername
		}
		log.Debug().Msgf("using ssh agent auth with username '%s'", username)
		auth, err := gitssh.NewSSHAgentAuth(repoConfig.SshCredentials.Username)
		if err != nil {
			return nil, fmt.Errorf("failed to configure SSH agent auth: %w", err)
		}
		return auth, nil
	case config.AuthMethodSshKey:
		return sshKeyAuth(fs, repoConfig.SshCredentials, log)
	default:
		return nil, fmt.Errorf("unknown auth method")
	}
}

func sshKeyAuth(
	fs billy.Filesystem,
	creds config.SshCredentials,
	log *zerolog.Logger,
) (transport.AuthMethod, error) {
	username := creds.Username
	if username == "" {
		username = defaultGitUsername
	}
	log.Debug().Msgf("using ssh key auth with username '%s'", username)

	sshKeyBytes, err := util.ReadFile(fs, creds.KeyPath)
	log.Debug().Msgf("ssh key read (bytes=%d)", len(sshKeyBytes))
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read SSH key from path '%s': %w",
			creds.KeyPath,
			err,
		)
	}

	auth, err := gitssh.NewPublicKeys(
		username,
		sshKeyBytes,
		creds.KeyPassword,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to configure SSH key auth: %w", err)
	}

	if creds.IgnoreHostKey {
		log.Warn().Msg("disabling SSH host key check")
		auth.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	} else if creds.HostKey != "" {
		pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(creds.HostKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH host key: %w", err)
		}
		auth.HostKeyCallback = ssh.FixedHostKey(pubKey)
	} else {
		// TODO: rewrite the hosts callback to use virtualised FS
		auth.HostKeyCallback, err = gitssh.NewKnownHostsCallback(creds.KnownHostsPaths...)
		if err != nil {
			return nil, fmt.Errorf("failed to create host key callback: %w", err)
		}
	}

	return auth, nil
}
