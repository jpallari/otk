package envsubst

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	envSubstRe = regexp.MustCompile("\\$?\\$\\{([^}]+)\\}")
)

func Replace(text string, vars map[string]string) (string, error) {
	unknownKeys := make(map[string]bool)
	s := envSubstRe.ReplaceAllStringFunc(text, func(match string) string {
		// replace escaped characters
		if strings.HasPrefix(match, "$$") {
			return match[1:]
		}

		// remove surrounding ${} characters
		key := strings.TrimSpace(match[2 : len(match)-1])

		v, ok := vars[key]
		if !ok {
			unknownKeys[key] = true
		}
		return v
	})

	if len(unknownKeys) == 0 {
		return s, nil
	}

	unknownKeysSlice := make([]string, 0, len(unknownKeys))
	for key := range unknownKeys {
		unknownKeysSlice = append(unknownKeysSlice, key)
	}

	err := &KeyError{
		keys: unknownKeysSlice,
	}
	return s, err
}

type KeyError struct {
	keys []string
}

func (this *KeyError) MissingKeys() []string {
	return this.keys
}

func (this *KeyError) Error() string {
	keysStr := strings.Join(this.keys, ", ")
	return fmt.Sprintf("no value found for keys: %s", keysStr)
}
