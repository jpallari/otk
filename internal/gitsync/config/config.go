package config

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"go.lepovirta.org/otk/internal/duration"
	"go.lepovirta.org/otk/internal/envsubst"
	"go.lepovirta.org/otk/internal/envvar"
	"go.lepovirta.org/otk/internal/matcher"
	"go.lepovirta.org/otk/internal/validation"
	"github.com/rs/zerolog/log"
)

const (
	AppName             = "gitsync"
	envVarSubstErrorMsg = "environment variable substitution failed"
)

// ConfigSingle is used for syncing a single local Git repository
// to one or more remote repositories.
type ConfigSingle struct {
	// Path is the path to a Git repository in the file system that
	// is to be synchronised to remote repositories. By default,
	// the current working directory is used.
	Path string `json:"path"`

	// Targets contain the information on which Git repositories to
	// synchronise the local Git repository to. The target key is
	// used as the remote identifier during mirroring and in logs.
	Targets map[string]Target `json:"targets"`
}

// Target specifies the target Git repository to sync to
// and details on how to perform the synchronisation.
type Target struct {
	// Repository specifies the target Git repository details.
	Repository

	// SyncSpec specifies how and what to synchronise to the target repository.
	SyncSpec
}

// Config is used for syncing one or more Git repositories
// to one ore more remote repositories.
type Config struct {
	// Repositories specifies details of all of the Git repositories
	// involved during synchronisation. The key is the ID of the
	// repository, which is referenced in the mappings source and target fields.
	Repositories map[string]Repository `json:"repositories"`

	// Mappings specifies which Git repositories are synchronised where.
	Mappings []SyncMapping `json:"mappings"`
}

// Repository specifies details of a single Git repository
type Repository struct {
	// Credentials specifies the authentication credentials used
	// when connecting to the Git repository.
	Credentials

	// URL is the remote URL where the Git repository is located
	URL string `json:"url"`

	// When InMemory is set to `true`, the Git repository is downloaded
	// to memory rather than the file system.
	InMemory bool `json:"inMemory"`

	// LocalPath specifies the path where the Git repository is downloaded to.
	// When InMemory is set to `true`, this value is ignored.
	// When left unset, a temporary directory is created for the Git repository.
	LocalPath string `json:"localPath"`
}

// Credentials specifies the authentication credentials used
// when connecting to the Git repository.
type Credentials struct {
	// TargetAuthMethod specifies which authentication method is used
	// when connecting to the Git repository.
	TargetAuthMethod AuthMethod `json:"authMethod"`

	// HttpToken specifies a HTTP token used for connecting to HTTPS-based
	// Git repositories.
	HttpToken string `json:"httpToken"`

	// HttpCredentials specifies HTTP basic auth credentials used for
	// connecting to HTTPS-based Git repositories
	HttpCredentials HttpCredentials `json:"httpCredentials"`

	// SshCredentials specifies credentials used when connecting to
	// SSH-based Git repositories.
	SshCredentials SshCredentials `json:"sshCredentials"`
}

// HttpCredentials specifies HTTP basic auth credentials used for
// connecting to HTTPS-based Git repositories
type HttpCredentials struct {
	// Username is the HTTP basic auth username field
	Username string `json:"username"`

	// Password is the HTTP basic auth password field
	Password string `json:"password"`
}

// SshCredentials specifies credentials used when connecting to
// SSH-based Git repositories.
type SshCredentials struct {
	// When UseAgent is set to `true`, SSH agent is used for acquiring
	// the SSH key for connecting to the remote repository.
	UseAgent bool `json:"useAgent"`

	// Username is the SSH username to use for connecting to the Git repository.
	// Default value is "git".
	Username string `json:"username"`

	// KeyPath is the path to a SSH key used for connecting to the Git repository.
	KeyPath string `json:"keyPath"`

	// KeyPassword specifies the password for unlocking the SSH key specified in KeyPath.
	KeyPassword string `json:"keyPassword"`

	// HostKey is the SSH host key expected from the remote server.
	// When left unset, host key is checked from the known hosts file.
	// HostKey is supplied in authorized_keys format according to sshd(8) manual page.
	HostKey string `json:"hostKey"`

	// KnownHostsPaths points to file paths where known SSH hosts are recorded.
	// When left unset, the default hosts paths are used (e.g. ~/.ssh/known_hosts).
	// The files in the given paths must be in ssh_known_hosts format according to
	// sshd(8) manual page.
	KnownHostsPaths []string `json:"knownHostsPaths"`

	// When IgnoreHostKey is set to `true`, the SSH host key for the Git repository
	// is not verified. Not recommended to be used in production!
	IgnoreHostKey bool `json:"ignoreHostKey"`
}

