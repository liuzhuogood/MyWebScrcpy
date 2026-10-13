package mdscript

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mywebscrcpy/internal/aiprovider"
	"mywebscrcpy/internal/db"
)

var generatedFixture = func() string {
	value, err := os.ReadFile("../../.pi/skills/markdown-to-python/assets/example.py")
	if err != nil {
		panic(err)
	}
	return string(value)
}()

type modelFixture struct {
	calls   int
	respond func(context.Context, aiprovider.GenerationInput, int) (string, error)
}

func (m *modelFixture) GetConfig() (aiprovider.ModelConfig, error) {
	return aiprovider.ModelConfig{Provider: "custom", Model: "fixture"}, nil
}
func (m *modelFixture) StreamGeneration(ctx context.Context, cfg aiprovider.ModelConfig, input aiprovider.GenerationInput, emit func(aiprovider.ChatEvent) error) error {
	m.calls++
	reply, err := m.respond(ctx, input, m.calls)
	if err != nil {
		return err
	}
	return emit(aiprovider.ChatEvent{Type: "done", Reply: reply})
}
func ready(code string) string {
	value, _ := json.Marshal(map[string]any{"status": "ready", "python": code, "issues": []any{}})
	return string(value)
}
func generationFixture(t *testing.T) (*db.DB, *Generator, *db.MarkdownScriptRecord, *modelFixture) {
	t.Helper()
	d, _, _ := testExecutor(t)
	skillDir := t.TempDir()
	for _, name := range []string{"SKILL.md", "references/sdk-api.md", "assets/example.py"} {
		data, err := os.ReadFile(filepath.Join("../../.pi/skills/markdown-to-python", name))
		if err != nil {
			t.Fatal(err)
		}
		dest := filepath.Join(skillDir, name)
		os.MkdirAll(filepath.Dir(dest), 0700)
		if err := os.WriteFile(dest, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	model := &modelFixture{respond: func(context.Context, aiprovider.GenerationInput, int) (string, error) {
		return ready(generatedFixture), nil
	}}
	apiDocument, err := os.ReadFile("../../web/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	g := NewGenerator(d, model, skillDir, apiDocument)
	script, err := d.CreateMarkdownScript("设备查询", []string{"device-A"}, "version: 1\n- 用 ADB 查询设备型号并打印", "1", "", 0, 30)
	if err != nil {
		t.Fatal(err)
	}
	return d, g, script, model
}
func ignoreGeneration(GenerationEvent) error { return nil }

func TestGenerationFreezeRetryAndProvenance(t *testing.T) {
	d, g, script, model := generationFixture(t)
	model.respond = func(ctx context.Context, input aiprovider.GenerationInput, attempt int) (string, error) {
		if !strings.Contains(input.Files["current.md"], "ADB") || !strings.Contains(input.SkillBody, "markdown-to-python") {
			t.Fatal("missing frozen context")
		}
		if attempt == 1 {
			return ready(strings.Replace(generatedFixture, "device.adb(", "device.fake(", 1)), nil
		}
		if input.Files["previous.json"] == "" {
			t.Fatal("missing correction context")
		}
		return ready(generatedFixture), nil
	}
	var complete bool
	if err := g.Generate(context.Background(), *script, func(event GenerationEvent) error {
		if event.Type == "done" {
			complete = event.Script != nil && event.Generation != nil
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if model.calls != 2 || !complete {
		t.Fatalf("calls=%d completed=%v", model.calls, complete)
	}
	saved, _ := d.GetMarkdownScript(script.ID)
	if err := g.Check(*saved); err != nil {
		t.Fatal(err)
	}
	provenance, _ := d.GetScriptGeneration(script.ID)
	if provenance.Provider != "custom" || provenance.Model != "fixture" || provenance.CodeHash == "" {
		t.Fatal(provenance)
	}
	apiBefore := g.APIDocument
	g.APIDocument = `{"openapi":"3.1.0","paths":{}}`
	if g.Check(*saved) == nil {
		t.Fatal("changed API document accepted")
	}
	g.APIDocument = apiBefore
	if err := os.WriteFile(filepath.Join(g.SkillDir, "references/sdk-api.md"), []byte("# SDK API sdk-v1\nChanged"), 0600); err != nil {
		t.Fatal(err)
	}
	if g.Check(*saved) == nil {
		t.Fatal("modified skill artifact accepted")
	}
}

func TestGenerationConcurrentEditAndMissingInformation(t *testing.T) {
	d, g, script, model := generationFixture(t)
	model.respond = func(context.Context, aiprovider.GenerationInput, int) (string, error) {
		if err := d.UpdateMarkdownScript(script.ID, script.Name, []string{"device-B"}, script.Content, "1", "", 0, 30); err != nil {
			t.Fatal(err)
		}
		return ready(generatedFixture), nil
	}
	if err := g.Generate(context.Background(), *script, ignoreGeneration); !errors.Is(err, db.ErrGenerationStale) {
		t.Fatal(err)
	}
	saved, _ := d.GetMarkdownScript(script.ID)
	if saved.CompiledPython != "" {
		t.Fatal("stale result saved")
	}
	model.calls = 0
	model.respond = func(context.Context, aiprovider.GenerationInput, int) (string, error) {
		return `{"status":"needs_input","python":"","issues":[{"step":"步骤 1","message":"请提供点击坐标"}]}`, nil
	}
	if err := g.Generate(context.Background(), *saved, ignoreGeneration); err == nil || !strings.Contains(err.Error(), "点击坐标") {
		t.Fatal(err)
	}
	if model.calls != 1 {
		t.Fatal("needs_input automatically retried")
	}
}

func TestGenerationFailureAndCancellationKeepLastArtifact(t *testing.T) {
	d, g, script, model := generationFixture(t)
	if err := g.Generate(context.Background(), *script, ignoreGeneration); err != nil {
		t.Fatal(err)
	}
	saved, _ := d.GetMarkdownScript(script.ID)
	model.calls = 0
	model.respond = func(context.Context, aiprovider.GenerationInput, int) (string, error) { return "not JSON", nil }
	if g.Generate(context.Background(), *saved, ignoreGeneration) == nil || model.calls != 3 {
		t.Fatal("retry bound missing")
	}
	after, _ := d.GetMarkdownScript(script.ID)
	if after.CompiledPython != saved.CompiledPython || g.Check(*after) != nil {
		t.Fatal("previous artifact lost")
	}
	ctx, cancel := context.WithCancel(context.Background())
	model.respond = func(context.Context, aiprovider.GenerationInput, int) (string, error) {
		cancel()
		return ready(generatedFixture), nil
	}
	if g.Generate(ctx, *saved, ignoreGeneration) == nil {
		t.Fatal("cancellation accepted")
	}
	after, _ = d.GetMarkdownScript(script.ID)
	if after.CompiledAt != saved.CompiledAt {
		t.Fatal("cancelled result saved")
	}
	release, err := g.begin(script.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.begin(script.ID); err == nil {
		t.Fatal("duplicate request accepted")
	}
	release()
}

func TestExecutionRejectsHistoricalAndExpiredArtifacts(t *testing.T) {
	d, g, script, _ := generationFixture(t)
	d.SaveCompiledPython(script.ID, "1", "print('legacy')")
	saved, _ := d.GetMarkdownScript(script.ID)
	if g.Check(*saved) == nil {
		t.Fatal("legacy artifact accepted")
	}
	if err := g.Generate(context.Background(), *saved, ignoreGeneration); err != nil {
		t.Fatal(err)
	}
	d.UpdateMarkdownScript(script.ID, script.Name, []string{"device-B"}, script.Content, "1", "", 0, 30)
	changed, _ := d.GetMarkdownScript(script.ID)
	if g.Check(*changed) == nil {
		t.Fatal("changed targets accepted")
	}
	_, mgr, _ := testExecutor(t)
	executor := NewExecutor(d, mgr, func(string) (bool, bool) { return true, true }, nil)
	executor.ArtifactCheck = g.Check
	if _, err := executor.Execute(script.ID); err == nil {
		t.Fatal("stale execution accepted")
	}
}

func TestGenerationProtocol(t *testing.T) {
	for _, reply := range []string{`{}`, `{"status":"ready","status":"ready","python":"x","issues":[]}`, `{"status":"needs_input","python":"","issues":[{"message":"missing step"}]}`, `{"status":"ready","python":"x","issues":null}`, `{"status":"ready","python":"","issues":[]}`, `{"status":"ready","python":"x","issues":[],"extra":1}`, `{"status":"needs_input","python":"x","issues":[]}`, ready(generatedFixture) + " {}"} {
		if _, err := parseGeneration(reply); err == nil {
			t.Fatalf("accepted %s", reply)
		}
	}
}
