// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package highlight

import (
	"path"
	"strings"
)

var (
	// File name should ignore highlight.
	ignoreFileNames = map[string]bool{
		"license": true,
		"copying": true,
	}

	// File names that are representing highlight classes.
	highlightFileNames = map[string]bool{
		"dockerfile": true,
		"makefile":   true,
	}

	// Extensions that are same as highlight classes.
	highlightExts = map[string]struct{}{
		".arm":   {},
		".as":    {},
		".sh":    {},
		".cs":    {},
		".cpp":   {},
		".c":     {},
		".css":   {},
		".cmake": {},
		".bat":   {},
		".dart":  {},
		".patch": {},
		".erl":   {},
		".go":    {},
		".html":  {},
		".xml":   {},
		".hs":    {},
		".ini":   {},
		".json":  {},
		".java":  {},
		".js":    {},
		".less":  {},
		".lua":   {},
		".php":   {},
		".py":    {},
		".rb":    {},
		".rs":    {},
		".scss":  {},
		".sql":   {},
		".scala": {},
		".swift": {},
		".ts":    {},
		".vb":    {},
		".yml":   {},
		".yaml":  {},
	}

	// Extensions that are not same as highlight classes.
	highlightMapping = map[string]string{
		".txt":     "nohighlight",
		".escript": "erlang",
		".ex":      "elixir",
		".exs":     "elixir",
	}
)

// FileNameToHighlightClass returns the best match for highlight class name
// based on the rule of highlight.js.
func FileNameToHighlightClass(fname string) string {
	fname = strings.ToLower(fname)
	if ignoreFileNames[fname] {
		return "nohighlight"
	}

	if highlightFileNames[fname] {
		return fname
	}

	ext := path.Ext(fname)
	if _, ok := highlightExts[ext]; ok {
		return ext[1:]
	}

	name, ok := HighlightMapping[ext]
	if ok {
		return name
	}

	return ""
}
