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

func (r *Reader) Init(fs billy.Filesystem, fileCount int) {
	r.fs = fs
	r.fileHandles = make([]billy.File, 0, fileCount)
}

func (r *Reader) Open(filename string) (billy.File, error) {
	file, err := r.fs.Open(filepath.Clean(filename))
	if err != nil {
		return file, err
	}
	r.fileHandles = append(r.fileHandles, file)
	return file, nil
}

func (r *Reader) Close() error {
	errs := make([]error, 0, len(r.fileHandles))
	for _, handle := range r.fileHandles {
		err := handle.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
