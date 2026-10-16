package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"mywebscrcpy/internal/ocr"
)

func TestOCRAPIDocsMatchRegisteredRoutes(t *testing.T) {
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(openAPISpec, &spec); err != nil {
		t.Fatal(err)
	}
	source, err := parser.ParseFile(token.NewFileSet(), "internal/ocr/routes.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ast.Inspect(source, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		method, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || method.Sel.Name != "HandleFunc" {
			return true
		}
		pattern, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			t.Fatal("OCR route must use a literal pattern")
		}
		value, err := strconv.Unquote(pattern.Value)
		if err != nil {
			t.Fatal(err)
		}
		verb, path, ok := strings.Cut(value, " ")
		if !ok {
			t.Fatalf("OCR route has no method: %s", value)
		}
		if _, ok := spec.Paths[path][strings.ToLower(verb)]; !ok {
			t.Errorf("registered OCR route %s is missing from OpenAPI", value)
		}
		return true
	})
}

func TestOCRAPIDocsMatchResponseFields(t *testing.T) {
	var spec struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]json.RawMessage `json:"properties"`
				Required   []string                   `json:"required"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(openAPISpec, &spec); err != nil {
		t.Fatal(err)
	}
	for name, response := range map[string]any{
		"OCRDetectResponse": ocr.DetectResponse{},
		"OCRBox":            ocr.Box{},
		"OCRStatusResponse": ocr.StatusResponse{},
	} {
		t.Run(name, func(t *testing.T) {
			schema := spec.Components.Schemas[name]
			responseType := reflect.TypeOf(response)
			if len(schema.Properties) != responseType.NumField() {
				t.Errorf("schema has %d fields, response has %d", len(schema.Properties), responseType.NumField())
			}
			for i := 0; i < responseType.NumField(); i++ {
				field, options, _ := strings.Cut(responseType.Field(i).Tag.Get("json"), ",")
				if _, ok := schema.Properties[field]; !ok {
					t.Errorf("response field %s is undocumented", field)
				}
				required := false
				for _, value := range schema.Required {
					required = required || value == field
				}
				if required == strings.Contains(options, "omitempty") {
					t.Errorf("required flag for %s does not match the response", field)
				}
			}
		})
	}
}
