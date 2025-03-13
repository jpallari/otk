package config

import (
	"bytes"
	"testing"
	"time"

	"go.lepovirta.org/otk/internal/duration"
	"go.lepovirta.org/otk/internal/envvar"
	"go.lepovirta.org/otk/internal/matcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var envVarsMap = map[string]string{
	"HOME":                    "/home/testuser",
	"GITLAB_SSH_KEY_PASSWORD": "gitlab_ssh_password",
	"GITLAB_USERNAME":         "gitlabuser",
}

const goodConfigJson = `
{
  "repositories": {
    "otk-github": {
      "sshCredentials": {
        "useAgent": true
      },
      "url": "ssh://github.com:jpallari/otk.git",
      "inMemory": true
    },
    "keruu-github": {
      "httpCredentials": {
        "username": "testuser"
      },
      "url": "https://github.com/jpallari/keruu.git"
    },
    "yahe-github": {
      "localPath": "${HOME}/git/yahe.git",
      "url": "https://github.com/jpallari/yahe.git"
    },
    "otk-gitlab": {
      "authMethod": "ssh",
      "sshCredentials": {
        "keyPath": "./gitlab/ssh-key.ed25519",
        "keyPassword": "${GITLAB_SSH_KEY_PASSWORD}"
      },
      "url": "ssh://gitlab.com:${GITLAB_USERNAME}/otk.git"
    },
    "keruu-gitlab": {
      "authMethod": "http-token",
      "url": "https://gitlab.com/${GITLAB_USERNAME}/keruu.git"
    },
    "keruu-ssh": {
      "sshCredentials": {
        "keyPath": "${HOME}/.ssh/ssh-key.ed25519",
        "ignoreHostKey": true
      },
      "url": "ssh://192.168.100.69/srv/git/keruu.git"
    },
    "yahe-gitlab": {
      "url": "https://gitlab.com/${GITLAB_USERNAME}/yahe.git"
    }
  },
  "mappings": [
    {
      "source": "otk-github",
      "targets": [ "otk-gitlab" ],
      "interval": "60m",
      "branches": [ { "spec": "main" } ],
      "tags": [
        { "spec": "v.*", "useRegex": true }
      ]
    },
    {
      "source": "keruu-github",
      "targets": [ "keruu-gitlab", "keruu-ssh" ],
      "interval": "6h",
      "branches": [
        { "spec": "main.*", "useRegex": true }
      ],
      "tags": []
    },
    {
      "source": "yahe-github",
      "targets": [ "yahe-gitlab" ],
      "interval": "48h",
      "branches": [ { "spec": "main" } ],
      "tags": [
        { "spec": "release-.*", "useRegex": true }
      ]
    }
  ]
}
`

const goodCredentialsJson = `
{
  "keruu-github": {
    "httpCredentials": {
      "password": "testuser_password"
    }
  },
  "keruu-gitlab": {
    "httpToken": "http_token"
  },
  "keruu-ssh": {
    "sshCredentials": {
      "keyPassword": "ssh_key_password"
    }
  },
  "yahe-gitlab": {
    "httpCredentials": {
      "username": "http_username",
      "password": "http_password"
    }
  }
}
`

var goodConfig = Config{
	Repositories: map[string]Repository{
		"otk-github": {
			Credentials: Credentials{
				SshCredentials: SshCredentials{
					UseAgent: true,
				},
			},
			URL:      "ssh://github.com:jpallari/otk.git",
			InMemory: true,
		},
		"keruu-github": {
			Credentials: Credentials{
				HttpCredentials: HttpCredentials{
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
			Credentials: Credentials{
				TargetAuthMethod: AuthMethodSshKey,
				SshCredentials: SshCredentials{
					KeyPath:     "./gitlab/ssh-key.ed25519",
					KeyPassword: "gitlab_ssh_password",
				},
			},
			URL: "ssh://gitlab.com:gitlabuser/otk.git",
		},
		"keruu-gitlab": {
			Credentials: Credentials{
				TargetAuthMethod: AuthMethodHttpToken,
				HttpToken:        "http_token",
			},
			URL: "https://gitlab.com/gitlabuser/keruu.git",
		},
		"keruu-ssh": {
			Credentials: Credentials{
				SshCredentials: SshCredentials{
					KeyPassword:   "ssh_key_password",
					KeyPath:       "/home/testuser/.ssh/ssh-key.ed25519",
					IgnoreHostKey: true,
				},
			},
			URL: "ssh://192.168.100.69/srv/git/keruu.git",
		},
		"yahe-gitlab": {
			Credentials: Credentials{
				HttpCredentials: HttpCredentials{
					Username: "http_username",
					Password: "http_password",
				},
			},
			URL: "https://gitlab.com/gitlabuser/yahe.git",
		},
	},
	Mappings: []SyncMapping{
		{
			Source:  "otk-github",
			Targets: []string{"otk-gitlab"},
			SyncSpec: SyncSpec{
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
			SyncSpec: SyncSpec{
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
			SyncSpec: SyncSpec{
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

func TestParseGood(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	var conf Config
	var envVars envvar.Vars
	envVars.FromMap(envVarsMap)
	configStream := bytes.NewBufferString(goodConfigJson)
	credentialsStream := bytes.NewBufferString(goodCredentialsJson)

	err := conf.Parse(envVars, configStream, credentialsStream)
	require.NoError(err, "config parse")

	assert.Equal(goodConfig, conf)
}
