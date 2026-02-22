// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package htmlutil

import (
	"fmt"
	"html/template"
	"io"
	"slices"
	"strings"
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

func htmlFormatArgs(s template.HTML, rawArgs []any) []any {
	if !strings.Contains(string(s), "%") || len(rawArgs) == 0 {
		panic("HTMLFormat requires one or more arguments")
	}
	args := slices.Clone(rawArgs)
	for i, v := range args {
		switch v := v.(type) {
		case nil, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, template.HTML:
			// for most basic types (including template.HTML which is safe), just do nothing and use it
		case string:
			args[i] = template.HTMLEscapeString(v)
		case template.URL:
			args[i] = template.HTMLEscapeString(string(v))
		case fmt.Stringer:
			args[i] = template.HTMLEscapeString(v.String())
		default:
			args[i] = template.HTMLEscapeString(fmt.Sprint(v))
		}
	}
	return args
}

func HTMLFormat(s template.HTML, rawArgs ...any) template.HTML {
	return template.HTML(fmt.Sprintf(string(s), htmlFormatArgs(s, rawArgs)...))
}

func HTMLPrintf(w io.Writer, s template.HTML, rawArgs ...any) (int, error) {
	return fmt.Fprintf(w, string(s), htmlFormatArgs(s, rawArgs)...)
}

func HTMLPrint(w io.Writer, s template.HTML) (int, error) {
	return io.WriteString(w, string(s))
}

func HTMLPrintTag(w io.Writer, tag template.HTML, attrs map[string]string) (written int, err error) {
	n, err := io.WriteString(w, "<"+string(tag))
	written += n
	if err != nil {
		return written, err
	}
	for k, v := range attrs {
		n, err = fmt.Fprintf(w, ` %s="%s"`, template.HTMLEscapeString(k), template.HTMLEscapeString(v))
		written += n
		if err != nil {
			return written, err
		}
	}
	n, err = io.WriteString(w, ">")
	written += n
	return written, err
}