// Mappings specifies which Git repositories are synchronised where.
type SyncMapping struct {
	// SyncSpec specifies what to sync to the target repository and when.
	SyncSpec

	// Source is the ID of the repository to sync to the targets.
	Source string `json:"source"`

	// Targets contains the IDs of the repositories to sync the source to.
	Targets []string `json:"targets"`
}

// SyncSpec specifies what to sync to the target Git repository and when.
type SyncSpec struct {
	// Interval specifies how frequently to synchronise the Git repository.
	// Default is 1 hour.
	Interval duration.D `json:"interval"`

	// Branches contains the matcher rules to determine which branches to
	// synchronise to the target Git repository.
	Branches []matcher.M `json:"branches"`

	// Tags contains the matcher rules to determine which tags to
	// synchronise to the target Git repository.
	Tags []matcher.M `json:"tags"`
}

/////////////////////////////////////////////////
// Auth method
/////////////////////////////////////////////////

func (this *Credentials) AuthMethod() AuthMethod {
	if this.TargetAuthMethod != AuthMethodUndefined {
		return this.TargetAuthMethod
	}
	if this.HttpToken != "" {
		return AuthMethodHttpToken
	}
	if this.HttpCredentials.enabled() {
		return AuthMethodHttpCredentials
	}
	if this.SshCredentials.UseAgent {
		return AuthMethodSshAgent
	}
	if this.SshCredentials.keyEnabled() {
		return AuthMethodSshKey
	}
	return AuthMethodNone
}

func (this *SshCredentials) keyEnabled() bool {
	return this.KeyPath != ""
}

func (this *HttpCredentials) enabled() bool {
	return this.Username != "" && this.Password != ""
}

/////////////////////////////////////////////////
// Credentials merge
/////////////////////////////////////////////////

func (this *ConfigSingle) mergeCredentials(credentials map[string]Credentials) {
	for targetId, creds := range credentials {
		target, ok := this.Targets[targetId]
		if ok {
			target.merge(&creds)
			this.Targets[targetId] = target
		} else {
			log.Warn().Str("target", targetId).Msgf(
				"credentials specified for target '%s' but the target is not found in the configuration",
				targetId,
			)
		}
	}
}

func (this *Config) mergeCredentials(credentials map[string]Credentials) {
	for repoId, creds := range credentials {
		repo, ok := this.Repositories[repoId]
		if ok {
			repo.merge(&creds)
			this.Repositories[repoId] = repo
		} else {
			log.Warn().Str("repo", repoId).Msgf(
				"credentials specified for repository '%s' but the repository is not found in the configuration",
				repoId,
			)
		}
	}
}

func (this *Credentials) merge(other *Credentials) {
	overrideStr(&this.HttpToken, other.HttpToken)
	overrideStr(&this.HttpCredentials.Username, other.HttpCredentials.Username)
	overrideStr(&this.HttpCredentials.Password, other.HttpCredentials.Password)
	overrideBool(&this.SshCredentials.UseAgent, other.SshCredentials.UseAgent)
	overrideStr(&this.SshCredentials.Username, other.SshCredentials.Username)
	overrideStr(&this.SshCredentials.KeyPath, other.SshCredentials.KeyPath)
	overrideStr(&this.SshCredentials.KeyPassword, other.SshCredentials.KeyPassword)
	overrideBool(&this.SshCredentials.IgnoreHostKey, other.SshCredentials.IgnoreHostKey)
}

/////////////////////////////////////////////////
// Environment variable substitution
/////////////////////////////////////////////////

func (this *ConfigSingle) resolveEnvVars(envVars map[string]string) {
	var err error
	this.Path, err = envsubst.Replace(this.Path, envVars)
	if err != nil {
		logEnvVarSubstWarning(err, "", "localPath")
	}

	for k, target := range this.Targets {
		target.resolveEnvVars(k, envVars)
		this.Targets[k] = target
	}
}

func (this *Config) resolveEnvVars(envVars map[string]string) {
	for k, repo := range this.Repositories {
		repo.resolveEnvVars(k, envVars)
		this.Repositories[k] = repo
	}
}

