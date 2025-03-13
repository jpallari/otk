package gitsynctesting

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
)

func currentDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filename)
}

func copyFileFromOsFs(
	fs billy.Filesystem,
	sourcePath string,
	targetPath string,
) error {
	fileBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read file from path '%s': %w", sourcePath, err)
	}

	if err := util.WriteFile(fs, targetPath, fileBytes, 0600); err != nil {
		return fmt.Errorf("failed to write file to path '%s': %w", targetPath, err)
	}

	return nil
}
