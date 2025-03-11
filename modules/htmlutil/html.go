// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package htmlutil

import (
	"fmt"
	"html/template"
	"slices"
)

// ParseSizeAndClass get size and class from string with default values
// If present, "others" expects the new size first and then the classes to use
func ParseSizeAndClass(defaultSize int, defaultClass string, others ...any) (int, string) {
	size := defaultSize
	if len(others) >= 1 {
		if v, ok := others[0].(int); ok && v != 0 {
			size = v
		}
	}
	class := defaultClass
	if len(others) >= 2 {
		if v, ok := others[1].(string); ok && v != "" {
			if class != "" {
				class += " "
			}
			class += v
		}
	}
	return size, class
}

func HTMLFormat(s template.HTML, rawArgs ...any) template.HTML {
	args := slices.Clone(rawArgs)
	for i, v := range args {
		switch v := v.(type) {
		case nil, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, template.HTML:
			// for most basic types (including template.HTML which is safe), just do nothing and use it
		case string:
			args[i] = template.HTMLEscapeString(v)
		case fmt.Stringer:
			args[i] = template.HTMLEscapeString(v.String())
		default:
			args[i] = template.HTMLEscapeString(fmt.Sprint(v))
		}
	}
	return template.HTML(fmt.Sprintf(string(s), args...))
}
