// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Script holds all the parameters necessary to compile or find in cache
// and then execute a script.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/modules-scripting.html
// for details of scripting.
type Script struct {
	script string
	typ    string
	lang   string
	params map[string]interface{}
}

// NewScript creates and initializes a new Script. By default, it is of
// type "inline". Use NewScriptStored for a stored script (where type is "id").
func NewScript(script string) *Script {
	return &Script{
		script: script,
		typ:    "inline",
		params: make(map[string]interface{}),
	}
}

// NewScriptInline creates and initializes a new inline script, i.e. code.
func NewScriptInline(script string) *Script {
	return NewScript(script).Type("inline")
}

// NewScriptStored creates and initializes a new stored script.
func NewScriptStored(script string) *Script {
	return NewScript(script).Type("id")
}

// Script is either the cache key of the script to be compiled/executed
// or the actual script source code for inline scripts. For indexed
// scripts this is the id used in the request. For file scripts this is
// the file name.
func (s *Script) Script(script string) *Script {
	s.script = script
	return s
}

// Type sets the type of script: "inline" or "id".
func (s *Script) Type(typ string) *Script {
	s.typ = typ
	return s
}

// Lang sets the language of the script. The default scripting language
// is Painless ("painless").
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/modules-scripting.html
// for details.
func (s *Script) Lang(lang string) *Script {
	s.lang = lang
	return s
}

// Param adds a key/value pair to the parameters that this script will be executed with.
func (s *Script) Param(name string, value interface{}) *Script {
	if s.params == nil {
		s.params = make(map[string]interface{})
	}
	s.params[name] = value
	return s
}

// Params sets the map of parameters this script will be executed with.
func (s *Script) Params(params map[string]interface{}) *Script {
	s.params = params
	return s
}

// Source returns the JSON serializable data for this Script.
func (s *Script) Source() (interface{}, error) {
	if s.typ == "" && s.lang == "" && len(s.params) == 0 {
		return s.script, nil
	}
	source := make(map[string]interface{})
	// Beginning with 6.0, the type can only be "source" or "id"
	if s.typ == "" || s.typ == "inline" {
		src, err := s.rawScriptSource(s.script)
		if err != nil {
			return nil, err
		}
		source["source"] = src
	} else {
		source["id"] = s.script
	}
	if s.lang != "" {
		source["lang"] = s.lang
	}
	if len(s.params) > 0 {
		source["params"] = s.params
	}
	return source, nil
}

// rawScriptSource returns an embeddable script. If it uses a short
// script form, e.g. "ctx._source.likes++" (without the quotes), it
// is quoted. Otherwise it returns the raw script that will be directly
// embedded into the JSON data.
func (s *Script) rawScriptSource(script string) (interface{}, error) {
	v := strings.TrimSpace(script)
	if !strings.HasPrefix(v, "{") && !strings.HasPrefix(v, `"`) {
		v = fmt.Sprintf("%q", v)
	}
	raw := json.RawMessage(v)
	return &raw, nil
}

// -- Script Field --

// ScriptField is a single script field.
type ScriptField struct {
	FieldName string // name of the field

	script        *Script
	ignoreFailure *bool // used in e.g. ScriptSource
}

// NewScriptField creates and initializes a new ScriptField.
func NewScriptField(fieldName string, script *Script) *ScriptField {
	return &ScriptField{FieldName: fieldName, script: script}
}

// IgnoreFailure indicates whether to ignore failures. It is used
// in e.g. ScriptSource.
func (f *ScriptField) IgnoreFailure(ignore bool) *ScriptField {
	f.ignoreFailure = &ignore
	return f
}

// Source returns the serializable JSON for the ScriptField.
func (f *ScriptField) Source() (interface{}, error) {
	if f.script == nil {
		return nil, errors.New("ScriptField expects script")
	}
	source := make(map[string]interface{})
	src, err := f.script.Source()
	if err != nil {
		return nil, err
	}
	source["script"] = src
	if v := f.ignoreFailure; v != nil {
		source["ignore_failure"] = *v
	}
	return source, nil
}
