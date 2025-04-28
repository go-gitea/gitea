// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import (
	"fmt"
	"os"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

// Load project data from file, with optional validation
func Load(filename string, data any, validation bool) error {
	isJSON := strings.HasSuffix(filename, ".json")

	bs, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	if validation {
		err := validate(bs, data, isJSON)
		if err != nil {
			return err
		}
	}
	return unmarshal(bs, data, isJSON)
}

func unmarshal(bs []byte, data any, isJSON bool) error {
	if isJSON {
		return json.Unmarshal(bs, data)
	}
	return yaml.Unmarshal(bs, data)
}

func getSchema(filename string) (*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	c.LoadURL = openSchema
	return c.Compile(filename)
}

func validate(bs []byte, datatype any, isJSON bool) error {
	var v any
	err := unmarshal(bs, &v, isJSON)
	if err != nil {
		return err
	}
	if !isJSON {
		v, err = toStringKeys(v)
		if err != nil {
			return err
		}
	}

	var schemaFilename string
	switch datatype := datatype.(type) {
	case *[]*Issue:
		schemaFilename = "issue.json"
	case *[]*Milestone:
		schemaFilename = "milestone.json"
	default:
		return fmt.Errorf("file_format:validate: %T has not a validation implemented", datatype)
	}

	sch, err := getSchema(schemaFilename)
	if err != nil {
		return err
	}
	err = sch.Validate(v)
	if err != nil {
		log.Error("migration validation with %s failed:\n%#v", schemaFilename, err)
	}
	return err
}

func toStringKeys(val any) (any, error) {
	var err error
	switch val := val.(type) {
	case map[string]any:
		m := make(map[string]any)
		for k, v := range val {
			m[k], err = toStringKeys(v)
			if err != nil {
				return nil, err
			}
		}
		return m, nil
	case []any:
		l := make([]any, len(val))
		for i, v := range val {
			l[i], err = toStringKeys(v)
			if err != nil {
				return nil, err
			}
		}
		return l, nil
	case time.Time:
		return val.Format(time.RFC3339), nil
	default:
		return val, nil
	}
}
