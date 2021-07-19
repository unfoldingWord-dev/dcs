// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"

	"github.com/ghodss/yaml"
	"github.com/xeipuuv/gojsonschema"
)

// ValidateYAMLFile validates a yaml file
func ValidateYAMLFile(entry *git.TreeEntry) string {
	if _, err := ReadYAMLFromBlob(entry.Blob()); err != nil {
		return strings.ReplaceAll(err.Error(), " converting YAML to JSON", "")
	}
	return ""
}

// ValidateJSONFile validates a json file
func ValidateJSONFile(entry *git.TreeEntry) string {
	err := ValidateJSONFromBlob(entry.Blob())
	if err == nil {
		return ""
	}
	dataRc, err2 := entry.Blob().DataAsync()
	if err2 != nil {
		log.Error("DataAsync Error: %v\n", err2)
		return fmt.Sprintf("Error reading JSON file: %s\n", err.Error())
	}
	defer dataRc.Close()
	content, _ := ioutil.ReadAll(dataRc)

	switch err := err.(type) {
	case *json.SyntaxError:
		var errors string
		scanner := bufio.NewScanner(strings.NewReader(string(content)))
		var line int
		var readBytes int64
		for scanner.Scan() {
			// +1 for the \n character
			readBytes += int64(len(scanner.Bytes()) + 1)
			line++
			if readBytes >= err.Offset {
				errors += fmt.Sprintf("error: json: line %d: %s\n", line, err.Error())
			}
		}
		return errors
	default:
		log.Warn("Error decoding JSON: %v\n", err)
		return fmt.Sprintf("Error decoding JSON: %s\n", err.Error())
	}
}

// ValidateManifestFile validates a manifest file and returns the results as a string
func ValidateManifestFile(entry *git.TreeEntry) string {
	var result *gojsonschema.Result
	if entry != nil {
		if r, err := ValidateManifestTreeEntry(entry); err != nil {
			fmt.Printf("ValidateManifestTreeEntry: %v\n", err)
		} else {
			result = r
		}
	}
	return StringifyManifestValidationResults(result)
}

// StringifyManifestValidationResults returns the errors and a string
func StringifyManifestValidationResults(result *gojsonschema.Result) string {
	return StringifyValidationErrors(result)
}

// StringHasSuffix returns bool if str ends in the suffix
func StringHasSuffix(str string, suffix string) bool {
	return strings.HasSuffix(str, suffix)
}

// ValidateManifestTreeEntry validates a tree entry that is a manifest file and returns the results
func ValidateManifestTreeEntry(entry *git.TreeEntry) (*gojsonschema.Result, error) {
	manifest, err := ReadYAMLFromBlob(entry.Blob())
	if err != nil {
		return nil, err
	}
	return ValidateBlobByRC020Schema(manifest)
}

// StringifyValidationErrors returns a semi-colon & new line separated string of the errors
func StringifyValidationErrors(result *gojsonschema.Result) string {
	if result.Valid() {
		return ""
	}
	errStrings := make([]string, len(result.Errors()))
	for i, v := range result.Errors() {
		errStrings[i] = v.String()
	}
	return " * " + strings.Join(errStrings, ";\n * ")
}

// ValidateBlobByRC020Schema Validates a blob by the RC v0.2.0 schema and returns the result
func ValidateBlobByRC020Schema(manifest *map[string]interface{}) (*gojsonschema.Result, error) {
	schema, err := GetRC020Schema()
	if err != nil {
		return nil, err
	}
	schemaLoader := gojsonschema.NewBytesLoader(schema)
	documentLoader := gojsonschema.NewGoLoader(manifest)

	return gojsonschema.Validate(schemaLoader, documentLoader)
}

var rc02Schema []byte

// GetRC020Schema Returns the schema for RC v0.2, first trying the online URL, then from file if not already done
func GetRC020Schema() ([]byte, error) {
	rc02SchmeFileName := "rc.schema.json"
	schemaOnlineURL := "https://raw.githubusercontent.com/unfoldingWord/rc-schema/master/" + rc02SchmeFileName
	if rc02Schema == nil {
		var err error
		if res, err := http.Get(schemaOnlineURL); err == nil {
			defer res.Body.Close()
			// read all
			if body, err := ioutil.ReadAll(res.Body); err == nil {
				rc02Schema = body
			}
		}
		if rc02Schema == nil {
			// Failed to get schema online, falling back to the one in the options dir
			if rc02Schema, err = options.Schemas(rc02SchmeFileName); err != nil {
				return nil, err
			}
		}
	}
	return rc02Schema, nil
}

// ReadYAMLFromBlob reads a yaml file from a blob and unmarshals it
func ReadYAMLFromBlob(blob *git.Blob) (*map[string]interface{}, error) {
	dataRc, err := blob.DataAsync()
	if err != nil {
		log.Warn("DataAsync Error: %v\n", err)
		return nil, err
	}
	defer dataRc.Close()
	content, _ := ioutil.ReadAll(dataRc)

	var result *map[string]interface{}
	if err := yaml.Unmarshal(content, &result); err != nil {
		log.Error("yaml.Unmarshal: %v", err)
		return nil, err
	}
	return result, nil
}

// ValidateJSONFromBlob reads a json file from a blob and unmarshals it returning any errors
func ValidateJSONFromBlob(blob *git.Blob) error {
	dataRc, err := blob.DataAsync()
	if err != nil {
		log.Warn("DataAsync Error: %v\n", err)
		return err
	}
	defer dataRc.Close()
	content, _ := ioutil.ReadAll(dataRc)

	var result interface{}
	err = json.Unmarshal(content, &result)
	if err != nil {
		log.Error("json.Unmarshal: %v", err)
	}
	return err
}
