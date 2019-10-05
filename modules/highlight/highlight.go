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
	highlightFileNames = map[string]string{
		"dockerfile":     "dockerfile",
		"makefile":       "makefile",
		"gnumakefile":    "makefile",
		"cmakelists.txt": "cmake",
	}

	// Extensions that are same as highlight classes.
	// See hljs.listLanguages() for list of language names.
	highlightExts = map[string]struct{}{
		".applescript": {},
		".arm":         {},
		".as":          {},
		".bash":        {},
		".bat":         {},
		".c":           {},
		".cmake":       {},
		".cpp":         {},
		".cs":          {},
		".css":         {},
		".dart":        {},
		".diff":        {},
		".django":      {},
		".go":          {},
		".gradle":      {},
		".groovy":      {},
		".haml":        {},
		".handlebars":  {},
		".html":        {},
		".ini":         {},
		".java":        {},
		".json":        {},
		".less":        {},
		".lua":         {},
		".php":         {},
		".scala":       {},
		".scss":        {},
		".sql":         {},
		".swift":       {},
		".ts":          {},
		".xml":         {},
		".yaml":        {},
	}

	// Extensions that are not same as highlight classes.
	highlightMapping = map[string]string{
		".ahk":     "autohotkey",
		".crmsh":   "crmsh",
		".dash":    "shell",
		".erl":     "erlang",
		".escript": "erlang",
		".ex":      "elixir",
		".exs":     "elixir",
		".f":       "fortran",
		".f77":     "fortran",
		".f90":     "fortran",
		".f95":     "fortran",
		".feature": "gherkin",
		".fish":    "shell",
		".for":     "fortran",
		".hbs":     "handlebars",
		".hs":      "haskell",
		".hx":      "haxe",
		".js":      "javascript",
		".jsx":     "javascript",
		".ksh":     "shell",
		".kt":      "kotlin",
		".l":       "ocaml",
		".ls":      "livescript",
		".md":      "markdown",
		".mjs":     "javascript",
		".mli":     "ocaml",
		".mll":     "ocaml",
		".mly":     "ocaml",
		".patch":   "diff",
		".pl":      "perl",
		".pm":      "perl",
		".ps1":     "powershell",
		".psd1":    "powershell",
		".psm1":    "powershell",
		".py":      "python",
		".pyw":     "python",
		".rb":      "ruby",
		".rs":      "rust",
		".scpt":    "applescript",
		".scptd":   "applescript",
		".sh":      "bash",
		".tcsh":    "shell",
		".ts":      "typescript",
		".tsx":     "typescript",
		".txt":     "plaintext",
		".vb":      "vbnet",
		".vbs":     "vbscript",
		".yml":     "yaml",
		".zsh":     "shell",
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

	if name, ok := highlightFileNames[fname]; ok {
		return name
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
