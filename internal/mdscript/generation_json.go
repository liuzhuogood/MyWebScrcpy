package mdscript

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// uniqueJSON rejects duplicate fields at every level instead of accepting the last value.
func uniqueJSON(raw string) error {
	decoder := json.NewDecoder(strings.NewReader(raw))
	var value func(int) error
	value = func(depth int) error {
		if depth > 64 {
			return errors.New("生成 JSON 嵌套过深")
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok {
					return errors.New("JSON 键类型错误")
				}
				if seen[name] {
					return errors.New("生成 JSON 包含重复字段")
				}
				seen[name] = true
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		default:
			return errors.New("JSON 结构错误")
		}
		_, err = decoder.Token()
		return err
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("生成 JSON 包含多余内容")
	}
	return nil
}
