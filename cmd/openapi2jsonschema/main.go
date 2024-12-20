package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

type Schema struct {
	Spec struct {
		Names struct {
			Kind string `yaml:"kind"`
		} `yaml:"names"`
		Group    string `yaml:"group"`
		Version  string `yaml:"version"`
		Versions []struct {
			Name   string `yaml:"name"`
			Schema struct {
				OpenAPIV3Schema map[interface{}]interface{} `yaml:"openAPIV3Schema"`
			} `yaml:"schema"`
		} `yaml:"versions"`
		Validation struct {
			OpenAPIV3Schema map[interface{}]interface{} `yaml:"openAPIV3Schema"`
		} `yaml:"validation"`
	} `yaml:"spec"`
}

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	log := zapr.NewLogger(logger)

	if len(os.Args) < 2 {
		log.Error(fmt.Errorf("missing FILE parameter"), "Usage: %s [FILE]", os.Args[0])
		os.Exit(1)
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}

	for _, crdFile := range os.Args[1:] {
		var data []byte
		var err error

		if strings.HasPrefix(crdFile, "http") {
			resp, err := client.Get(crdFile)
			if err != nil {
				log.Error(err, "Failed to fetch URL")
				continue
			}
			defer resp.Body.Close()
			data, err = io.ReadAll(resp.Body)
		} else {
			data, err = os.ReadFile(crdFile)
		}

		if err != nil {
			log.Error(err, "Failed to read file")
			continue
		}

		decoder := yaml.NewDecoder(bytes.NewReader(data))
		filenameFormat := os.Getenv("FILENAME_FORMAT")
		if filenameFormat == "" {
			filenameFormat = "{kind}_{version}"
		}

		for {
			var def Schema
			err = decoder.Decode(&def)
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Error(err, "Failed to unmarshal YAML")
				continue
			}

			if len(def.Spec.Versions) > 0 {
				for _, version := range def.Spec.Versions {
					if version.Schema.OpenAPIV3Schema != nil {
						filename := strings.ReplaceAll(filenameFormat, "{kind}", def.Spec.Names.Kind)
						filename = strings.ReplaceAll(filename, "{version}", version.Name)
						filename = strings.ToLower(filename) + ".json"
						writeSchemaFile(log, convertMap(version.Schema.OpenAPIV3Schema), filename)
					}
				}
			} else if def.Spec.Validation.OpenAPIV3Schema != nil {
				filename := strings.ReplaceAll(filenameFormat, "{kind}", def.Spec.Names.Kind)
				filename = strings.ReplaceAll(filename, "{version}", def.Spec.Version)
				filename = strings.ToLower(filename) + ".json"
				writeSchemaFile(log, convertMap(def.Spec.Validation.OpenAPIV3Schema), filename)
			}
		}
	}
}

func convertMap(m map[interface{}]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		key := fmt.Sprintf("%v", k)
		switch value := v.(type) {
		case map[interface{}]interface{}:
			result[key] = convertMap(value)
		case []interface{}:
			result[key] = convertSlice(value)
		default:
			result[key] = value
		}
	}
	return result
}

func convertSlice(s []interface{}) []interface{} {
	for i, v := range s {
		switch value := v.(type) {
		case map[interface{}]interface{}:
			s[i] = convertMap(value)
		case []interface{}:
			s[i] = convertSlice(value)
		}
	}
	return s
}

func writeSchemaFile(logger logr.Logger, schema map[string]interface{}, filename string) {
	schemaJSON, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		logger.Error(err, "Failed to marshal schema to JSON")
		return
	}

	err = os.WriteFile(filename, schemaJSON, 0644)
	if err != nil {
		logger.Error(err, "Failed to write schema to file")
		return
	}

	logger.Info("JSON schema written", "filename", filename)
}
