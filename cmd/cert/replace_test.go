// SPDX-License-Identifier: MIT

package cert

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReplaceFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.key")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "new" {
		t.Errorf("replaced file %q, %v", data, err)
	}
	if fi, err := os.Stat(path); runtime.GOOS != "windows" && (err != nil || fi.Mode().Perm() != 0o600) {
		t.Errorf("mode %v, %v; want 0600 whatever the old file had", fi.Mode().Perm(), err)
	}
	// No directory to create the file in.
	if err := replaceFile(filepath.Join(dir, "missing", "a.key"), []byte("x")); err == nil {
		t.Error("replaceFile in a missing directory succeeded")
	}
	// A directory at the path: the rename fails, and no temporary file stays.
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "x"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(sub, []byte("x")); err == nil {
		t.Error("replaceFile over a non-empty directory succeeded")
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "a.key" && e.Name() != "sub" {
			t.Errorf("left %s behind", e.Name())
		}
	}
}
