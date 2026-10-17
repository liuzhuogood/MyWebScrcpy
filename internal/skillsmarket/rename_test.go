package skillsmarket

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPublicationPreservesExistingTargets(t *testing.T) {
	for _, kind := range []string{"file", "empty-directory", "nonempty-directory"} {
		t.Run(kind, func(t *testing.T) {
			home := t.TempDir()
			if err := os.Mkdir(filepath.Join(home, "stage"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, "stage", "SKILL.md"), []byte("new skill"), 0600); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(home, "target")
			if kind == "file" {
				if err := os.WriteFile(target, []byte("user file"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Mkdir(target, 0700); err != nil {
					t.Fatal(err)
				}
				if kind == "nonempty-directory" {
					if err := os.WriteFile(filepath.Join(target, "user.txt"), []byte("user file"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			before, err := os.Lstat(target)
			if err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(home)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			if err := renameInDirNoReplace(root, ".", "stage", "target"); err == nil {
				t.Fatal("publication replaced an existing target")
			}
			after, err := os.Lstat(target)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("existing target changed: %v", err)
			}
			b, err := os.ReadFile(filepath.Join(home, "stage", "SKILL.md"))
			if err != nil || string(b) != "new skill" {
				t.Fatalf("stage changed: %v", err)
			}
			if kind == "file" {
				b, err = os.ReadFile(target)
			} else if kind == "nonempty-directory" {
				b, err = os.ReadFile(filepath.Join(target, "user.txt"))
			} else {
				return
			}
			if err != nil || string(b) != "user file" {
				t.Fatalf("user contents changed: %v", err)
			}
		})
	}
}

func TestPublicationCreatesMissingTarget(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skip("platform has no atomic publication implementation")
	}
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, "stage"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "stage", "SKILL.md"), []byte("complete skill"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(home)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := renameInDirNoReplace(root, ".", "stage", "target"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(home, "target", "SKILL.md"))
	if err != nil || string(b) != "complete skill" {
		t.Fatalf("published contents: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "stage")); !os.IsNotExist(err) {
		t.Fatalf("stage still exists: %v", err)
	}
}
