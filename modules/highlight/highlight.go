// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package highlight

import (
	"path"
	"strings"

	"code.gitea.io/gitea/modules/setting"
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
		".arm":    {},
		".as":     {},
		".sh":     {},
		".cs":     {},
		".cpp":    {},
		".c":      {},
		".css":    {},
		".cmake":  {},
		".bat":    {},
		".dart":   {},
		".patch":  {},
		".elixir": {},
		".erlang": {},
		".go":     {},
		".html":   {},
		".xml":    {},
		".hs":     {},
		".ini":    {},
		".json":   {},
		".java":   {},
		".js":     {},
		".less":   {},
		".lua":    {},
		".php":    {},
		".py":     {},
		".rb":     {},
		".scss":   {},
		".sql":    {},
		".scala":  {},
		".swift":  {},
		".ts":     {},
		".vb":     {},
		".yml":    {},
		".yaml":   {},
	}

	// Extensions that are not same as highlight classes.
	highlightMapping = map[string]string{
		".txt": "nohighlight",
	}
)

// NewContext loads highlight map
func NewContext() {
	keys := setting.Cfg.Section("highlight.mapping").Keys()
	for i := range keys {
		highlightMapping[keys[i].Name()] = keys[i].Value()
	}
}

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

	name, ok := highlightMapping[ext]
	if ok {
		return name
	}

	return ""
}
