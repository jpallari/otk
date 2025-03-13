package internal

import (
	"flag"
	"fmt"
	"os"
)

const appName = "git-sync"

type Flags struct {
	Run             bool
	Once            bool
	ConfigPath      string
	CredentialsPath string
}

func (this *Flags) ConfigFromStdin() bool {
	return this.ConfigPath == "-"
}

func (this *Flags) CredentialsFromStdin() bool {
	return this.CredentialsPath == "-"
}

func (this *Flags) validate() error {
	if this.ConfigPath == "" {
		return fmt.Errorf("Config path is not defined.")
	}
	return nil
}

func (this *Flags) FromArgs(args []string) error {
	var flagSet flag.FlagSet
	flagSet.Init(appName, flag.ExitOnError)
	flagSet.Usage = func() {
		fmt.Fprintf(flagSet.Output(), "Usage:\n")
		flagSet.PrintDefaults()
	}

	flagSet.BoolVar(
		&this.Run,
		"run",
		false,
		"Run the Git sync. If not enabled, a dry-run will be executed instead.",
	)
	flagSet.BoolVar(
		&this.Once,
		"once",
		false,
		"Run Git sync only once instead of the repeatedly as specified in the configuration.",
	)
	flagSet.StringVar(
		&this.ConfigPath,
		"config",
		"",
		"Path to config. Use '-' to read from STDIN.",
	)
	flagSet.StringVar(
		&this.CredentialsPath,
		"credentials",
		"",
		"Path to credentials file. Use '-' to read from STDIN.",
	)

	// error handling not necessary because exit on error is enabled
	_ = flagSet.Parse(args)

	// Fall back to env vars
	if this.ConfigPath == "" {
		this.ConfigPath = os.Getenv("GIT_SYNC_CONFIG")
	}
	if this.CredentialsPath == "" {
		this.CredentialsPath = os.Getenv("GIT_SYNC_CREDENTIALS")
	}

	return this.validate()
}
