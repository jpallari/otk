package envvar

import (
	"fmt"
	"os"
	"strings"
)

type Vars struct {
	vars []string
}

func (this *Vars) FromSlice(slice []string) error {
	for i, v := range slice {
		if !strings.Contains(v, "=") {
			return &FormatError{index: i, value: v}
		}
	}

	this.vars = slice
	return nil
}

func (this *Vars) FromMap(m map[string]string) {
	this.vars = make([]string, 0, len(m))
	for k, v := range m {
		kv := fmt.Sprintf("%s=%s", k, v)
		this.vars = append(this.vars, kv)
	}
}

func (this *Vars) ToMap() map[string]string {
	m := make(map[string]string, len(this.vars))
	for _, kv := range this.vars {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) < 2 {
			panic(fmt.Sprintf("not enough parts in env var '%s'", kv))
		}
		m[parts[0]] = parts[1]
	}
	return m
}

func (this *Vars) FromEnv() {
	this.vars = os.Environ()
}

func (this *Vars) All() []string {
	return this.vars
}

func (this *Vars) Lookup(key string) (string, bool) {
	key = key + "="
	for _, entry := range this.vars {
		if strings.HasPrefix(entry, key) {
			v := entry[len(key):]
			return v, true
		}
	}
	return "", false
}

func (this *Vars) LookupForApp(appName, varName string) (string, bool) {
	return this.Lookup(AppKey(appName, varName))
}

func (this *Vars) Get(key string) string {
	v, _ := this.Lookup(key)
	return v
}

func (this *Vars) GetOr(key string, alternative string) string {
	v, ok := this.Lookup(key)
	if ok {
		return v
	}
	return alternative
}

func (this *Vars) GetForApp(appName, varName string) string {
	return this.Get(AppKey(appName, varName))
}

func (this *Vars) GetForAppOr(appName, varName string, alternative string) string {
	return this.GetOr(AppKey(appName, varName), alternative)
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

func (this *FormatError) Index() int {
	return this.index
}

func (this *FormatError) Value() string {
	return this.value
}

func (this *FormatError) Error() string {
	return fmt.Sprintf(
		"invalid var format at index %d: %s",
		this.index, this.value,
	)
}
