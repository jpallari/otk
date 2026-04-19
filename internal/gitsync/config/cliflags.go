package config

import (
	"flag"
	"fmt"
	"io"

	"go.lepovirta.org/otk/internal/envvar"
)

const StdinPath = "-"

type CliFlags struct {
	Run             bool
	Once            bool
	ConfigPath      string
	CredentialsPath string
}

func (f *CliFlags) validate() error {
	if f.ConfigPath == "" {
		return fmt.Errorf("config path not specified")
	}
	if f.ConfigPath == StdinPath && f.CredentialsPath == StdinPath {
		return fmt.Errorf("loading config and credentials from STDIN at the same time is not supported")
	}
	return nil
}

func (f *CliFlags) Parse(
	envVars envvar.Vars,
	args []string,
	output io.Writer,
) error {
	var flagSet flag.FlagSet
	flagSet.Init(AppName, flag.ContinueOnError)
	flagSet.SetOutput(output)
	flagSet.Usage = func() {
		_, _ = fmt.Fprintf(
			flagSet.Output(),
			"Usage: %s [-config <path>] [-credentials <path>] [-once] [-run] [-h | --help]\n\nOptions:\n",
			args[0],
		)
		flagSet.PrintDefaults()
	}

	flagSet.BoolVar(
		&f.Run,
		"run",
		false,
		"Run the Git sync. If not enabled, a dry run will be executed instead.",
	)
	flagSet.BoolVar(
		&f.Once,
		"once",
		false,
		"Run Git sync only once instead of the repeatedly as specified in the configuration.",
	)
	flagSet.StringVar(
		&f.ConfigPath,
		"config",
		"-",
		"Path to a configuration file. Use '-' to read from STDIN. By default, config is read from STDIN.",
	)
	flagSet.StringVar(
		&f.CredentialsPath,
		"credentials",
		"",
		"Path to a credentials file. Use '-' to read from STDIN.",
	)

	if err := flagSet.Parse(args[1:]); err != nil {
		return err
	}

	// Fall back to env vars
	if f.ConfigPath == "" {
		f.ConfigPath = envVars.GetForApp(AppName, "CONFIG_PATH")
	}
	if f.CredentialsPath == "" {
		f.CredentialsPath = envVars.GetForApp(AppName, "CREDENTIALS")
	}

	return f.validate()
}
