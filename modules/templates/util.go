// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"fmt"
	"reflect"
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
