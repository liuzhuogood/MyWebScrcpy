package desktop

import (
	"os"
	"testing"
)

func TestParseEnvOutput(t *testing.T) {
	cases := []struct {
		name     string
		input    []byte
		wantPath string
		wantKey  string
		wantVal  string
		count    int
	}{
		{
			name:     "valid marker",
			input:    []byte("some banner\n" + envStartMarker + "PATH=/usr/local/bin:/bin\x00FOO=BAR\x00" + envEndMarker + "trailing\n"),
			wantPath: "/usr/local/bin:/bin",
			wantKey:  "FOO",
			wantVal:  "BAR",
			count:    2,
		},
		{
			name:     "missing markers",
			input:    []byte("PATH=/bin\x00"),
			wantPath: "",
			count:    0,
		},
		{
			name:     "empty payload",
			input:    []byte(envStartMarker + envEndMarker),
			wantPath: "",
			count:    0,
		},
		{
			name:     "value with equals sign",
			input:    []byte(envStartMarker + "KEY=A=B=C\x00PATH=/test\x00" + envEndMarker),
			wantPath: "/test",
			wantKey:  "KEY",
			wantVal:  "A=B=C",
			count:    2,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseEnvOutput(c.input)
			if len(got) != c.count {
				t.Fatalf("expected count %d, got %d", c.count, len(got))
			}
			if c.wantPath != "" && got["PATH"] != c.wantPath {
				t.Errorf("expected PATH %q, got %q", c.wantPath, got["PATH"])
			}
			if c.wantKey != "" && got[c.wantKey] != c.wantVal {
				t.Errorf("expected %s=%q, got %q", c.wantKey, c.wantVal, got[c.wantKey])
			}
		})
	}
}

func TestApplyDarwinEnv(t *testing.T) {
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	envMap := map[string]string{
		"PATH":                 "/custom/test/path",
		"ANDROID_HOME":         "/custom/sdk",
		"XPC_SERVICE_NAME":     "ignored_xpc",
		"__CF_USER_TEXT_ENCOD": "ignored_cf",
		"CUSTOM_NEW_VAR":       "custom_val",
	}

	applyDarwinEnv(envMap)

	if got := os.Getenv("PATH"); got != "/custom/test/path" {
		t.Errorf("PATH not updated: got %s", got)
	}
	if got := os.Getenv("ANDROID_HOME"); got != "/custom/sdk" {
		t.Errorf("ANDROID_HOME not updated: got %s", got)
	}
	if got := os.Getenv("CUSTOM_NEW_VAR"); got != "custom_val" {
		t.Errorf("CUSTOM_NEW_VAR not updated: got %s", got)
	}
	_ = os.Unsetenv("CUSTOM_NEW_VAR")
	_ = os.Unsetenv("ANDROID_HOME")
}
