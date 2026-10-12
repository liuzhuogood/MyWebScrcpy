package pibundle

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"
)

func fixture(t *testing.T) fstest.MapFS {
	t.Helper()
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	config := manifest{Version: "test", GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Files: make(map[string]fileRecord)}
	for _, name := range []string{"pi" + executableSuffix(), "bin/rg" + executableSuffix(), "bin/fd" + executableSuffix(), "package.json"} {
		mode := os.FileMode(0755)
		if name == "package.json" {
			mode = 0644
		}
		content := []byte("fixture:" + name)
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(mode)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatal(err)
		}
		config.Files[name] = fileRecord{SHA256: hashString(content), Mode: mode}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	config.SHA256 = hashString(data.Bytes())
	manifestJSON, _ := json.Marshal(config)
	return fstest.MapFS{"payload/runtime.zip": {Data: data.Bytes()}, "payload/manifest.json": {Data: manifestJSON}}
}

func hashString(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func TestExtractionReuseAndSkillEdits(t *testing.T) {
	root := filepath.Join(t.TempDir(), "portable app 中文")
	skills := fstest.MapFS{"review/SKILL.md": {Data: []byte("bundled skill")}, "review/references/rules.md": {Data: []byte("reference")}}
	source := fixture(t)
	paths, err := prepare(root, skills, source)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(paths.Executable) || paths.SkillsDir != filepath.Join(root, ".pi", "skills") {
		t.Fatal(paths)
	}
	info, err := os.Stat(paths.Executable)
	if err != nil {
		t.Fatal(err)
	}
	skillPath := filepath.Join(paths.SkillsDir, "review", "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("user edit"), 0644); err != nil {
		t.Fatal(err)
	}
	customPath := filepath.Join(paths.SkillsDir, "custom.md")
	if err := os.WriteFile(customPath, []byte("custom"), 0644); err != nil {
		t.Fatal(err)
	}
	// A valid installation is reused without even opening the embedded archive.
	delete(source, "payload/runtime.zip")
	if _, err := prepare(root, skills, source); err != nil {
		t.Fatal(err)
	}
	current, _ := os.Stat(paths.Executable)
	if !info.ModTime().Equal(current.ModTime()) {
		t.Fatal("runtime unnecessarily replaced")
	}
	for name, expected := range map[string]string{skillPath: "user edit", customPath: "custom"} {
		actual, err := os.ReadFile(name)
		if err != nil || string(actual) != expected {
			t.Fatalf("existing skill changed: %s %s %v", name, actual, err)
		}
	}
	// A damaged managed runtime is rebuilt; edited skills remain unchanged.
	if err := os.WriteFile(paths.Executable, []byte("damaged"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := prepare(root, skills, fixture(t)); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(paths.Executable)
	if string(content) != "fixture:pi"+executableSuffix() {
		t.Fatalf("runtime not repaired: %s", content)
	}
	backups, _ := filepath.Glob(filepath.Dir(paths.Executable) + ".invalid-*")
	if len(backups) != 1 {
		t.Fatalf("damaged runtime was not preserved: %v", backups)
	}
}

func TestRejectInvalidPayload(t *testing.T) {
	if _, err := prepare(t.TempDir(), fstest.MapFS{}, fstest.MapFS{}); !errors.Is(err, ErrUnprepared) {
		t.Fatal(err)
	}
	for _, test := range []string{"platform", "archive", "file", "traversal"} {
		t.Run(test, func(t *testing.T) {
			source := fixture(t)
			var config manifest
			_ = json.Unmarshal(source["payload/manifest.json"].Data, &config)
			switch test {
			case "platform":
				config.GOOS = "unsupported"
			case "archive":
				source["payload/runtime.zip"].Data = []byte("broken")
			case "file":
				config.Files["package.json"] = fileRecord{SHA256: "bad", Mode: 0644}
			case "traversal":
				config.Files["../outside"] = fileRecord{SHA256: "bad", Mode: 0644}
			}
			source["payload/manifest.json"].Data, _ = json.Marshal(config)
			if _, err := prepare(t.TempDir(), fstest.MapFS{}, source); err == nil {
				t.Fatal("accepted invalid bundle")
			}
		})
	}
}
