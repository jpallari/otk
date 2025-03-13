package gitsync

import (
	"bufio"
	"fmt"
	"io"

	"go.lepovirta.org/otk/internal/file"
	"go.lepovirta.org/otk/internal/gitsync/config"
	"go.lepovirta.org/otk/internal/osenv"
)

func parseConfig(
	osEnv osenv.OsEnv,
	cliFlags *config.CliFlags,
	cfg *config.Config,
) error {
	if cliFlags.ConfigPath == config.StdinPath {
		return cfg.Parse(
			osEnv.EnvVars,
			osEnv.Stdin,
			nil,
		)
	}

	var fileReader file.Reader
	var configReader io.Reader
	var credentialsReader io.Reader

	fileReader.Init(osEnv.Fs, 2)

	if cliFlags.ConfigPath != "" {
		file, err := fileReader.Open(cliFlags.ConfigPath)
		if err != nil {
			_ = fileReader.Close()
			return fmt.Errorf(
				"failed to open config in path '%s': %w",
				cliFlags.ConfigPath,
				err,
			)
		}
		configReader = bufio.NewReader(file)
	} else {
		panic("unexpected config empty path")
	}

	if cliFlags.CredentialsPath == config.StdinPath {
		credentialsReader = osEnv.Stdin
	} else if cliFlags.CredentialsPath != "" {
		file, err := fileReader.Open(cliFlags.CredentialsPath)
		if err != nil {
			_ = fileReader.Close()
			return fmt.Errorf(
				"failed to open credentials in path '%s': %w",
				cliFlags.CredentialsPath,
				err,
			)
		}
		credentialsReader = bufio.NewReader(file)
	}

	if err := cfg.Parse(
		osEnv.EnvVars,
		configReader,
		credentialsReader,
	); err != nil {
		_ = fileReader.Close()
		return err
	}

	return fileReader.Close()
}
