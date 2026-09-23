package appdata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesSelfManagedLayout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "mygpt-test-data")
	paths, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if paths.Root != root {
		t.Fatalf("root = %q, want %q", paths.Root, root)
	}
	for _, dir := range []string{paths.Uploads, paths.Metadata, paths.Database, paths.Config, paths.Temp} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("directory %q was not created: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("path %q is not a directory", dir)
		}
	}
}
