package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
)

func TestConvertMap(t *testing.T) {
	input := map[interface{}]interface{}{
		"key1": "value1",
		"key2": map[interface{}]interface{}{
			"nestedKey1": "nestedValue1",
		},
		"key3": []interface{}{
			"value3",
			map[interface{}]interface{}{
				"nestedKey2": "nestedValue2",
			},
		},
	}

	expected := map[string]interface{}{
		"key1": "value1",
		"key2": map[string]interface{}{
			"nestedKey1": "nestedValue1",
		},
		"key3": []interface{}{
			"value3",
			map[string]interface{}{
				"nestedKey2": "nestedValue2",
			},
		},
	}

	result := convertMap(input)
	if !compareMaps(result, expected) {
		t.Errorf("convertMap() = %v, want %v", result, expected)
	}
}

func TestWriteSchemaFile(t *testing.T) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	log := zapr.NewLogger(logger)

	schema := map[string]interface{}{
		"key1": "value1",
		"key2": map[string]interface{}{
			"nestedKey1": "nestedValue1",
		},
	}

	filename := "test_schema.json"
	defer os.Remove(filename)

	writeSchemaFile(log, schema, filename)

	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if !compareMaps(result, schema) {
		t.Errorf("writeSchemaFile() = %v, want %v", result, schema)
	}
}

func TestMainFunction(t *testing.T) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	yamlData := `
spec:
  names:
    kind: TestKind
  versions:
    - name: v1
      schema:
        openAPIV3Schema:
          key1: value1
          key2:
            nestedKey1: nestedValue1
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(yamlData))
	}))
	defer server.Close()

	os.Args = []string{"cmd", server.URL}
	main()

	filename := "testkind_v1.json"
	defer os.Remove(filename)

	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	expected := map[string]interface{}{
		"key1": "value1",
		"key2": map[string]interface{}{
			"nestedKey1": "nestedValue1",
		},
	}

	if !compareMaps(result, expected) {
		t.Errorf("main() = %v, want %v", result, expected)
	}
}

func compareMaps(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v := range a {
		switch v := v.(type) {
		case map[string]interface{}:
			if bv, ok := b[k].(map[string]interface{}); ok {
				if !compareMaps(v, bv) {
					return false
				}
			} else {
				return false
			}
		case []interface{}:
			if bv, ok := b[k].([]interface{}); ok {
				if !compareSlices(v, bv) {
					return false
				}
			} else {
				return false
			}
		default:
			if b[k] != v {
				return false
			}
		}
	}

	return true
}

func compareSlices(a, b []interface{}) bool {
	if len(a) != len(b) {
		return false
	}

	for i, v := range a {
		switch v := v.(type) {
		case map[string]interface{}:
			if bv, ok := b[i].(map[string]interface{}); ok {
				if !compareMaps(v, bv) {
					return false
				}
			} else {
				return false
			}
		case []interface{}:
			if bv, ok := b[i].([]interface{}); ok {
				if !compareSlices(v, bv) {
					return false
				}
			} else {
				return false
			}
		default:
			if b[i] != v {
				return false
			}
		}
	}

	return true
}
