// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build ignore

/*
This tool is used to compare the CSS names in a chroma builtin styles with the Gitea theme CSS names.

It outputs the difference between the two sets of CSS names, eg:

```
CSS names not in builtin:
.chroma .ln
----
Builtin CSS names not in file:
.chroma .vm
```

Developers could use this tool to re-sync the CSS names in the Gitea theme.
*/

package main

import (
	"os"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
)

func main() {
	if len(os.Args) != 2 {
		println("Usage: chroma-style-diff css-or-less-file")
		os.Exit(1)
	}

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}

	content := string(data)

	// a simple CSS parser to collect CSS names
	content = regexp.MustCompile("//.*\r?\n").ReplaceAllString(content, "\n")
	content = regexp.MustCompile("/\\*.*?\\*/").ReplaceAllString(content, "")
	matches := regexp.MustCompile("\\s*([-.#:\\w\\s]+)\\s*\\{[^}]*}").FindAllStringSubmatch(content, -1)

	cssNames := map[string]bool{}
	for _, matchGroup := range matches {
		cssName := strings.TrimSpace(matchGroup[1])
		cssNames[cssName] = true
	}

	// collect Chroma builtin CSS names
	builtin := map[string]bool{}
	for tokenType, cssName := range chroma.StandardTypes {
		if tokenType > 0 && cssName != "" {
			builtin[".chroma ."+cssName] = true
		}
	}

	// show the diff
	println("CSS names not in builtin:")
	for cssName := range cssNames {
		if !builtin[cssName] {
			println(cssName)
		}
	}
	println("----")
	println("Builtin CSS names not in file:")
	for cssName := range builtin {
		if !cssNames[cssName] {
			println(cssName)
		}
	}
}
