package v2

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/inkly/CasaOS-Common/utils/file"
)

var defaultDirectoryPaths = []string{
	"AppData",
	"Documents",
	"Downloads",
	"Gallery",
	filepath.Join("Media", "Movies"),
	filepath.Join("Media", "TV Shows"),
	filepath.Join("Media", "Music"),
}

// EnsureDefaultDirectories creates the standard CasaOS data directories below
// root if they do not already exist.
func EnsureDefaultDirectories(root string) error {
	if root == "" {
		return errors.New("default directory root is empty")
	}

	var errs []error
	for _, relativePath := range defaultDirectoryPaths {
		path := filepath.Join(root, relativePath)
		if err := ensureDefaultDirectory(path); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
		}
	}
	return errors.Join(errs...)
}

func ensureDefaultDirectory(path string) error {
	if err := file.IsNotExistMkDir(path); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory")
	}
	return nil
}
