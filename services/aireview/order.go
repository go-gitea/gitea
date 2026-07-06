// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import (
	"maps"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	goImport     = regexp.MustCompile(`(?m)^\s*import\s+(?:"([^"]+)"|"([^"]+)"\s+$)`)
	goImportMult = regexp.MustCompile(`(?m)^\s+"([^"]+)"`)
	jsImport     = regexp.MustCompile(`(?m)(?:import|require)\s*\(?\s*['"](\.\.?/)`)
	pyImport     = regexp.MustCompile(`(?m)^\s*(?:import|from)\s+(\S+)`)
)

// SortFilesByDependency orders files so that dependencies come before dependents.
// Uses a simple heuristic based on import/require statements.
func SortFilesByDependency(files []FileDiff) []FileDiff {
	if len(files) < 2 {
		return files
	}

	fileImports := make(map[string]map[string]bool, len(files))
	fileSet := make(map[string]bool, len(files))
	for _, f := range files {
		fileSet[f.Path] = true
		fileImports[f.Path] = extractImports(f.Path, f.Patch)
	}

	depGraph := make(map[string][]string)
	for _, f := range files {
		var deps []string
		for imp := range fileImports[f.Path] {
			for other := range fileSet {
				if other != f.Path && importMatchesPath(imp, other) {
					deps = append(deps, other)
				}
			}
		}
		depGraph[f.Path] = deps
	}

	// Topological sort (Kahn's algorithm)
	inDegree := make(map[string]int, len(files))
	for _, f := range files {
		inDegree[f.Path] = len(depGraph[f.Path])
	}

	// Group by in-degree for stable ordering
	// First pass: collect files already satisfied
	ordered := make([]FileDiff, 0, len(files))

	fileMap := make(map[string]FileDiff, len(files))
	for _, f := range files {
		fileMap[f.Path] = f
	}

	remaining := make(map[string]int, len(files))
	maps.Copy(remaining, inDegree)

	for len(ordered) < len(files) {
		var zero []string
		for path, deg := range remaining {
			if deg == 0 {
				zero = append(zero, path)
			}
		}
		if len(zero) == 0 {
			// Cycle or no more zero-degree — add remaining by depth
			for path := range remaining {
				zero = append(zero, path)
			}
		}

		sort.Slice(zero, func(i, j int) bool {
			di, dj := pathDepth(zero[i]), pathDepth(zero[j])
			if di != dj {
				return di < dj
			}
			return zero[i] < zero[j]
		})

		for _, path := range zero {
			ordered = append(ordered, fileMap[path])
			delete(remaining, path)
			for other := range remaining {
				for _, dep := range depGraph[other] {
					if dep == path {
						remaining[other]--
					}
				}
			}
		}
	}

	return ordered
}

// extractImports parses the diff/patch to find import statements.
func extractImports(filePath, patch string) map[string]bool {
	imports := make(map[string]bool)

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		matches := goImport.FindAllStringSubmatch(patch, -1)
		for _, m := range matches {
			if m[1] != "" {
				imports[m[1]] = true
			}
		}
		matches = goImportMult.FindAllStringSubmatch(patch, -1)
		for _, m := range matches {
			imports[m[1]] = true
		}
	case ".js", ".ts", ".jsx", ".tsx", ".mjs", ".cjs":
		matches := jsImport.FindAllStringSubmatch(patch, -1)
		for _, m := range matches {
			imports[m[1]] = true
		}
	case ".py":
		matches := pyImport.FindAllStringSubmatch(patch, -1)
		for _, m := range matches {
			imports[m[1]] = true
		}
	}

	return imports
}

// importMatchesPath checks if an import path refers to a file in the repo.
func importMatchesPath(importPath, filePath string) bool {
	imp := strings.TrimPrefix(importPath, "./")
	imp = strings.TrimPrefix(imp, "../")

	normPath := strings.ReplaceAll(filePath, "\\", "/")
	normDir := filepath.ToSlash(filepath.Dir(normPath))
	normStem := strings.TrimSuffix(filepath.Base(normPath), filepath.Ext(normPath))

	if strings.HasSuffix(imp, "/"+normDir) || imp == normDir {
		return true
	}

	if strings.HasSuffix(normPath, imp) {
		return true
	}

	parts := strings.Split(imp, "/")
	if len(parts) > 0 && parts[len(parts)-1] == normStem {
		return true
	}

	return false
}

// pathDepth counts directory depth (number of separators)
func pathDepth(path string) int {
	return strings.Count(path, "/") + strings.Count(path, "\\")
}
