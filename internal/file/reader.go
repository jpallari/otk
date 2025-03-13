package file

import (
	"errors"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
)

type Reader struct {
	fs          billy.Filesystem
	fileHandles []billy.File
}

func (this *Reader) Init(fs billy.Filesystem, fileCount int) {
	this.fs = fs
	this.fileHandles = make([]billy.File, 0, fileCount)
}

func (this *Reader) Open(filename string) (billy.File, error) {
	file, err := this.fs.Open(filepath.Clean(filename))
	if err != nil {
		return file, err
	}
	this.fileHandles = append(this.fileHandles, file)
	return file, nil
}

func (this *Reader) Close() error {
	errs := make([]error, 0, len(this.fileHandles))
	for _, handle := range this.fileHandles {
		err := handle.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
