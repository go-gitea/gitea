// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"gitea.dev/modules/log"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/translation/i18n"
)

type keyUsage struct {
	keys     []string
	used     []bool
	keyIndex map[string]int
}

func newKeyUsage(keys []string) *keyUsage {
	idx := make(map[string]int, len(keys))
	for i, key := range keys {
		idx[key] = i
	}
	return &keyUsage{
		keys:     keys,
		used:     make([]bool, len(keys)),
		keyIndex: idx,
	}
}

func (u *keyUsage) mark(key string) bool {
	i, ok := u.keyIndex[key]
	if !ok {
		return false
	}
	u.used[i] = true
	return true
}

func (u *keyUsage) unused() []string {
	var unused []string
	for i, used := range u.used {
		if !used {
			unused = append(unused, u.keys[i])
		}
	}
	return unused
}

type scanIssue struct {
	path string
	line int
	msg  string
}

func (i scanIssue) String() string {
	if i.line > 0 {
		return fmt.Sprintf("%s:%d: %s", i.path, i.line, i.msg)
	}
	return fmt.Sprintf("%s: %s", i.path, i.msg)
}

func unquoteString(lit *ast.BasicLit) (string, bool) {
	if lit == nil || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

func selectorName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		if pkg, ok := e.X.(*ast.Ident); ok {
			return pkg.Name + "." + e.Sel.Name
		}
		return e.Sel.Name
	default:
		return ""
	}
}

func isCompositingCall(call *ast.CallExpr) bool {
	switch selectorName(call.Fun) {
	case "Sprintf", "Sprint", "fmt.Sprintf", "fmt.Sprint", "printf", "print":
		return true
	default:
		return false
	}
}

func isPassThroughKeyExpr(expr ast.Expr) bool {
	switch expr.(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
		return true
	default:
		return false
	}
}

func checkTrKeyArg(fset *token.FileSet, path string, arg ast.Expr, usage *keyUsage, missing, dynamic *[]scanIssue) {
	switch e := arg.(type) {
	case *ast.BasicLit:
		key, ok := unquoteString(e)
		if !ok {
			return
		}
		if !usage.mark(key) {
			*missing = append(*missing, scanIssue{path: path, line: fset.Position(e.Pos()).Line, msg: fmt.Sprintf("missing locale key %q", key)})
		}
	case *ast.BinaryExpr:
		*dynamic = append(*dynamic, scanIssue{path: path, line: fset.Position(e.Pos()).Line, msg: "composited translation key (string concatenation is forbidden)"})
	case *ast.CallExpr:
		if isCompositingCall(e) {
			*dynamic = append(*dynamic, scanIssue{path: path, line: fset.Position(e.Pos()).Line, msg: "composited translation key (printf/print is forbidden)"})
		}
	case *ast.ParenExpr:
		checkTrKeyArg(fset, path, e.X, usage, missing, dynamic)
	default:
		if !isPassThroughKeyExpr(e) {
			*dynamic = append(*dynamic, scanIssue{path: path, line: fset.Position(e.Pos()).Line, msg: "non-static translation key (keys must be verbatim string literals at the fusion point)"})
		}
	}
}

func markDeclaredLiteral(s string, usage *keyUsage) {
	if !strings.Contains(s, ".") {
		return
	}
	usage.mark(s)
}

func markLocaleStructTags(fset *token.FileSet, path string, astf *ast.File, usage *keyUsage, missing *[]scanIssue) {
	ast.Inspect(astf, func(n ast.Node) bool {
		field, ok := n.(*ast.Field)
		if !ok || field.Tag == nil {
			return true
		}
		raw, ok := unquoteString(field.Tag)
		if !ok {
			return true
		}
		key := reflect.StructTag(raw).Get("locale")
		if key == "" {
			return true
		}
		if !usage.mark(key) {
			*missing = append(*missing, scanIssue{path: path, line: fset.Position(field.Tag.Pos()).Line, msg: fmt.Sprintf("missing locale key %q", key)})
		}
		return true
	})
}

