package config

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"go.lepovirta.org/otk/internal/duration"
	"go.lepovirta.org/otk/internal/envsubst"
	"go.lepovirta.org/otk/internal/envvar"
	"go.lepovirta.org/otk/internal/matcher"
	"go.lepovirta.org/otk/internal/validation"
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

func (c *Credentials) AuthMethod() AuthMethod {
	if c.TargetAuthMethod != AuthMethodUndefined {
		return c.TargetAuthMethod
	}
	if c.HttpToken != "" {
		return AuthMethodHttpToken
	}
	if c.HttpCredentials.enabled() {
		return AuthMethodHttpCredentials
	}
	if c.SshCredentials.UseAgent {
		return AuthMethodSshAgent
	}
	if c.SshCredentials.keyEnabled() {
		return AuthMethodSshKey
	}
	return AuthMethodNone
}

func (s *SshCredentials) keyEnabled() bool {
	return s.KeyPath != ""
}

func (h *HttpCredentials) enabled() bool {
	return h.Username != "" && h.Password != ""
}

/////////////////////////////////////////////////
// Credentials merge
/////////////////////////////////////////////////

func (cs *ConfigSingle) mergeCredentials(credentials map[string]Credentials) {
	for targetId, creds := range credentials {
		target, ok := cs.Targets[targetId]
		if ok {
			target.merge(&creds)
			cs.Targets[targetId] = target
		} else {
			slog.Warn(
				"credentials specified for target but target not found in configuration",
				slog.String("target", targetId),
			)
		}
	}
}

func (cfg *Config) mergeCredentials(credentials map[string]Credentials) {
	for repoId, creds := range credentials {
		repo, ok := cfg.Repositories[repoId]
		if ok {
			repo.merge(&creds)
			cfg.Repositories[repoId] = repo
		} else {
			slog.Warn(
				"credentials specified for repository but repository not found in configuration",
				slog.String("repo", repoId),
			)
		}
	}
}

func (c *Credentials) merge(other *Credentials) {
	overrideStr(&c.HttpToken, other.HttpToken)
	overrideStr(&c.HttpCredentials.Username, other.HttpCredentials.Username)
	overrideStr(&c.HttpCredentials.Password, other.HttpCredentials.Password)
	overrideBool(&c.SshCredentials.UseAgent, other.SshCredentials.UseAgent)
	overrideStr(&c.SshCredentials.Username, other.SshCredentials.Username)
	overrideStr(&c.SshCredentials.KeyPath, other.SshCredentials.KeyPath)
	overrideStr(&c.SshCredentials.KeyPassword, other.SshCredentials.KeyPassword)
	overrideBool(&c.SshCredentials.IgnoreHostKey, other.SshCredentials.IgnoreHostKey)
}

/////////////////////////////////////////////////
// Environment variable substitution
/////////////////////////////////////////////////

func (cs *ConfigSingle) resolveEnvVars(envVars map[string]string) {
	var err error
	cs.Path, err = envsubst.Replace(cs.Path, envVars)
	if err != nil {
		logEnvVarSubstWarning(err, "", "localPath")
	}

	for k, target := range cs.Targets {
		target.resolveEnvVars(k, envVars)
		cs.Targets[k] = target
	}
}

func (cfg *Config) resolveEnvVars(envVars map[string]string) {
	for k, repo := range cfg.Repositories {
		repo.resolveEnvVars(k, envVars)
		cfg.Repositories[k] = repo
	}
}

func (r *Repository) resolveEnvVars(parent string, envVars map[string]string) {
	var err error
	r.URL, err = envsubst.Replace(r.URL, envVars)
	if err != nil {
		logEnvVarSubstWarning(err, parent, "url")
	}
	r.LocalPath, err = envsubst.Replace(r.LocalPath, envVars)
	if err != nil {
		logEnvVarSubstWarning(err, parent, "localPath")
	}
	r.HttpToken, err = envsubst.Replace(r.HttpToken, envVars)
	if err != nil {
		logEnvVarSubstWarning(err, parent, "httpToken")
	}
	r.HttpCredentials.resolveEnvVars(parent, envVars)
	r.SshCredentials.resolveEnvVars(parent, envVars)
}