func (this *Repository) resolveEnvVars(parent string, envVars map[string]string) {
	var err error
	this.URL, err = envsubst.Replace(this.URL, envVars)
	if err != nil {
		logEnvVarSubstWarning(err, parent, "url")
	}
	this.LocalPath, err = envsubst.Replace(this.LocalPath, envVars)
	if err != nil {
		logEnvVarSubstWarning(err, parent, "localPath")
	}
	this.HttpToken, err = envsubst.Replace(this.HttpToken, envVars)
	if err != nil {
		logEnvVarSubstWarning(err, parent, "httpToken")
	}
	this.HttpCredentials.resolveEnvVars(parent, envVars)
	this.SshCredentials.resolveEnvVars(parent, envVars)
}

func (this *HttpCredentials) resolveEnvVars(parent string, envVars map[string]string) {
	var err error
	this.Username, err = envsubst.Replace(this.Username, envVars)
	if err != nil {
		logEnvVarSubstWarning(err, parent, "username")
	}
	this.Password, err = envsubst.Replace(this.Password, envVars)
	if err != nil {
		logEnvVarSubstWarning(err, parent, "password")
	}
}

func (this *SshCredentials) resolveEnvVars(parent string, envVars map[string]string) {
	var err error
	this.Username, err = envsubst.Replace(this.Username, envVars)
	if err != nil {
		logEnvVarSubstWarning(err, parent, "username")
	}
	this.KeyPassword, err = envsubst.Replace(this.KeyPassword, envVars)
	if err != nil {
		logEnvVarSubstWarning(err, parent, "keyPassword")
	}
	this.KeyPath, err = envsubst.Replace(this.KeyPath, envVars)
	if err != nil {
		logEnvVarSubstWarning(err, parent, "keyPath")
	}
}

func logEnvVarSubstWarning(err error, field ...string) {
	fieldCompiled := strings.Join(field, ".")
	log.Warn().Str("field", fieldCompiled).Err(err).Msg(envVarSubstErrorMsg)
}

/////////////////////////////////////////////////
// Validation
/////////////////////////////////////////////////

func (this *ConfigSingle) validate(v *validation.V) {
	v.FailWhen(
		len(this.Targets) == 0,
		"targets",
		"at least one target must be specified",
	)

	reposV := v.Sub("targets")
	for targetId, target := range this.Targets {
		targetV := reposV.Sub(targetId)
		target.validate(targetV)
	}
}

func (this *Target) validate(v *validation.V) {
	this.Repository.validate(v)
	this.SyncSpec.validate(v)
}

func (this *SyncSpec) validate(v *validation.V) {
	v.FailWhen(
		this.Interval.Nanoseconds() < 0,
		"interval",
		"must not be negative",
	)
	v.FailWhen(
		len(this.Branches) == 0 && len(this.Tags) == 0,
		"branches/tags",
		"at least one branch or tag spec must be specified",
	)

	branchV := v.Sub("branches")
	for i, branch := range this.Branches {
		branchV.IndexFailFWhen(branch.IsEmpty(), i, "matcher must not be empty")
	}

	tagV := v.Sub("tags")
	for i, tag := range this.Tags {
		tagV.IndexFailFWhen(tag.IsEmpty(), i, "matcher must not be empty")
	}
}

func (this *Config) validate(v *validation.V) {
	v.FailWhen(
		len(this.Repositories) == 0,
		"repositories",
		"at least one repository must be specified",
	)
	v.FailWhen(
		len(this.Mappings) == 0,
		"mappings",
		"at least one mapping must be specified",
	)

	reposV := v.Sub("repositories")
	for repoId, repo := range this.Repositories {
		repoV := reposV.Sub(repoId)
		repo.validate(repoV)
	}

	mappingsV := v.Sub("mappings")
	for i, mapping := range this.Mappings {
		mappingV := mappingsV.IndexedSub(i)
		mapping.validate(mappingV)

		if _, ok := this.Repositories[mapping.Source]; !ok {
			mappingV.FailF("source", "source %s is not specified", mapping.Source)
		}

		targetsV := mappingV.Sub("targets")
		for i, target := range mapping.Targets {
			if _, ok := this.Repositories[target]; !ok {
				targetsV.IndexFailF(i, "target %s is not specified", target)
			}
		}
	}
}

func (this *SyncMapping) validate(v *validation.V) {
	v.FailWhen(
		this.Source == "",
		"source",
		"source cannot be an empty string",
	)
	v.FailWhen(
		len(this.Targets) == 0,
		"targets",
		"at least one target must be defined",
	)

	targetsV := v.Sub("targets")
	for i, target := range this.Targets {
		if target == "" {
			targetsV.IndexFail(i, "target cannot be an empty string")
		}
	}
	this.SyncSpec.validate(v)
}

