package mdscript

import (
	"strings"
	"testing"
)

func TestExecutionOptionsHeader(t *testing.T) {
	for _, content := range []string{"version: 1\nrecording:\n  enabled: true", "# 正文\n```yaml\nrecording:\n  enabled: true\n```"} {
		o, e := ParseOptions(content)
		if e != nil || o.Recording.Enabled || o.Report.Enabled {
			t.Fatalf("body interpreted: %+v %v", o, e)
		}
	}
	o, e := ParseOptions("---\nversion: 1\nrecording:\n  enabled: true\n  record_audio: false\nreport:\n  enabled: true\n  prompt: |\n    第一句\n    第二句\n---\n正文")
	if e != nil || !o.Recording.Enabled || o.Recording.RecordAudio || !o.Report.Enabled || !strings.Contains(o.Report.Prompt, "第二句") {
		t.Fatalf("%+v %v", o, e)
	}
	for _, header := range []string{"recording: true", "recording:\n  enabled: yes", "recording:\n  enabled: \"true\"", "recording:\n  audio: true", "report:\n  prompt: 3", "report:\n  enabled: true\n  enabled: false", "recording: &r\n  enabled: true\nreport: *r", "report:\n  prompt: " + strings.Repeat("中", 4001)} {
		if _, err := ParseOptions("---\n" + header + "\n---"); err == nil {
			t.Fatalf("accepted %s", header)
		}
	}
}