func (h *HttpCredentials) resolveEnvVars(parent string, envVars map[string]string) {
	var err error
	h.Username, err = envsubst.Replace(h.Username, envVars)
	if err != nil {
		logEnvVarSubstWarning(err, parent, "username")
	}
	h.Password, err = envsubst.Replace(h.Password, envVars)
	if err != nil {
		logEnvVarSubstWarning(err, parent, "password")
	}
}

func (s *SshCredentials) resolveEnvVars(parent string, envVars map[string]string) {
	var err error
	s.Username, err = envsubst.Replace(s.Username, envVars)
	if err != nil {
		logEnvVarSubstWarning(err, parent, "username")
	}
	s.KeyPassword, err = envsubst.Replace(s.KeyPassword, envVars)
	if err != nil {
		logEnvVarSubstWarning(err, parent, "keyPassword")
	}
	s.KeyPath, err = envsubst.Replace(s.KeyPath, envVars)
	if err != nil {
		logEnvVarSubstWarning(err, parent, "keyPath")
	}
}

func logEnvVarSubstWarning(err error, field ...string) {
	fieldCompiled := strings.Join(field, ".")
	slog.Warn(envVarSubstErrorMsg, slog.String("field", fieldCompiled), slog.Any("error", err))
}

/////////////////////////////////////////////////
// Validation
/////////////////////////////////////////////////

func (cs *ConfigSingle) validate(v *validation.V) {
	v.FailWhen(
		len(cs.Targets) == 0,
		"targets",
		"at least one target must be specified",
	)

	reposV := v.Sub("targets")
	for targetId, target := range cs.Targets {
		targetV := reposV.Sub(targetId)
		target.validate(targetV)
	}
}

func (t *Target) validate(v *validation.V) {
	t.Repository.validate(v)
	t.SyncSpec.validate(v)
}

func (ss *SyncSpec) validate(v *validation.V) {
	v.FailWhen(
		ss.Interval.Nanoseconds() < 0,
		"interval",
		"must not be negative",
	)
	v.FailWhen(
		len(ss.Branches) == 0 && len(ss.Tags) == 0,
		"branches/tags",
		"at least one branch or tag spec must be specified",
	)

	branchV := v.Sub("branches")
	for i, branch := range ss.Branches {
		branchV.IndexFailFWhen(branch.IsEmpty(), i, "matcher must not be empty")
	}

	tagV := v.Sub("tags")
	for i, tag := range ss.Tags {
		tagV.IndexFailFWhen(tag.IsEmpty(), i, "matcher must not be empty")
	}
}

func (cfg *Config) validate(v *validation.V) {
	v.FailWhen(
		len(cfg.Repositories) == 0,
		"repositories",
		"at least one repository must be specified",
	)
	v.FailWhen(
		len(cfg.Mappings) == 0,
		"mappings",
		"at least one mapping must be specified",
	)

	reposV := v.Sub("repositories")
	for repoId, repo := range cfg.Repositories {
		repoV := reposV.Sub(repoId)
		repo.validate(repoV)
	}

	mappingsV := v.Sub("mappings")
	for i, mapping := range cfg.Mappings {
		mappingV := mappingsV.IndexedSub(i)
		mapping.validate(mappingV)

		if _, ok := cfg.Repositories[mapping.Source]; !ok {
			mappingV.FailF("source", "source %s is not specified", mapping.Source)
		}

		targetsV := mappingV.Sub("targets")
		for i, target := range mapping.Targets {
			if _, ok := cfg.Repositories[target]; !ok {
				targetsV.IndexFailF(i, "target %s is not specified", target)
			}
		}
	}
}

