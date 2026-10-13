// Package pythonsdk distributes the same installable SDK used outside the application.
package pythonsdk

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

//go:embed src/mywebscrcpy/*.py
var source embed.FS

func Hash() string {
	paths, _ := fs.Glob(source, "src/mywebscrcpy/*.py")
	sort.Strings(paths)
	sum := sha256.New()
	for _, path := range paths {
		value, _ := source.ReadFile(path)
		sum.Write([]byte(path))
		sum.Write(value)
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// Install provides the embedded SDK beside the script, without invoking pip.
// Third-party dependencies remain in the configured Python environment.
func Install(directory string) error {
	paths, err := fs.Glob(source, "src/mywebscrcpy/*.py")
	if err != nil {
		return err
	}
	target := filepath.Join(directory, "mywebscrcpy")
	if err := os.MkdirAll(target, 0700); err != nil {
		return err
	}
	for _, path := range paths {
		value, err := source.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(target, filepath.Base(path)), value, 0600); err != nil {
			return err
		}
	}
	return nil
}
