package config

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AuthMethod specifies which authentication method is used
// when connecting to the Git repository.
type AuthMethod int

const (
	// AuthMethodUndefined means that the authentication method is unknown.
	// Authentication method is automatically determined based on
	// the specified credentials.
	AuthMethodUndefined AuthMethod = iota

	// AuthMethodNone means that no authentication is used.
	// Commonly used for local repositories.
	AuthMethodNone

	// AuthMethodHttpToken means that HTTP token is used for authentication.
	AuthMethodHttpToken

	// AuthMethodHttpCredentials means that HTTP basic auth is used for authentication.
	AuthMethodHttpCredentials

	// AuthMethodSshAgent means that SSH agent is used for acquiring SSH credentials.
	AuthMethodSshAgent

	// AuthMethodSshKey means that SSH keys are used for authentication.
	AuthMethodSshKey
)

func (a AuthMethod) MarshalJSON() ([]byte, error) {
	var s string
	switch a {
	case AuthMethodUndefined:
		return json.Marshal(nil)
	case AuthMethodNone:
		s = "none"
	case AuthMethodHttpToken:
		s = "http-token"
	case AuthMethodHttpCredentials:
		s = "http"
	case AuthMethodSshAgent:
		s = "ssh-agent"
	case AuthMethodSshKey:
		s = "ssh"
	default:
		return nil, fmt.Errorf("unknown auth method '%s'", a)
	}
	return json.Marshal(s)
}

func (a *AuthMethod) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch strings.ToLower(v) {
	case "", "undefined":
		*a = AuthMethodUndefined
	case "none", "disabled":
		*a = AuthMethodNone
	case "http-token":
		*a = AuthMethodHttpToken
	case "http", "http-basic":
		*a = AuthMethodHttpCredentials
	case "ssh-agent":
		*a = AuthMethodSshAgent
	case "ssh", "ssh-key":
		*a = AuthMethodSshKey
	default:
		return fmt.Errorf("unexpected value '%s' for auth method", v)
	}
	return nil
}

func (a AuthMethod) String() string {
	switch a {
	case AuthMethodUndefined:
		return ""
	case AuthMethodNone:
		return "none"
	case AuthMethodHttpToken:
		return "http-token"
	case AuthMethodHttpCredentials:
		return "http"
	case AuthMethodSshAgent:
		return "ssh-agent"
	case AuthMethodSshKey:
		return "ssh"
	default:
		return fmt.Sprintf("unknown(%d)", a)
	}
}
