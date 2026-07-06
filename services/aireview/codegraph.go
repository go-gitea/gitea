// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	goFunc    = regexp.MustCompile(`(?m)^func\s+([A-Z]\w*)`)
	goType    = regexp.MustCompile(`(?m)^type\s+([A-Z]\w*)`)
	goVar     = regexp.MustCompile(`(?m)^var\s+([A-Z]\w*)`)
	jsExport  = regexp.MustCompile(`(?m)^export\s+(?:default\s+)?(?:function|class|const|let|var)\s+(\w+)`)
	pyFunc    = regexp.MustCompile(`(?m)^(?:async\s+)?def\s+([a-z]\w*)`)
	pyClass   = regexp.MustCompile(`(?m)^class\s+([A-Z]\w*)`)
)

// CodeGraph holds the relationship map between files in a PR.
type CodeGraph struct {
	Exports  map[string][]string // filePath → exported symbols
	Imports  map[string][]string // filePath → imported symbols (from other PR files)
}

// BuildCodeGraph analyzes all changed files and builds a relationship graph.
func BuildCodeGraph(files []FileDiff) *CodeGraph {
	cg := &CodeGraph{
		Exports: make(map[string][]string),
		Imports: make(map[string][]string),
	}

	for _, f := range files {
		cg.Exports[f.Path] = extractExports(f.Path, f.Patch)
		cg.Imports[f.Path] = extractImportsFromDiff(f.Path, f.Patch, files)
	}

	return cg
}

// String returns a human-readable representation of the code graph.
func (cg *CodeGraph) String() string {
	if len(cg.Exports) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n**Code relationship graph:**\n")

	for _, file := range sortedFileKeys(cg.Exports) {
		exports := cg.Exports[file]
		imports := cg.Imports[file]

		var parts []string
		if len(exports) > 0 {
			parts = append(parts, fmt.Sprintf("exports: %s", strings.Join(exports, ", ")))
		}
		if len(imports) > 0 {
			parts = append(parts, fmt.Sprintf("uses from PR: %s", strings.Join(imports, ", ")))
		}
		if len(parts) > 0 {
			b.WriteString(fmt.Sprintf("- `%s` — %s\n", file, strings.Join(parts, "; ")))
		}
	}

	return b.String()
}

// extractExports finds publicly exported symbols from a file's diff.
func extractExports(filePath, patch string) []string {
	ext := strings.ToLower(extension(filePath))
	var syms []string
	seen := make(map[string]bool)

	var matches [][]string
	switch ext {
	case ".go":
		matches = goFunc.FindAllStringSubmatch(patch, -1)
		matches = append(matches, goType.FindAllStringSubmatch(patch, -1)...)
		matches = append(matches, goVar.FindAllStringSubmatch(patch, -1)...)
	case ".js", ".ts", ".jsx", ".tsx", ".mjs", ".cjs":
		matches = jsExport.FindAllStringSubmatch(patch, -1)
	case ".py":
		matches = pyFunc.FindAllStringSubmatch(patch, -1)
		matches = append(matches, pyClass.FindAllStringSubmatch(patch, -1)...)
	}

	for _, m := range matches {
		if len(m) >= 2 && !seen[m[1]] {
			syms = append(syms, m[1])
			seen[m[1]] = true
		}
	}
	return syms
}

// extractImportsFromDiff finds imports that reference other files in the PR.
func extractImportsFromDiff(filePath, patch string, allFiles []FileDiff) []string {
	imports := extractImports(filePath, patch)
	if len(imports) == 0 {
		return nil
	}

	prFiles := make(map[string]bool)
	for _, f := range allFiles {
		prFiles[f.Path] = true
	}

	var result []string
	seen := make(map[string]bool)
	for imp := range imports {
		for prFile := range prFiles {
			if prFile != filePath && importMatchesPath(imp, prFile) && !seen[prFile] {
				result = append(result, prFile)
				seen[prFile] = true
			}
		}
	}
	return result
}

func extension(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i:]
		}
		if path[i] == '/' || path[i] == '\\' {
			break
		}
	}
	return ""
}

func sortedFileKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