func (this *Repository) validate(v *validation.V) {
	v.FailFWhen(
		this.URL == "" && this.LocalPath == "",
		"url",
		"both repository URL and local path cannot be empty",
	)

	var authV *validation.V
	switch this.TargetAuthMethod {
	case AuthMethodUndefined, AuthMethodNone:
		break
	case AuthMethodHttpToken:
		v.FailWhen(
			this.HttpToken == "",
			"httpToken",
			"expected HTTP token to be set",
		)
	case AuthMethodHttpCredentials:
		authV = v.Sub("httpCredentials")
		authV.FailWhen(
			this.HttpCredentials.Username == "",
			"username",
			"expected HTTP username to be set",
		)
		authV.FailWhen(
			this.HttpCredentials.Password == "",
			"password",
			"expected HTTP password to be set",
		)
	case AuthMethodSshAgent:
		authV = v.Sub("sshCredentials")
		authV.FailWhen(
			!this.SshCredentials.UseAgent,
			"useAgent",
			"expected SSH agent to be enabled",
		)
	case AuthMethodSshKey:
		authV = v.Sub("sshCredentials")
		authV.FailWhen(
			this.SshCredentials.KeyPath == "",
			"keyPath",
			"expected SSH key path to be set",
		)
	default:
		v.FailF("authMethod", "unexpected auth method %s", this.TargetAuthMethod)
	}
}

/////////////////////////////////////////////////
// Parsing
/////////////////////////////////////////////////

func (this *Config) Parse(
	envVars envvar.Vars,
	config io.Reader,
	credentials io.Reader,
) error {
	var temp struct{
		ConfigSingle
		Config
	}

	// Read config stream (JSON)
	if err := json.NewDecoder(config).Decode(&temp); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Full config not specified, so we assume there's a single config
	if len(temp.Repositories) == 0 && len(temp.Mappings) == 0 {
		if err := temp.ConfigSingle.parse(envVars, credentials); err != nil {
			return err
		}
		this.fromSingle(&temp.ConfigSingle)
		return nil
	}

	// Parse full config
	if err := temp.Config.parse(envVars, credentials); err != nil {
		return err
	}
	*this = temp.Config
	return nil
}

func (this *ConfigSingle) parse(
	envVars envvar.Vars,
	credentials io.Reader,
) error {
	// When credentials stream is defined, read credentials (JSON) and
	// merge them to the config.
	parsedCreds, err := parseCredentials(credentials)
	if err != nil {
		return err
	}
	this.mergeCredentials(parsedCreds)

	// Resolve any environment variables used in strings
	this.resolveEnvVars(envVars.ToMap())

	// Validate the config
	var v validation.V
	v.Init()
	this.validate(&v)
	return v.ToError()
}

func (this *Config) parse(
	envVars envvar.Vars,
	credentials io.Reader,
) error {
	// When credentials stream is defined, read credentials (JSON) and
	// merge them to the config.
	parsedCreds, err := parseCredentials(credentials)
	if err != nil {
		return err
	}
	this.mergeCredentials(parsedCreds)

	// Resolve any environment variables used in strings
	this.resolveEnvVars(envVars.ToMap())

	// Validate the config
	var v validation.V
	v.Init()
	this.validate(&v)
	return v.ToError()
}

func parseCredentials(credentials io.Reader) (map[string]Credentials, error) {
	if credentials == nil {
		return nil, nil
	}
	var parsedCreds map[string]Credentials
	if err := json.NewDecoder(credentials).Decode(&parsedCreds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}
	return parsedCreds, nil
}

func (this *Config) fromSingle(cfg *ConfigSingle) {
	sourceKey := "source"
	this.Repositories = make(map[string]Repository, len(cfg.Targets) + 1)
	this.Repositories[sourceKey] = Repository{
		Credentials: Credentials{
			TargetAuthMethod: AuthMethodNone,
		},
		LocalPath: cfg.Path,
		URL: "",
		InMemory: false,
	}

	for targetId, target := range cfg.Targets {
		this.Repositories[targetId] = target.Repository
		this.Mappings = append(this.Mappings, SyncMapping{
			Source: sourceKey,
			Targets: []string{targetId},
			SyncSpec: target.SyncSpec,
		})
	}
}
