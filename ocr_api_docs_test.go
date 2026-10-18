package main

import (
	_ "embed"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"mywebscrcpy/internal/ocr"
)

// Embed only the contract so this file can also run without a frontend build:
// go test ocr_api_docs_test.go -run TestOCRAPIDocs -count=1
//
//go:embed web/openapi.json
var ocrDocsSpec []byte

func TestOCRAPIDocsMatchRegisteredRoutes(t *testing.T) {
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(ocrDocsSpec, &spec); err != nil {
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
	if err := json.Unmarshal(ocrDocsSpec, &spec); err != nil {
		t.Fatal(err)
	}
	for name, response := range map[string]any{
		"OCRDetectResponse":  ocr.DetectResponse{},
		"OCRTextArtResponse": ocr.TextArtResponse{},
		"OCRBox":             ocr.Box{},
		"OCRStatusResponse":  ocr.StatusResponse{},
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

func ocrDocsObject(t *testing.T) map[string]any {
	t.Helper()
	var spec map[string]any
	if err := json.Unmarshal(ocrDocsSpec, &spec); err != nil {
		t.Fatal(err)
	}
	return spec
}

func ocrDocsAt(t *testing.T, value any, path ...string) any {
	t.Helper()
	for _, key := range path {
		object, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("expected object at %s, got %T", key, value)
		}
		value, ok = object[key]
		if !ok {
			t.Fatalf("missing contract key %s (path %v)", key, path)
		}
	}
	return value
}

func ocrDocsEqual(t *testing.T, got any, wantJSON string) {
	t.Helper()
	var want any
	if err := json.Unmarshal([]byte(wantJSON), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("contract mismatch: got %v, want %s", got, wantJSON)
	}
}

func TestOCRAPIDocsRequestConstraints(t *testing.T) {
	spec := ocrDocsObject(t)
	ocrDocsEqual(t, spec["openapi"], `"3.1.0"`)
	schemas := ocrDocsAt(t, spec, "components", "schemas")
	for _, name := range []string{"OCRDetectRequest", "OCRTextArtRequest", "OCRDetectUpload"} {
		t.Run(name, func(t *testing.T) {
			schema := ocrDocsAt(t, schemas, name)
			props := ocrDocsAt(t, schema, "properties").(map[string]any)
			// Check every actual request tag, not a hand-maintained subset.
			requestType := reflect.TypeOf(ocr.DetectRequest{})
			for i := 0; i < requestType.NumField(); i++ {
				field := requestType.Field(i)
				key := strings.Split(field.Tag.Get("json"), ",")[0]
				if _, ok := props[key]; !ok {
					t.Errorf("request field %s is undocumented", key)
				}
			}
			stabilization := ocrDocsAt(t, props, "stabilization_ms")
			for key, want := range map[string]string{
				"type": `"integer"`, "minimum": `0`, "maximum": `5000`, "default": `0`,
			} {
				ocrDocsEqual(t, ocrDocsAt(t, stabilization, key), want)
			}
			ocrDocsEqual(t, ocrDocsAt(t, props, "engine", "enum"), `["auto","parallel","indraocr","macocr","paddleocr"]`)
			if name == "OCRDetectUpload" {
				if required, ok := schema.(map[string]any)["required"].([]any); ok && slices.Contains(required, any("file")) {
					t.Error("multipart file must not be globally required")
				}
				// anyOf allows file+serial at zero/omitted duration; oneOf would reject it.
				ocrDocsEqual(t, ocrDocsAt(t, schema, "anyOf"), `[
					{"required":["file"]},
					{"required":["serial"],"properties":{"serial":{"type":"string","pattern":"\\S"}}}
				]`)
				// The required guard is essential: an absent duration must not trigger then.
				ocrDocsEqual(t, ocrDocsAt(t, schema, "if"), `{
					"required":["stabilization_ms"],
					"properties":{"stabilization_ms":{"exclusiveMinimum":0}}
				}`)
				ocrDocsEqual(t, ocrDocsAt(t, schema, "then"), `{
					"required":["serial"],
					"properties":{"serial":{"type":"string","pattern":"\\S"}},
					"not":{"required":["file"]}
				}`)
				ocrDocsEqual(t, ocrDocsAt(t, props, "bbox", "type"), `"string"`)
				ocrDocsEqual(t, ocrDocsAt(t, props, "keywords", "type"), `"string"`)
			} else {
				ocrDocsEqual(t, ocrDocsAt(t, schema, "required"), `["serial"]`)
				ocrDocsEqual(t, ocrDocsAt(t, props, "serial", "pattern"), `"\\S"`)
				ocrDocsEqual(t, ocrDocsAt(t, schema, "not"), `{"required":["file"]}`)
				ocrDocsEqual(t, ocrDocsAt(t, props, "bbox", "type"), `"array"`)
				ocrDocsEqual(t, ocrDocsAt(t, props, "keywords", "type"), `"array"`)
			}
		})
	}
}

func TestOCRAPIDocsOperationModesAndFailures(t *testing.T) {
	spec := ocrDocsObject(t)
	for path, request := range map[string]string{
		"/api/vision/ocr": "OCRDetectRequest", "/api/vision/ocr-text-art": "OCRTextArtRequest",
	} {
		t.Run(path, func(t *testing.T) {
			op := ocrDocsAt(t, spec, "paths", path, "post")
			for mediaType, name := range map[string]string{
				"application/json": request, "multipart/form-data": "OCRDetectUpload",
			} {
				ocrDocsEqual(t, ocrDocsAt(t, op, "requestBody", "content", mediaType, "schema", "$ref"), `"#/components/schemas/`+name+`"`)
			}
			response := "OCRDetectResponse"
			if strings.HasSuffix(path, "ocr-text-art") {
				response = "OCRTextArtResponse"
			}
			ocrDocsEqual(t, ocrDocsAt(t, op, "responses", "200", "content", "application/json", "schema", "$ref"), `"#/components/schemas/`+response+`"`)
			for _, status := range []string{"400", "408", "429", "502", "504"} {
				ocrDocsEqual(t, ocrDocsAt(t, op, "responses", status, "content", "application/json", "schema", "$ref"), `"#/components/schemas/OCRError"`)
			}
			// Preserve the operational limits along with the machine-readable shape.
			description := ocrDocsAt(t, op, "description").(string)
			for _, fact := range []string{"50 ms", "101", "30 秒", "5 秒", "最多 2 个", "auto", "parallel", "fallback_reason", "慢引擎超时"} {
				if !strings.Contains(description, fact) {
					t.Errorf("operation description omits %q", fact)
				}
			}
		})
	}
}

func TestOCRAPIDocsTextArtQueryAndMetadata(t *testing.T) {
	spec := ocrDocsObject(t)
	op := ocrDocsAt(t, spec, "paths", "/api/vision/ocr-text-art", "post")
	if required := ocrDocsAt(t, op, "requestBody").(map[string]any)["required"]; required == true {
		t.Error("text-art must retain bodyless query calls")
	}
	parameters := ocrDocsAt(t, op, "parameters").([]any)
	for _, name := range []string{"serial", "engine", "stabilization_ms", "columns", "rows"} {
		t.Run(name, func(t *testing.T) {
			var found map[string]any
			for _, parameter := range parameters {
				p := parameter.(map[string]any)
				if p["name"] == name && p["in"] == "query" {
					found = p
				}
			}
			if found == nil {
				t.Fatalf("missing text-art query parameter %s", name)
			}
			want := ocrDocsAt(t, spec, "components", "schemas", "OCRTextArtRequest", "properties", name)
			if !reflect.DeepEqual(found["schema"], want) {
				t.Errorf("query %s must use the same contract as JSON", name)
			}
		})
	}
	ocrDocsEqual(t, ocrDocsAt(t, op, "responses", "200", "content", "text/plain", "schema", "type"), `"string"`)
	for _, name := range []string{"OCRDetectResponse", "OCRTextArtResponse"} {
		props := ocrDocsAt(t, spec, "components", "schemas", name, "properties")
		ocrDocsEqual(t, ocrDocsAt(t, props, "samples", "type"), `"integer"`)
		ocrDocsEqual(t, ocrDocsAt(t, props, "samples", "minimum"), `1`)
		ocrDocsEqual(t, ocrDocsAt(t, props, "samples", "maximum"), `101`)
		ocrDocsEqual(t, ocrDocsAt(t, props, "engines_used", "type"), `"array"`)
		ocrDocsEqual(t, ocrDocsAt(t, props, "engines_used", "items", "type"), `"string"`)
		ocrDocsEqual(t, ocrDocsAt(t, props, "engines_used", "items", "enum"), `["indraocr","macocr","paddleocr"]`)
		ocrDocsEqual(t, ocrDocsAt(t, props, "engine", "enum"), `["parallel","indraocr","macocr","paddleocr"]`)
	}
}
