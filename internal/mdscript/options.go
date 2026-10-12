package mdscript

import (
	"fmt"
	yaml "go.yaml.in/yaml/v4"
	"mywebscrcpy/internal/taskartifact"
	"strings"
	"unicode/utf8"
)

// ParseOptions reads only a delimited header. The legacy scalar parser remains unchanged.
func ParseOptions(content string) (taskartifact.Options, error) {
	var options taskartifact.Options
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return options, nil
	}
	end := 0
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			end = i
			break
		}
	}
	if end == 0 {
		return options, fmt.Errorf("Markdown 头部缺少结束 ---")
	}
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &node); err != nil {
		return options, fmt.Errorf("Markdown 头部无效: %w", err)
	}
	if len(node.Content) == 0 {
		return options, nil
	}
	root := node.Content[0]
	if root.Kind != yaml.MappingNode {
		return options, fmt.Errorf("Markdown 头部必须是对象")
	}
	if err := validateHeaderNode(root); err != nil {
		return options, err
	}
	for i := 0; i < len(root.Content); i += 2 {
		key, value := root.Content[i].Value, root.Content[i+1]
		if key != "recording" && key != "report" {
			continue
		}
		if value.Kind != yaml.MappingNode {
			return options, fmt.Errorf("%s 必须是对象", key)
		}
		for j := 0; j < len(value.Content); j += 2 {
			field, n := value.Content[j].Value, value.Content[j+1]
			switch key + "." + field {
			case "recording.enabled", "recording.record_audio", "report.enabled":
				if n.Tag != "!!bool" || (n.Value != "true" && n.Value != "false") {
					return options, fmt.Errorf("%s.%s 必须是 true 或 false", key, field)
				}
				enabled := n.Value == "true"
				switch key + "." + field {
				case "recording.enabled":
					options.Recording.Enabled = enabled
				case "recording.record_audio":
					options.Recording.RecordAudio = enabled
				case "report.enabled":
					options.Report.Enabled = enabled
				}
			case "report.prompt":
				if n.Kind != yaml.ScalarNode || n.Tag != "!!str" || utf8.RuneCountInString(n.Value) > 4000 {
					return options, fmt.Errorf("report.prompt 必须是最多 4000 字的文本")
				}
				options.Report.Prompt = n.Value
			default:
				return options, fmt.Errorf("未知配置: %s.%s", key, field)
			}
		}
	}
	return options, nil
}
func validateHeaderNode(n *yaml.Node) error {
	if n.Kind == yaml.AliasNode || n.Anchor != "" {
		return fmt.Errorf("Markdown 头部不支持 YAML 别名或锚点")
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(n.Content); i += 2 {
			key := n.Content[i]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Value == "<<" {
				return fmt.Errorf("Markdown 头部键必须是文本，不支持合并键")
			}
			if seen[key.Value] {
				return fmt.Errorf("Markdown 头部重复键: %s", key.Value)
			}
			seen[key.Value] = true
		}
	}
	for _, child := range n.Content {
		if err := validateHeaderNode(child); err != nil {
			return err
		}
	}
	return nil
}
