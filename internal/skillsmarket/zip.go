package skillsmarket

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"time"
)

// Manifest represents package integrity and ownership metadata.
type Manifest struct {
	ManagedBy string            `json:"managedBy"`
	SkillID   string            `json:"skillId"`
	Version   string            `json:"version"`
	Files     map[string]string `json:"files"`
}

var deterministicModTime = time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)

// getSkillFiles reads all files for the skill from embedded fs and computes sha256.
func getSkillFiles(skillID SkillID) (map[string][]byte, *Manifest, error) {
	if !IsKnownSkillID(string(skillID)) {
		return nil, nil, fmt.Errorf("unknown skill: %s", skillID)
	}

	rootPath := fmt.Sprintf("assets/skills/%s", skillID)
	files := make(map[string][]byte)
	fileChecksums := make(map[string]string)

	err := fs.WalkDir(skillsFS, rootPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepathRel(rootPath, path)
		if err != nil {
			return err
		}

		data, err := skillsFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}

		hash := sha256.Sum256(data)
		checksum := hex.EncodeToString(hash[:])

		files[rel] = data
		fileChecksums[rel] = checksum
		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	manifest := &Manifest{
		ManagedBy: "mywebscrcpy",
		SkillID:   string(skillID),
		Version:   "1.0.0",
		Files:     fileChecksums,
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("marshal manifest: %w", err)
	}
	files[".mywebscrcpy-manifest.json"] = manifestBytes

	return files, manifest, nil
}

// GenerateSkillZip builds a standard ZIP package for the given skill.
func GenerateSkillZip(skillID SkillID) ([]byte, error) {
	files, _, err := getSkillFiles(skillID)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	for relPath, data := range files {
		entryName := fmt.Sprintf("%s/%s", skillID, relPath)
		header := &zip.FileHeader{
			Name:     entryName,
			Method:   zip.Deflate,
			Modified: deterministicModTime,
		}
		if relPath == "scripts/request_client.py" {
			header.SetMode(0755)
		} else {
			header.SetMode(0644)
		}

		entryWriter, err := w.CreateHeader(header)
		if err != nil {
			_ = w.Close()
			return nil, fmt.Errorf("create zip entry %s: %w", entryName, err)
		}

		if _, err := io.Copy(entryWriter, bytes.NewReader(data)); err != nil {
			_ = w.Close()
			return nil, fmt.Errorf("write zip entry %s: %w", entryName, err)
		}
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

func filepathRel(base, target string) (string, error) {
	if base == target {
		return ".", nil
	}
	prefix := base + "/"
	if len(target) > len(prefix) && target[:len(prefix)] == prefix {
		return target[len(prefix):], nil
	}
	return target, nil
}
