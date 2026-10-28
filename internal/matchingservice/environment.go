package matchingservice

import (
	"context"
	"encoding/json"
	"fmt"
	"mywebscrcpy/internal/dependencies"
	pythonsdk "mywebscrcpy/packages/python-sdk"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Dependency struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Error     string `json:"error"`
}
type Environment struct {
	Warning      string       `json:"warning"`
	Path         string       `json:"path"`
	Version      string       `json:"version"`
	Ready        bool         `json:"ready"`
	Dependencies []Dependency `json:"dependencies"`
	Error        string       `json:"error"`
}

const probeScript = `import importlib,json,sys
items=[]
from importlib.metadata import version
try:
    from mywebscrcpy import Client, Device, Matcher, FrameTimeout
    from mywebscrcpy.dependencies import vision_dependencies
    v = version("mywebscrcpy-sdk")
    req = sys.argv[1]
    if v != req and v.replace(".post", "-") != req and req.replace(".post", "-") != v:
        raise RuntimeError("SDK 版本不符，需要 " + sys.argv[1])
    items.append(dict(name="mywebscrcpy-sdk",available=True,error=""))
except Exception as e:
    items.append(dict(name="mywebscrcpy-sdk",available=False,error=type(e).__name__+": "+str(e)))
for name in ('httpx','websocket','av','cv2','numpy'):
    try:
        importlib.import_module(name)
        items.append(dict(name=name,available=True,error=''))
    except Exception as e:
        items.append(dict(name=name,available=False,error=type(e).__name__+': '+str(e)))
version='.'.join(map(str,sys.version_info[:3]))
print(json.dumps(dict(version=version,ready=sys.version_info>=(3,10) and all(i['available'] for i in items),dependencies=items,error='' if sys.version_info>=(3,10) else '需要 Python 3.10 或更高版本')))
`

func inspectPython(ctx context.Context, path string) Environment {
	result := Environment{Dependencies: []Dependency{}}
	result.Path = path
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "-B", "-c", probeScript, pythonsdk.Version)
	cmd.WaitDelay = time.Second
	cmd.Dir, cmd.Env = os.TempDir(), dependencies.PythonEnvironment()
	var stderr limitedOutput
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		result.Error = fmt.Sprintf("Python 环境检测失败: %v", err)
		return result
	}
	if err = json.Unmarshal(output, &result); err != nil {
		result.Error = "Python 环境返回无效检测结果"
		result.Ready = false
		return result
	}
	result.Path = path
	if len(result.Dependencies) != 6 || result.Version == "" {
		result.Error = "Python 环境检测字段不完整"
		result.Ready = false
		return result
	}
	for i, name := range []string{"mywebscrcpy-sdk", "httpx", "websocket", "av", "cv2", "numpy"} {
		if result.Dependencies[i].Name != name || !result.Dependencies[i].Available {
			result.Ready = false
		}
	}
	if stderr.Len() > 0 { /* Import warnings are kept distinct from actual failure. */
		result.Warning = stderr.String()
	}
	return result
}
func bounded(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// limitedOutput drains subprocess stderr while retaining only the first 1500 bytes.
type limitedOutput struct{ strings.Builder }

func (w *limitedOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := 1500 - w.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		w.Builder.Write(p)
	}
	return n, nil
}
