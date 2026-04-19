package osenv

import (
	"io"
	"net/http"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"go.lepovirta.org/otk/internal/envvar"
)

type OsEnv struct {
	Args          []string
	Fs            billy.Filesystem
	EnvVars       envvar.Vars
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
	HttpTransport http.RoundTripper
}

func (e *OsEnv) FromRealEnv() {
	e.Args = os.Args
	e.Fs = osfs.New("")
	e.EnvVars.FromEnv()
	e.Stdin = os.Stdin
	e.Stdout = os.Stdout
	e.Stderr = os.Stderr
	e.HttpTransport = http.DefaultTransport
}
