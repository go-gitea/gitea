// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"fmt"
	"html"
	"html/template"
	"reflect"

	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
)

func dictMerge(base map[string]any, arg any) bool {
	if arg == nil {
		return true
	}
	rv := reflect.ValueOf(arg)
	if rv.Kind() == reflect.Map {
		for _, k := range rv.MapKeys() {
			base[k.String()] = rv.MapIndex(k).Interface()
		}
		return true
	}
	return false
}

// dict is a helper function for creating a map[string]any from a list of key-value pairs.
// If the key is dot ".", the value is merged into the base map, just like Golang template's dot syntax: dot means current
// The dot syntax is highly discouraged because it might cause unclear key conflicts. It's always good to use explicit keys.
func dict(args ...any) (map[string]any, error) {
	if len(args)%2 != 0 {
		return nil, fmt.Errorf("invalid dict constructor syntax: must have key-value pairs")
	}
	m := make(map[string]any, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			return nil, fmt.Errorf("invalid dict constructor syntax: unable to merge args[%d]", i)
		}
		if key == "." {
			if ok = dictMerge(m, args[i+1]); !ok {
				return nil, fmt.Errorf("invalid dict constructor syntax: dot arg[%d] must be followed by a dict", i)
			}
		} else {
			m[key] = args[i+1]
		}
	}
	return m, nil
}

func dumpVarMarshalable(v any, dumped container.Set[uintptr]) (ret any, ok bool) {
	if v == nil {
		return nil, true
	}
	e := reflect.ValueOf(v)
	for e.Kind() == reflect.Pointer {
		e = e.Elem()
	}
	if e.CanAddr() {
		addr := e.UnsafeAddr()
		if !dumped.Add(addr) {
			return "[dumped]", false
		}
		defer dumped.Remove(addr)
	}
	switch e.Kind() {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return e.Interface(), true
	case reflect.Struct:
		m := map[string]any{}
		for i := 0; i < e.NumField(); i++ {
			k := e.Type().Field(i).Name
			if !e.Type().Field(i).IsExported() {
				continue
			}
			v := e.Field(i).Interface()
			m[k], _ = dumpVarMarshalable(v, dumped)
		}
		return m, true
	case reflect.Map:
		m := map[string]any{}
		for _, k := range e.MapKeys() {
			m[k.String()], _ = dumpVarMarshalable(e.MapIndex(k).Interface(), dumped)
		}
		return m, true
	case reflect.Array, reflect.Slice:
		var m []any
		for i := 0; i < e.Len(); i++ {
			v, _ := dumpVarMarshalable(e.Index(i).Interface(), dumped)
			m = append(m, v)
		}
		return m, true
	default:
		return "[" + reflect.TypeOf(v).String() + "]", false
	}
}

// dumpVar helps to dump a variable in a template, to help debugging and development.
func dumpVar(v any) template.HTML {
	if setting.IsProd {
		return "<pre>dumpVar: only available in dev mode</pre>"
	}
	m, ok := dumpVarMarshalable(v, make(container.Set[uintptr]))
	var dumpStr string
	jsonBytes, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		dumpStr = fmt.Sprintf("dumpVar: unable to marshal %T: %v", v, err)
	} else if ok {
		dumpStr = fmt.Sprintf("dumpVar: %T\n%s", v, string(jsonBytes))
	} else {
		dumpStr = fmt.Sprintf("dumpVar: unmarshalable %T\n%s", v, string(jsonBytes))
	}
	return template.HTML("<pre>" + html.EscapeString(dumpStr) + "</pre>")
}
