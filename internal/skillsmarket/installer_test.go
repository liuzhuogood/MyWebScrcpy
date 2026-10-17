package skillsmarket

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestSkillZipArchiveContentAndStructure(t *testing.T) {
	for _, skillID := range []SkillID{SkillDeviceBase, SkillKuaishouDaily, SkillWechatHelper} {
		zipBytes, err := GenerateSkillZip(skillID)
		if err != nil {
			t.Fatalf("GenerateSkillZip(%s) error: %v", skillID, err)
		}

		r, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
		if err != nil {
			t.Fatalf("read zip for %s error: %v", skillID, err)
		}

		foundRoot := false
		foundSkillMD := false
		foundRequestClient := false
		foundManifest := false

		expectedPrefix := string(skillID) + "/"

		for _, f := range r.File {
			if !strings.HasPrefix(f.Name, expectedPrefix) {
				t.Fatalf("file %s in zip does not start with expected prefix %s", f.Name, expectedPrefix)
			}
			relName := strings.TrimPrefix(f.Name, expectedPrefix)
			if relName == "SKILL.md" {
				foundSkillMD = true
				rc, err := f.Open()
				if err != nil {
					t.Fatal(err)
				}
				content, _ := io.ReadAll(rc)
				rc.Close()
				str := string(content)
				if !strings.Contains(str, "name: "+string(skillID)) {
					t.Fatalf("SKILL.md missing name frontmatter: %s", str)
				}
			}
			if relName == "scripts/request_client.py" {
				foundRequestClient = true
			}
			if relName == ".mywebscrcpy-manifest.json" {
				foundManifest = true
				rc, err := f.Open()
				if err != nil {
					t.Fatal(err)
				}
				content, _ := io.ReadAll(rc)
				rc.Close()
				var m Manifest
				if err := json.Unmarshal(content, &m); err != nil {
					t.Fatalf("invalid manifest in zip: %v", err)
				}
				if m.SkillID != string(skillID) || m.ManagedBy != "mywebscrcpy" {
					t.Fatalf("unexpected manifest values: %+v", m)
				}
			}
			foundRoot = true
		}

		if !foundRoot || !foundSkillMD || !foundRequestClient || !foundManifest {
			t.Fatalf("zip for %s missing essential files (root=%v, skillMD=%v, client=%v, manifest=%v)",
				skillID, foundRoot, foundSkillMD, foundRequestClient, foundManifest)
		}
	}
}

func TestInstallSingleTargetAndReconstructManager(t *testing.T) {
	tempHome := t.TempDir()
	mgr, err := NewManager(tempHome)
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}

	// 1. Initial status is not_installed
	st := mgr.CheckSkillTargetStatus(ToolCodex, SkillDeviceBase)
	if st.Installed || st.Status != "not_installed" {
		t.Fatalf("expected not_installed, got %+v", st)
	}

	// 2. Install to Codex
	res, err := mgr.InstallSkill(context.Background(), SkillDeviceBase, []AiToolID{ToolCodex})
	if err != nil {
		t.Fatalf("InstallSkill error: %v", err)
	}
	if len(res) != 1 || res[0].Status != "installed" {
		t.Fatalf("unexpected result: %+v", res)
	}

	expectedSkillFile := filepath.Join(tempHome, ".agents", "skills", string(SkillDeviceBase), "SKILL.md")
	data, err := os.ReadFile(expectedSkillFile)
	if err != nil {
		t.Fatalf("expected installed file %s not found: %v", expectedSkillFile, err)
	}
	if !strings.Contains(string(data), string(SkillDeviceBase)) {
		t.Fatalf("unexpected file content: %s", string(data))
	}

	// 3. Status is now installed
	st = mgr.CheckSkillTargetStatus(ToolCodex, SkillDeviceBase)
	if !st.Installed || st.Status != "installed" {
		t.Fatalf("expected installed, got %+v", st)
	}

	// 4. Reconstruct manager from filesystem
	mgr2, err := NewManager(tempHome)
	if err != nil {
		t.Fatal(err)
	}
	st2 := mgr2.CheckSkillTargetStatus(ToolCodex, SkillDeviceBase)
	if !st2.Installed || st2.Status != "installed" {
		t.Fatalf("reconstructed manager should read installed from filesystem, got %+v", st2)
	}

	// 5. Re-install returns already_installed
	res2, err := mgr2.InstallSkill(context.Background(), SkillDeviceBase, []AiToolID{ToolCodex})
	if err != nil {
		t.Fatal(err)
	}
	if len(res2) != 1 || res2[0].Status != "already_installed" {
		t.Fatalf("expected already_installed, got %+v", res2)
	}
}

