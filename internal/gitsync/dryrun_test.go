package gitsync

import (
	"bytes"
	"testing"
	"time"

	"go.lepovirta.org/otk/internal/duration"
	"go.lepovirta.org/otk/internal/gitsync/config"
	"go.lepovirta.org/otk/internal/matcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testConfig = config.Config{
	Repositories: map[string]config.Repository{
		"otk-github": {
			Credentials: config.Credentials{
				SshCredentials: config.SshCredentials{
					UseAgent: true,
				},
			},
			URL:      "ssh://github.com:jpallari/otk.git",
			InMemory: true,
		},
		"keruu-github": {
			Credentials: config.Credentials{
				HttpCredentials: config.HttpCredentials{
					Username: "testuser",
					Password: "testuser_password",
				},
			},
			URL: "https://github.com/jpallari/keruu.git",
		},
		"yahe-github": {
			LocalPath: "/home/testuser/git/yahe.git",
			URL:       "https://github.com/jpallari/yahe.git",
		},
		"otk-gitlab": {
			Credentials: config.Credentials{
				TargetAuthMethod: config.AuthMethodSshKey,
				SshCredentials: config.SshCredentials{
					KeyPath:     "./gitlab/ssh-key.ed25519",
					KeyPassword: "gitlab_ssh_password",
				},
			},
			URL: "ssh://gitlab.com:gitlabuser/otk.git",
		},
		"keruu-gitlab": {
			Credentials: config.Credentials{
				TargetAuthMethod: config.AuthMethodHttpToken,
				HttpToken:        "http_token",
			},
			URL: "https://gitlab.com/gitlabuser/keruu.git",
		},
		"keruu-ssh": {
			Credentials: config.Credentials{
				SshCredentials: config.SshCredentials{
					KeyPassword:   "ssh_key_password",
					KeyPath:       "/home/testuser/.ssh/ssh-key.ed25519",
					IgnoreHostKey: true,
				},
			},
			URL: "ssh://192.168.100.69/srv/git/keruu.git",
		},
		"yahe-gitlab": {
			Credentials: config.Credentials{
				HttpCredentials: config.HttpCredentials{
					Username: "http_username",
					Password: "http_password",
				},
			},
			URL: "https://gitlab.com/gitlabuser/yahe.git",
		},
	},
	Mappings: []config.SyncMapping{
		{
			Source:  "otk-github",
			Targets: []string{"otk-gitlab"},
			SyncSpec: config.SyncSpec{
				Interval: duration.New(time.Hour),
				Branches: []matcher.M{
					matcher.FromStringOrPanic("main"),
				},
				Tags: []matcher.M{
					matcher.FromStringOrPanic(`/v.*/`),
				},
			},
		},
		{
			Source:  "keruu-github",
			Targets: []string{"keruu-gitlab", "keruu-ssh"},
			SyncSpec: config.SyncSpec{
				Interval: duration.New(time.Duration(6) * time.Hour),
				Branches: []matcher.M{
					matcher.FromStringOrPanic(`/main.*/`),
				},
				Tags: []matcher.M{},
			},
		},
		{
			Source:  "yahe-github",
			Targets: []string{"yahe-gitlab"},
			SyncSpec: config.SyncSpec{
				Interval: duration.New(time.Duration(48) * time.Hour),
				Branches: []matcher.M{
					matcher.FromStringOrPanic("main"),
				},
				Tags: []matcher.M{
					matcher.FromStringOrPanic(`/release-.*/`),
				},
			},
		},
	},
}

const testConfigDryRunText = `!! DRY RUN !! Use flag -run to sync the following Git repos

sync: otk-github --> otk-gitlab
      otk-github = ssh://github.com:jpallari/otk.git (auth: ssh-agent)
      otk-gitlab = ssh://gitlab.com:gitlabuser/otk.git (auth: ssh)
      branches = main
      tags = /v.*/

sync: keruu-github --> keruu-gitlab, keruu-ssh
      keruu-github = https://github.com/jpallari/keruu.git (auth: http)
      keruu-gitlab = https://gitlab.com/gitlabuser/keruu.git (auth: http-token)
      keruu-ssh = ssh://192.168.100.69/srv/git/keruu.git (auth: ssh)
      branches = /main.*/

sync: yahe-github --> yahe-gitlab
      yahe-github = https://github.com/jpallari/yahe.git (auth: none)
      yahe-gitlab = https://gitlab.com/gitlabuser/yahe.git (auth: http)
      branches = main
      tags = /release-.*/
`


func TestDryRun(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	var out bytes.Buffer
	out.Grow(2 * 1024)

	require.NoError(dryRun(&out, &testConfig), "dry run")

	assert.Equal(testConfigDryRunText, out.String())
}
