package envvar

import (
	"fmt"
	"os"
	"strings"
)

type Vars struct {
	vars []string
}

func (vs *Vars) FromSlice(slice []string) error {
	for i, v := range slice {
		if !strings.Contains(v, "=") {
			return &FormatError{index: i, value: v}
		}
	}

	vs.vars = slice
	return nil
}

func (vs *Vars) FromMap(m map[string]string) {
	vs.vars = make([]string, 0, len(m))
	for k, v := range m {
		kv := fmt.Sprintf("%s=%s", k, v)
		vs.vars = append(vs.vars, kv)
	}
}

func (vs *Vars) ToMap() map[string]string {
	m := make(map[string]string, len(vs.vars))
	for _, kv := range vs.vars {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) < 2 {
			panic(fmt.Sprintf("not enough parts in env var '%s'", kv))
		}
		m[parts[0]] = parts[1]
	}
	return m
}

func (vs *Vars) FromEnv() {
	vs.vars = os.Environ()
}

func (vs *Vars) All() []string {
	return vs.vars
}

func (vs *Vars) Lookup(key string) (string, bool) {
	key = key + "="
	for _, entry := range vs.vars {
		if strings.HasPrefix(entry, key) {
			v := entry[len(key):]
			return v, true
		}
	}
	return "", false
}

func (vs *Vars) LookupForApp(appName, varName string) (string, bool) {
	return vs.Lookup(AppKey(appName, varName))
}

func (vs *Vars) Get(key string) string {
	v, _ := vs.Lookup(key)
	return v
}

func (vs *Vars) GetOr(key string, alternative string) string {
	v, ok := vs.Lookup(key)
	if ok {
		return v
	}
	return alternative
}

func (vs *Vars) GetForApp(appName, varName string) string {
	return vs.Get(AppKey(appName, varName))
}

func (vs *Vars) GetForAppOr(appName, varName string, alternative string) string {
	return vs.GetOr(AppKey(appName, varName), alternative)
}

func AppKey(appName, varName string) string {
	{
		appNameFiltered := strings.TrimSpace(appName)
		if appNameFiltered != appName {
			panic("whitespace in app name is not allowed")
		}
		if appNameFiltered == "" {
			panic("empty app name is not allowed")
		}
	}
	prefix := appName
	prefix = strings.ReplaceAll(prefix, "-", "_")
	prefix = strings.ReplaceAll(prefix, " ", "_")
	prefix = strings.ToUpper(prefix)
	return fmt.Sprintf("%s_%s", prefix, varName)
}

type FormatError struct {
	index int
	value string
}

func (e *FormatError) Index() int {
	return e.index
}

func (e *FormatError) Value() string {
	return e.value
}

func (e *FormatError) Error() string {
	return fmt.Sprintf(
		"invalid var format at index %d: %s",
		e.index, e.value,
	)
}