func TestInstallMultiTargetBatch(t *testing.T) {
	tempHome := t.TempDir()
	mgr, err := NewManager(tempHome)
	if err != nil {
		t.Fatal(err)
	}

	targets := []AiToolID{ToolCodex, ToolClaudeCode, ToolWorkbuddy}
	res, err := mgr.InstallSkill(context.Background(), SkillKuaishouDaily, targets)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 3 {
		t.Fatalf("expected 3 results, got %d", len(res))
	}
	for _, r := range res {
		if r.Status != "installed" {
			t.Fatalf("target %s failed: %+v", r.Target, r)
		}
		// verify on disk
		checkFile := filepath.Join(r.Path, "SKILL.md")
		if _, err := os.Stat(checkFile); err != nil {
			t.Fatalf("file not found at %s: %v", checkFile, err)
		}
	}
}

func TestExistingUserDirectoryAndModificationsPreserved(t *testing.T) {
	tempHome := t.TempDir()
	mgr, err := NewManager(tempHome)
	if err != nil {
		t.Fatal(err)
	}

	// Case A: User has their own pre-existing directory without our manifest
	userDir := filepath.Join(tempHome, ".claude", "skills", string(SkillWechatHelper))
	if err := os.MkdirAll(userDir, 0755); err != nil {
		t.Fatal(err)
	}
	userFile := filepath.Join(userDir, "custom.txt")
	userContent := "user custom private content"
	if err := os.WriteFile(userFile, []byte(userContent), 0644); err != nil {
		t.Fatal(err)
	}

	st := mgr.CheckSkillTargetStatus(ToolClaudeCode, SkillWechatHelper)
	if st.Installed || st.Status != "conflict" {
		t.Fatalf("expected conflict for user dir, got %+v", st)
	}

	res, err := mgr.InstallSkill(context.Background(), SkillWechatHelper, []AiToolID{ToolClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Status != "failed" {
		t.Fatalf("expected failed installation for conflict dir, got %+v", res)
	}

	// Verify user file is completely untouched
	persisted, err := os.ReadFile(userFile)
	if err != nil {
		t.Fatalf("user file was deleted! %v", err)
	}
	if string(persisted) != userContent {
		t.Fatalf("user file was modified! %s", string(persisted))
	}

	// Case B: Installed by app, then modified by user
	resB, err := mgr.InstallSkill(context.Background(), SkillDeviceBase, []AiToolID{ToolCodex})
	if err != nil || len(resB) != 1 || resB[0].Status != "installed" {
		t.Fatalf("setup install failed: %v", err)
	}
	installedMD := filepath.Join(tempHome, ".agents", "skills", string(SkillDeviceBase), "SKILL.md")
	if err := os.WriteFile(installedMD, []byte("user modified this skill"), 0644); err != nil {
		t.Fatal(err)
	}

	stB := mgr.CheckSkillTargetStatus(ToolCodex, SkillDeviceBase)
	if stB.Installed || stB.Status != "conflict" {
		t.Fatalf("expected conflict after user modified file, got %+v", stB)
	}
	resB2, err := mgr.InstallSkill(context.Background(), SkillDeviceBase, []AiToolID{ToolCodex})
	if err != nil || len(resB2) != 1 || resB2[0].Status != "failed" {
		t.Fatalf("expected failed install on modified skill, got %+v", resB2)
	}

	// Case C: Installed by app, then user adds an unknown file
	resC, err := mgr.InstallSkill(context.Background(), SkillKuaishouDaily, []AiToolID{ToolWorkbuddy})
	if err != nil || len(resC) != 1 || resC[0].Status != "installed" {
		t.Fatalf("setup install failed: %v", err)
	}
	extraFile := filepath.Join(tempHome, ".workbuddy", "skills", string(SkillKuaishouDaily), "extra.py")
	if err := os.WriteFile(extraFile, []byte("extra file"), 0644); err != nil {
		t.Fatal(err)
	}
	stC := mgr.CheckSkillTargetStatus(ToolWorkbuddy, SkillKuaishouDaily)
	if stC.Installed || stC.Status != "conflict" {
		t.Fatalf("expected conflict after extra file added, got %+v", stC)
	}
}

func TestSymlinkRejected(t *testing.T) {
	tempHome := t.TempDir()
	mgr, err := NewManager(tempHome)
	if err != nil {
		t.Fatal(err)
	}

	// Create symlink pointing elsewhere
	outsideDir := t.TempDir()
	claudeSkillsDir := filepath.Join(tempHome, ".claude", "skills")
	if err := os.MkdirAll(claudeSkillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	linkTarget := filepath.Join(claudeSkillsDir, string(SkillDeviceBase))
	if err := os.Symlink(outsideDir, linkTarget); err != nil {
		t.Skip("symlinks not supported on this filesystem")
	}

	st := mgr.CheckSkillTargetStatus(ToolClaudeCode, SkillDeviceBase)
	if st.Installed || st.Status != "conflict" {
		t.Fatalf("expected conflict for symlink, got %+v", st)
	}

	res, err := mgr.InstallSkill(context.Background(), SkillDeviceBase, []AiToolID{ToolClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Status != "failed" {
		t.Fatalf("expected failed install on symlink, got %+v", res)
	}
}

func TestPartialFailureDoesNotAffectSuccess(t *testing.T) {
	tempHome := t.TempDir()
	mgr, err := NewManager(tempHome)
	if err != nil {
		t.Fatal(err)
	}

	// Codex is clean, ClaudeCode has user conflict
	claudeConflictDir := filepath.Join(tempHome, ".claude", "skills", string(SkillDeviceBase))
	if err := os.MkdirAll(claudeConflictDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeConflictDir, "user.txt"), []byte("mine"), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := mgr.InstallSkill(context.Background(), SkillDeviceBase, []AiToolID{ToolCodex, ToolClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}

	// Codex must be installed
	if res[0].Target != ToolCodex || res[0].Status != "installed" {
		t.Fatalf("codex should succeed, got %+v", res[0])
	}
	// ClaudeCode must be failed
	if res[1].Target != ToolClaudeCode || res[1].Status != "failed" {
		t.Fatalf("claude-code should fail, got %+v", res[1])
	}

	// Verify Codex is actually installed on disk
	codexSkillFile := filepath.Join(tempHome, ".agents", "skills", string(SkillDeviceBase), "SKILL.md")
	if _, err := os.Stat(codexSkillFile); err != nil {
		t.Fatalf("codex install file missing: %v", err)
	}
	// Verify ClaudeCode user file untouched
	if _, err := os.Stat(filepath.Join(claudeConflictDir, "user.txt")); err != nil {
		t.Fatalf("claude conflict file missing: %v", err)
	}
}

func TestInstallValidation(t *testing.T) {
	tempHome := t.TempDir()
	mgr, err := NewManager(tempHome)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Empty targets
	if _, err := mgr.InstallSkill(context.Background(), SkillDeviceBase, []AiToolID{}); err == nil {
		t.Fatal("expected error for empty targets")
	}

	// 2. Duplicate targets
	if _, err := mgr.InstallSkill(context.Background(), SkillDeviceBase, []AiToolID{ToolCodex, ToolCodex}); err == nil {
		t.Fatal("expected error for duplicate targets")
	}

	// 3. Unknown target
	if _, err := mgr.InstallSkill(context.Background(), SkillDeviceBase, []AiToolID{"unknown"}); err == nil {
		t.Fatal("expected error for unknown target")
	}

	// 4. Unknown skill ID
	if _, err := mgr.InstallSkill(context.Background(), "unknown-skill", []AiToolID{ToolCodex}); err == nil {
		t.Fatal("expected error for unknown skill")
	}
}

func TestInstallConcurrencyAndContext(t *testing.T) {
	tempHome := t.TempDir()
	mgr, err := NewManager(tempHome)
	if err != nil {
		t.Fatal(err)
	}

	// Cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err := mgr.InstallSkill(ctx, SkillDeviceBase, []AiToolID{ToolCodex})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Status != "failed" {
		t.Fatalf("expected failed result for cancelled context, got %+v", res)
	}

	// Concurrent installs of different skills to different targets
	var wg sync.WaitGroup
	skills := []SkillID{SkillDeviceBase, SkillKuaishouDaily, SkillWechatHelper}
	for _, sk := range skills {
		wg.Add(1)
		go func(s SkillID) {
			defer wg.Done()
			_, _ = mgr.InstallSkill(context.Background(), s, []AiToolID{ToolWorkbuddy})
		}(sk)
	}
	wg.Wait()
}

func TestParentSymlinkRejected(t *testing.T) {
	outsideDir := t.TempDir()
	outsideMgr, err := NewManager(outsideDir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := outsideMgr.InstallSkill(context.Background(), SkillDeviceBase, []AiToolID{ToolCodex})
	if err != nil || len(res) == 0 || res[0].Status != "installed" {
		t.Fatalf("setup install failed: %v, %v", res, err)
	}

	testHome := t.TempDir()
	mgr, err := NewManager(testHome)
	if err != nil {
		t.Fatal(err)
	}

	// Create symlink ~/.agents -> outsideDir/.agents
	linkPath := filepath.Join(testHome, ".agents")
	targetPath := filepath.Join(outsideDir, ".agents")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatal(err)
	}

	// Status check must reject symlinked parent
	st := mgr.CheckSkillTargetStatus(ToolCodex, SkillDeviceBase)
	if st.Installed || st.Status == "installed" {
		t.Fatalf("expected symlinked parent to be rejected, got: %+v", st)
	}

	// Install must fail and not report already_installed or installed
	installRes, err := mgr.InstallSkill(context.Background(), SkillDeviceBase, []AiToolID{ToolCodex})
	if err != nil {
		t.Fatalf("InstallSkill error: %v", err)
	}
	if len(installRes) == 0 || installRes[0].Status != "failed" {
		t.Fatalf("expected install to fail on symlinked parent, got: %+v", installRes)
	}
}

func TestLargeFileBoundedRead(t *testing.T) {
	testHome := t.TempDir()
	mgr, err := NewManager(testHome)
	if err != nil {
		t.Fatal(err)
	}

	res, err := mgr.InstallSkill(context.Background(), SkillDeviceBase, []AiToolID{ToolCodex})
	if err != nil || len(res) == 0 || res[0].Status != "installed" {
		t.Fatalf("setup install failed: %v, %v", res, err)
	}

	p := filepath.Join(res[0].Path, "SKILL.md")
	// Inflate to 32MiB
	if err := os.Truncate(p, 32<<20); err != nil {
		t.Fatal(err)
	}

	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	st := mgr.CheckSkillTargetStatus(ToolCodex, SkillDeviceBase)
	runtime.ReadMemStats(&after)

	if st.Installed {
		t.Fatalf("expected truncated file to be detected as conflict, got %+v", st)
	}

	allocBytes := after.TotalAlloc - before.TotalAlloc
	if allocBytes >= 16<<20 {
		t.Fatalf("excessive memory allocation during status check: %d bytes", allocBytes)
	}
}

func TestCancellationDuringStaging(t *testing.T) {
	testHome := t.TempDir()
	mgr, err := NewManager(testHome)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before/during operation

	res, err := mgr.InstallSkill(ctx, SkillDeviceBase, []AiToolID{ToolCodex, ToolClaudeCode})
	if err != nil {
		t.Fatalf("expected nil error with cancelled results, got %v", err)
	}
	for _, r := range res {
		if r.Status != "failed" || !strings.Contains(r.Error, "取消") {
			t.Fatalf("expected cancelled failed status, got: %+v", r)
		}
	}

	// Verify nothing was committed
	st := mgr.CheckSkillTargetStatus(ToolCodex, SkillDeviceBase)
	if st.Installed {
		t.Fatal("skill was committed despite cancellation")
	}
}

func TestExtractedZipPythonClientExecutable(t *testing.T) {
	tempDir := t.TempDir()
	for _, skillID := range []SkillID{SkillDeviceBase, SkillKuaishouDaily, SkillWechatHelper} {
		zipBytes, err := GenerateSkillZip(skillID)
		if err != nil {
			t.Fatal(err)
		}
		r, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
		if err != nil {
			t.Fatal(err)
		}
		skillFolder := filepath.Join(tempDir, string(skillID))
		for _, f := range r.File {
			p := filepath.Join(tempDir, f.Name)
			if f.FileInfo().IsDir() {
				_ = os.MkdirAll(p, 0755)
				continue
			}
			_ = os.MkdirAll(filepath.Dir(p), 0755)
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, _ := io.ReadAll(rc)
			rc.Close()
			_ = os.WriteFile(p, data, f.Mode())
		}

		cmd := exec.Command("python3", "scripts/request_client.py", "--help")
		cmd.Dir = skillFolder
		// Windows 控制台默认 cp1252 代码页无法编码中文帮助与错误信息，固定该编码验证脚本自身统一 UTF-8 输出。
		cmd.Env = append(os.Environ(), "PYTHONIOENCODING=cp1252")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("execute request_client.py in %s: %v, out=%s", skillID, err, string(out))
		}
		if !strings.Contains(string(out), "Generic HTTP transport") {
			t.Fatalf("unexpected output for %s: %s", skillID, string(out))
		}
	}
}