func checkTranslationKeysInGoFile(path string, usage *keyUsage) (missing, dynamic []scanIssue, err error) {
	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, err
	}

	markLocaleStructTags(fs, path, node, usage, &missing)

	ast.Inspect(node, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok {
			if s, ok := unquoteString(lit); ok {
				markDeclaredLiteral(s, usage)
			}
			return true
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		funIdent, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch funIdent.Sel.Name {
		case "Tr", "TrString":
			if len(call.Args) >= 1 {
				checkTrKeyArg(fs, path, call.Args[0], usage, &missing, &dynamic)
			}
		case "TrN":
			if len(call.Args) >= 3 {
				checkTrKeyArg(fs, path, call.Args[1], usage, &missing, &dynamic)
				checkTrKeyArg(fs, path, call.Args[2], usage, &missing, &dynamic)
			}
		case "ErrorWrapTranslatable":
			if len(call.Args) >= 2 {
				checkTrKeyArg(fs, path, call.Args[1], usage, &missing, &dynamic)
			}
		}
		return true
	})

	return missing, dynamic, nil
}

func checkTranslationKeysInTemplateFile(path string, usage *keyUsage) (missing, dynamic []scanIssue, err error) {
	scan, err := templates.ScanTemplateI18n(path)
	if err != nil {
		return nil, nil, err
	}
	for _, key := range scan.Keys.Values() {
		if !usage.mark(key) {
			missing = append(missing, scanIssue{path: path, msg: fmt.Sprintf("missing locale key %q", key)})
		}
	}
	for _, key := range scan.Declared.Values() {
		markDeclaredLiteral(key, usage)
	}
	for _, msg := range scan.Dynamic {
		dynamic = append(dynamic, scanIssue{path: path, msg: msg})
	}
	return missing, dynamic, nil
}

func searchTranslationKeys(usage *keyUsage) (missing, dynamic []scanIssue, err error) {
	for _, dir := range []string{
		"cmd",
		"models",
		"modules",
		"routers",
		"services",
		"templates",
	} {
		if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			switch {
			case strings.HasSuffix(d.Name(), ".go") && !strings.HasSuffix(d.Name(), "_test.go"):
				fileMissing, fileDynamic, err := checkTranslationKeysInGoFile(path, usage)
				if err != nil {
					return err
				}
				missing = append(missing, fileMissing...)
				dynamic = append(dynamic, fileDynamic...)
			case strings.HasSuffix(d.Name(), ".tmpl"):
				fileMissing, fileDynamic, err := checkTranslationKeysInTemplateFile(path, usage)
				if err != nil {
					return err
				}
				missing = append(missing, fileMissing...)
				dynamic = append(dynamic, fileDynamic...)
			}
			return nil
		}); err != nil {
			return nil, nil, err
		}
	}
	return missing, dynamic, nil
}

func printIssues(title string, issues []scanIssue) {
	if len(issues) == 0 {
		return
	}
	slices.SortFunc(issues, func(a, b scanIssue) int {
		if a.path != b.path {
			return strings.Compare(a.path, b.path)
		}
		if a.line != b.line {
			return a.line - b.line
		}
		return strings.Compare(a.msg, b.msg)
	})
	fmt.Printf("%s\n---\n", title)
	for _, issue := range issues {
		fmt.Println(issue)
	}
	fmt.Println()
}

func main() {
	if len(os.Args) != 1 {
		println("usage: i18n-check")
		os.Exit(1)
	}

	fileContent, err := os.ReadFile("options/locale/locale_en-US.json")
	if err != nil {
		panic(err)
	}
	store := i18n.NewLocaleStore()
	if err = store.AddLocaleByJSON("English", "en-US", fileContent, nil); err != nil {
		log.Error("Failed to read translation from en-US: %v", err)
		os.Exit(1)
	}

	usage := newKeyUsage(store.Keys())
	missing, dynamic, err := searchTranslationKeys(usage)
	if err != nil {
		panic(err)
	}

	unused := usage.unused()
	found := false
	if len(unused) > 0 {
		found = true
		fmt.Println("unused locale keys found\n---")
		for _, key := range unused {
			fmt.Println(key)
		}
		fmt.Println()
	}

	if len(missing) > 0 {
		found = true
		printIssues("missing locale keys found", missing)
	}
	if len(dynamic) > 0 {
		found = true
		printIssues("composited or non-static translation keys found", dynamic)
	}

	if found {
		os.Exit(1)
	}
}
