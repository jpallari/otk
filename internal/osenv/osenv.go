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

func (this *OsEnv) FromRealEnv() {
	this.Args = os.Args
	this.Fs = osfs.New("")
	this.EnvVars.FromEnv()
	this.Stdin = os.Stdin
	this.Stdout = os.Stdout
	this.Stderr = os.Stderr
	this.HttpTransport = http.DefaultTransport
}
