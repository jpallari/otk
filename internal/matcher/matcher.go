package matcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var ErrUnexpectedType = errors.New("unexpected type for matcher")

// M provides a string matcher that is based on a raw string match
// or a regular expression match.
type M struct {
	pattern *regexp.Regexp
	spec    string
}

type mSpec struct {
	Spec     string `json:"spec"`
	UseRegex bool   `json:"useRegex"`
}

func Empty() M {
	return M{}
}

func FromString(s string) (matcher M, err error) {
	err = matcher.FromString(s)
	return
}

func FromStringOrPanic(s string) (matcher M) {
	if err := matcher.FromString(s); err != nil {
		panic(err)
	}
	return
}

func (m *M) Clear() {
	m.pattern = nil
	m.spec = ""
}

func (m *M) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.String())
}

func (m *M) UnmarshalJSON(b []byte) error {
	{
		var v string
		err := json.Unmarshal(b, &v)
		if err == nil {
			return m.FromString(v)
		}
		switch err.(type) {
		case *json.UnmarshalTypeError:
		default:
			return err
		}
	}

	var v mSpec
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	m.spec = v.Spec
	if v.UseRegex {
		return m.FromPattern(v.Spec)
	}
	return nil
}

func (m *M) FromString(s string) error {
	if strings.HasPrefix(s, "/") && strings.HasSuffix(s, "/") {
		m.spec = s[1 : len(s)-1]
		return m.FromPattern(m.spec)
	}
	m.spec = s
	return nil
}

func (m *M) FromPattern(s string) (err error) {
	m.pattern, err = regexp.Compile(s)
	return
}

func (m *M) String() string {
	if m.UsesRegex() {
		return fmt.Sprintf("/%s/", m.spec)
	}
	return m.spec
}

func (m *M) IsEmpty() bool {
	return m.pattern == nil && m.spec == ""
}

func (m *M) UsesRegex() bool {
	return m.pattern != nil
}

func (m *M) MatchString(s string) bool {
	if m.IsEmpty() {
		return true
	}
	if m.UsesRegex() {
		return m.pattern.MatchString(s)
	}
	return m.spec == s
}
