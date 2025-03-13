package config

import (
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"io"
	"os"
	"strings"
	"time"
)

type Config struct {
	Repositories map[string]Repository `json:"repositories"`
	Links        []Link                `json:"links"`
	Log          Logging               `json:"log"`
}

type Repository struct {
	Credentials
	AuthMethod string `json:"authMethod"`
	URL        string `json:"url"`
	InMemory   bool   `json:"inMemory"`
	LocalPath  string `json:"localPath"`
}

type Credentials struct {
	HttpToken       string          `json:"httpToken"`
	HttpCredentials HttpCredentials `json:"httpCredentials"`
	SshCredentials  SshCredentials  `json:"sshCredentials"`
}

type HttpCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type SshCredentials struct {
	UseAgent    bool   `json:"useAgent"`
	Username    string `json:"username"`
	KeyPath     string `json:"keyPath"`
	KeyPassword string `json:"keyPassword"`
}

type Link struct {
	Source   string        `json:"source"`
	Targets  []string      `json:"targets"`
	Internal time.Duration `json:"interval"`
	Branches []TargetSpec  `json:"branches"`
	Tags     []TargetSpec  `json:"tags"`
}

type TargetSpec struct {
	Spec     string `json:"spec"`
	UseRegex bool   `json:"useRegex"`
}

type Logging struct {
	Format string `json:"format"`
	Level  string `json:"level"`
}

func (this *Credentials) merge(other *Credentials) {
	overrideStr(&this.HttpToken, other.HttpToken)
	overrideStr(&this.HttpCredentials.Username, other.HttpCredentials.Username)
	overrideStr(&this.HttpCredentials.Password, other.HttpCredentials.Password)
	overrideBool(&this.SshCredentials.UseAgent, other.SshCredentials.UseAgent)
	overrideStr(&this.SshCredentials.KeyPath, other.SshCredentials.KeyPath)
	overrideStr(&this.SshCredentials.KeyPassword, other.SshCredentials.KeyPassword)
}

func (this *Config) mergeCredentials(credentials map[string]Credentials) {
	// TODO: move this out of here, it will not be called when credentials stream is not specified
	httpToken := os.Getenv("GIT_SYNC_HTTP_TOKEN")
	httpUsername := os.Getenv("GIT_SYNC_HTTP_USERNAME")
	httpPassword := os.Getenv("GIT_SYNC_HTTP_PASSWORD")
	sshUseAgent := os.Getenv("GIT_SYNC_USE_AGENT") == "true"
	sshKeyPath := os.Getenv("GIT_SYNC_SSH_KEY_PATH")
	sshKeyPassword := os.Getenv("GIT_SYNC_SSH_KEY_PASSWORD")

	for repoId, creds := range credentials {
		repo, ok := this.Repositories[repoId]
		if ok {
			repo.merge(&creds)
			defaultStr(&repo.HttpToken, httpToken)
			defaultStr(&repo.HttpCredentials.Username, httpUsername)
			defaultStr(&repo.HttpCredentials.Password, httpPassword)
			defaultBool(&repo.SshCredentials.UseAgent, sshUseAgent)
			defaultStr(&repo.SshCredentials.KeyPath, sshKeyPath)
			defaultStr(&repo.SshCredentials.KeyPassword, sshKeyPassword)
		} else {
			log.Warn().Msgf(
				"credentials specified for repository '%s' but the repository is not found in the configuration",
				repoId,
			)
		}
	}
}

func (this *Link) validate() error {
	if this.Source == "" {
		return fmt.Errorf("source must be defined")
	}
	if len(this.Targets) == 0 {
		return fmt.Errorf("at least one target must be defined")
	}
	for i, target := range this.Targets {
		if target == "" {
			return fmt.Errorf("target must be specified (target at index %d)", i)
		}
	}
	if this.Internal.Seconds() < 10 {
		return fmt.Errorf("interval must be 10 seconds or more")
	}
	if len(this.Branches) == 0 && len(this.Tags) == 0 {
		return fmt.Errorf("at least one branch or tag spec must be specified")
	}
	return nil
}

func (this *Repository) validate() error {
	if this.URL == "" {
		return fmt.Errorf("repository URL cannot be empty")
	}

	switch strings.ToLower(this.AuthMethod) {
	case "":
		break
	case "ssh-agent":
		if !this.SshCredentials.UseAgent {
			return fmt.Errorf("expected SSH agent to be enabled")
		}
	case "ssh-key":
		if this.SshCredentials.KeyPath == "" {
			return fmt.Errorf("expected SSH key path to be set")
		}
	case "http-basic":
		if this.HttpCredentials.Username == "" {
			return fmt.Errorf("expected HTTP username to be set")
		}
		if this.HttpCredentials.Password == "" {
			return fmt.Errorf("expected HTTP password to be set")
		}
	case "http-token":
		if this.HttpToken == "" {
			return fmt.Errorf("expected HTTP token to be set")
		}
	default:
		return fmt.Errorf("unexpected auth method: %s", this.AuthMethod)
	}
	return nil
}

func (this *TargetSpec) validate() error {
	if this.Spec == "" {
		return fmt.Errorf("spec cannot be empty")
	}
	return nil
}

func (this *Logging) validate() error {
	switch strings.ToLower(this.Format) {
	case "json", "pretty", "":
		break
	default:
		return fmt.Errorf("invalid log format: %s", this.Format)
	}
	return nil
}

func (this *Config) validate() error {
	if len(this.Repositories) == 0 {
		return fmt.Errorf("at least one repository must be specified")
	}
	if len(this.Links) == 0 {
		return fmt.Errorf("at least one link must be specified")
	}
	if err := this.Log.validate(); err != nil {
		return fmt.Errorf("logging: %w", err)
	}

	for repoId, repo := range this.Repositories {
		if err := repo.validate(); err != nil {
			return fmt.Errorf("repository '%s': %w", repoId, err)
		}
	}
	for i, link := range this.Links {
		if err := link.validate(); err != nil {
			return fmt.Errorf("link on index %d: %w", i, err)
		}
		if _, ok := this.Repositories[link.Source]; !ok {
			return fmt.Errorf("source %s for link on index %d does not exist", link.Source, i)
		}
		if _, ok := this.Repositories[link.Targets]; !ok {
			return fmt.Errorf("target %s for link on index %d does not exist", link.Targets, i)
		}
	}

	return nil
}

func (this *Config) FromJSON(config io.Reader, credentials io.Reader) error {
	if err := json.NewDecoder(config).Decode(this); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	if credentials != nil {
		var parsedCreds map[string]Credentials
		if err := json.NewDecoder(credentials).Decode(&parsedCreds); err != nil {
			return fmt.Errorf("failed to parse credentials: %w", err)
		}
		this.mergeCredentials(parsedCreds)
	}

	return this.validate()
}
