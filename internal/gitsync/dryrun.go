package gitsync

import (
	"fmt"
	"io"
	"strings"

	"go.lepovirta.org/otk/internal/gitsync/config"
)

const (
	syncHeader    = "sync:"
	syncSubHeader = "     "
)

func dryRun(
	out io.Writer,
	cfg *config.Config,
) error {
	if err := dryRun_(out, cfg); err != nil {
		return fmt.Errorf("failed to write dry run info: %w", err)
	}
	return nil
}

func dryRun_(
	out io.Writer,
	cfg *config.Config,
) (err error) {
	_, err = fmt.Fprintln(out, "!! DRY RUN !! Use flag -run to sync the following Git repos")
	if err != nil {
		return
	}
	for _, m := range cfg.Mappings {
		sourceRepo := cfg.Repositories[m.Source]
		_, err = fmt.Fprintf(
			out, "\n%s %s --> %s\n",
			syncHeader, m.Source,
			strings.Join(m.Targets, ", "),
		)
		if err != nil {
			return
		}
		_, err = fmt.Fprintf(
			out, "%s %s = %s (auth: %s)\n",
			syncSubHeader,
			m.Source, sourceRepo.URL,
			authMethodString(sourceRepo.AuthMethod()),
		)
		if err != nil {
			return
		}
		for _, target := range m.Targets {
			repo := cfg.Repositories[target]
			_, err = fmt.Fprintf(
				out, "%s %s = %s (auth: %s)\n",
				syncSubHeader,
				target, repo.URL,
				authMethodString(repo.AuthMethod()),
			)
			if err != nil {
				return
			}
		}
		if len(m.Branches) > 0 {
			branches := make([]string, 0, len(m.Branches))
			for _, branch := range m.Branches {
				branches = append(branches, branch.String())
			}
			_, err = fmt.Fprintf(
				out, "%s branches = %s\n",
				syncSubHeader,
				strings.Join(branches, ","),
			)
			if err != nil {
				return
			}
		}
		if len(m.Tags) > 0 {
			tags := make([]string, 0, len(m.Tags))
			for _, tag := range m.Tags {
				tags = append(tags, tag.String())
			}
			_, err = fmt.Fprintf(
				out, "%s tags = %s\n",
				syncSubHeader,
				strings.Join(tags, ","),
			)
			if err != nil {
				return
			}
		}
	}
	return
}

func authMethodString(authMethod config.AuthMethod) string {
	s := authMethod.String()
	if s == "" {
		return "auto"
	}
	return s
}
