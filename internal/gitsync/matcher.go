package internal

import (
	"fmt"
	"regexp"
	"slices"

	"github.com/jpallari/otk/internal/git-sync/config"
)

type matcher struct {
	explicit []string
	patterns []*regexp.Regexp
}

func (this *matcher) Init(specs []config.TargetSpec) error {
	this.explicit = make([]string, 0, len(specs))
	this.patterns = make([]*regexp.Regexp, 0, len(specs))
	for _, spec := range specs {
		if spec.UseRegex {
			pattern, err := regexp.Compile(spec.Spec)
			if err != nil {
				return fmt.Errorf("failed to compile pattern '%s': %w", spec.Spec, err)
			}
			this.patterns = append(this.patterns, pattern)
		} else {
			this.explicit = append(this.explicit, spec.Spec)
		}
	}
	return nil
}

func (this *matcher) Match(target string) bool {
	if slices.Contains(this.explicit, target) {
		return true
	}
	for _, pattern := range this.patterns {
		if pattern.MatchString(target) {
			return true
		}
	}
	return false
}