func (sm *SyncMapping) validate(v *validation.V) {
	v.FailWhen(
		sm.Source == "",
		"source",
		"source cannot be an empty string",
	)
	v.FailWhen(
		len(sm.Targets) == 0,
		"targets",
		"at least one target must be defined",
	)

	targetsV := v.Sub("targets")
	for i, target := range sm.Targets {
		if target == "" {
			targetsV.IndexFail(i, "target cannot be an empty string")
		}
	}
	sm.SyncSpec.validate(v)
}

func (r *Repository) validate(v *validation.V) {
	v.FailFWhen(
		r.URL == "" && r.LocalPath == "",
		"url",
		"both repository URL and local path cannot be empty",
	)

	var authV *validation.V
	switch r.TargetAuthMethod {
	case AuthMethodUndefined, AuthMethodNone:
		break
	case AuthMethodHttpToken:
		v.FailWhen(
			r.HttpToken == "",
			"httpToken",
			"expected HTTP token to be set",
		)
	case AuthMethodHttpCredentials:
		authV = v.Sub("httpCredentials")
		authV.FailWhen(
			r.HttpCredentials.Username == "",
			"username",
			"expected HTTP username to be set",
		)
		authV.FailWhen(
			r.HttpCredentials.Password == "",
			"password",
			"expected HTTP password to be set",
		)
	case AuthMethodSshAgent:
		authV = v.Sub("sshCredentials")
		authV.FailWhen(
			!r.SshCredentials.UseAgent,
			"useAgent",
			"expected SSH agent to be enabled",
		)
	case AuthMethodSshKey:
		authV = v.Sub("sshCredentials")
		authV.FailWhen(
			r.SshCredentials.KeyPath == "",
			"keyPath",
			"expected SSH key path to be set",
		)
	default:
		v.FailF("authMethod", "unexpected auth method %s", r.TargetAuthMethod)
	}
}

/////////////////////////////////////////////////
// Parsing
/////////////////////////////////////////////////

func (cfg *Config) Parse(
	envVars envvar.Vars,
	config io.Reader,
	credentials io.Reader,
) error {
	var temp struct {
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
		cfg.fromSingle(&temp.ConfigSingle)
		return nil
	}

	// Parse full config
	if err := temp.Config.parse(envVars, credentials); err != nil {
		return err
	}
	*cfg = temp.Config
	return nil
}

func (cs *ConfigSingle) parse(
	envVars envvar.Vars,
	credentials io.Reader,
) error {
	// When credentials stream is defined, read credentials (JSON) and
	// merge them to the config.
	parsedCreds, err := parseCredentials(credentials)
	if err != nil {
		return err
	}
	cs.mergeCredentials(parsedCreds)

	// Resolve any environment variables used in strings
	cs.resolveEnvVars(envVars.ToMap())

	// Validate the config
	var v validation.V
	v.Init()
	cs.validate(&v)
	return v.ToError()
}

func (cfg *Config) parse(
	envVars envvar.Vars,
	credentials io.Reader,
) error {
	// When credentials stream is defined, read credentials (JSON) and
	// merge them to the config.
	parsedCreds, err := parseCredentials(credentials)
	if err != nil {
		return err
	}
	cfg.mergeCredentials(parsedCreds)

	// Resolve any environment variables used in strings
	cfg.resolveEnvVars(envVars.ToMap())

	// Validate the config
	var v validation.V
	v.Init()
	cfg.validate(&v)
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

func (cfg *Config) fromSingle(cs *ConfigSingle) {
	sourceKey := "source"
	cfg.Repositories = make(map[string]Repository, len(cs.Targets)+1)
	cfg.Repositories[sourceKey] = Repository{
		Credentials: Credentials{
			TargetAuthMethod: AuthMethodNone,
		},
		LocalPath: cs.Path,
		URL:       "",
		InMemory:  false,
	}

	for targetId, target := range cs.Targets {
		cfg.Repositories[targetId] = target.Repository
		cfg.Mappings = append(cfg.Mappings, SyncMapping{
			Source:   sourceKey,
			Targets:  []string{targetId},
			SyncSpec: target.SyncSpec,
		})
	}
}
