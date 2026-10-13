package dependencies

import (
	"embed"
	"encoding/json"
	"runtime"
)

//go:embed catalog/*.json catalog/PI-LICENSE
var catalogFS embed.FS

type artifact struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}
type catalog struct {
	Version string                         `json:"version"`
	Targets map[string]map[string]artifact `json:"targets"`
}

func (m *Manager) spec(id string) (catalog, map[string]artifact, error) {
	var c catalog
	name := "pi"
	if id == "adb" {
		name = "adb"
	}
	raw, err := catalogFS.ReadFile("catalog/" + name + ".json")
	if err != nil {
		return c, nil, err
	}
	if err = json.Unmarshal(raw, &c); err != nil {
		return c, nil, err
	}
	return c, c.Targets[runtime.GOOS+"/"+runtime.GOARCH], nil
}
func (m *Manager) supported(id string) bool { _, s, e := m.spec(id); return e == nil && len(s) > 0 }
