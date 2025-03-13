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

type m struct {
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

func (this *M) Clear() {
	this.pattern = nil
	this.spec = ""
}

func (this *M) MarshalJSON() ([]byte, error) {
	return json.Marshal(this.String())
}

func (this *M) UnmarshalJSON(b []byte) error {
	{
		var v string
		err := json.Unmarshal(b, &v)
		if err == nil {
			return this.FromString(v)
		}
		switch err.(type) {
		case *json.UnmarshalTypeError:
		default:
			return err
		}
	}

	var v m
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	this.spec = v.Spec
	if v.UseRegex {
		return this.FromPattern(v.Spec)
	}
	return nil
}

func (this *M) FromString(s string) error {
	if strings.HasPrefix(s, "/") && strings.HasSuffix(s, "/") {
		this.spec = s[1 : len(s)-1]
		return this.FromPattern(this.spec)
	}
	this.spec = s
	return nil
}

func (this *M) FromPattern(s string) (err error) {
	this.pattern, err = regexp.Compile(s)
	return
}

func (this *M) String() string {
	if this.UsesRegex() {
		return fmt.Sprintf("/%s/", this.spec)
	}
	return this.spec
}

func (this *M) IsEmpty() bool {
	return this.pattern == nil && this.spec == ""
}

func (this *M) UsesRegex() bool {
	return this.pattern != nil
}

func (this *M) MatchString(s string) bool {
	if this.IsEmpty() {
		return true
	}
	if this.UsesRegex() {
		return this.pattern.MatchString(s)
	}
	return this.spec == s
}
