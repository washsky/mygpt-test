package appdata

import (
	"fmt"
	"os"
	"path/filepath"
)

type Paths struct {
	Root     string
	Files    string
	Uploads  string
	Metadata string
	Database string
	Config   string
	Temp     string
}

// Open creates the persistent data layout beside the executable by default.
// An explicit directory or MYGPT_DATA_DIR can override the default.
func Open(override string) (Paths, error) {
	root := override
	if root == "" {
		root = os.Getenv("MYGPT_DATA_DIR")
	}
	if root == "" {
		executable, err := os.Executable()
		if err != nil {
			return Paths{}, fmt.Errorf("locate executable: %w", err)
		}
		absolute, err := filepath.Abs(executable)
		if err != nil {
			return Paths{}, fmt.Errorf("resolve executable path: %w", err)
		}
		root = filepath.Join(filepath.Dir(absolute), "mygpt-test-data")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return Paths{}, fmt.Errorf("resolve data directory: %w", err)
	}

	paths := Paths{
		Root:     root,
		Files:    filepath.Join(root, "files"),
		Uploads:  filepath.Join(root, "files", "uploads"),
		Metadata: filepath.Join(root, "files", "metadata"),
		Database: filepath.Join(root, "database"),
		Config:   filepath.Join(root, "config"),
		Temp:     filepath.Join(root, "tmp"),
	}
	for _, dir := range []string{paths.Uploads, paths.Metadata, paths.Database, paths.Config, paths.Temp} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return Paths{}, fmt.Errorf("create data directory %s: %w", dir, err)
		}
	}
	return paths, nil
}
