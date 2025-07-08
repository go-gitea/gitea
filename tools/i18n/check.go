// Copyright 2025 The Gitea Authors. All rights reserved.
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
	"slices"
	"strings"
	"text/template"
	"text/template/parse"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"

	"github.com/gobwas/glob"
)

func searchTranslationKeyInDirs(keys []string) ([]bool, []string, error) {
	res := make([]bool, len(keys))
	untranslatedKeysSum := make([]string, 0, 20)
	for _, dir := range []string{
		"cmd",
		"models",
		"modules",
		"routers",
		"services",
		"templates",
	} {
		untranslatedKeys, err := checkTranslationKeysInDir(dir, keys, &res)
		if err != nil {
			return nil, nil, err
		}

		untranslatedKeysSum = append(untranslatedKeysSum, untranslatedKeys...)
	}
	return res, untranslatedKeysSum, nil
}

func checkTranslationKeysInDir(dir string, keys []string, res *[]bool) ([]string, error) {
	var untranslatedSum []string
	if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() ||
			(!strings.HasSuffix(d.Name(), ".go") && !strings.HasSuffix(d.Name(), ".tmpl")) ||
			strings.HasSuffix(d.Name(), "_test.go") { // don't search in test files
			return nil
		}

		// search unused keys in the file
		if err := searchUnusedKeyInFile(dir, path, keys, res); err != nil {
			return err
		}

		// search untranslated keys in the file
		untranslated, err := searchUnTranslatedKeyInFile(dir, path, keys)
		if err != nil {
			return err
		}
		untranslatedSum = append(untranslatedSum, untranslated...)

		return nil
	}); err != nil {
		return nil, err
	}
	return untranslatedSum, nil
}

func searchUnusedKeyInFile(dir, path string, keys []string, res *[]bool) error {
	bs, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for i, key := range keys {
		if !(*res)[i] && strings.Contains(string(bs), `"`+key+`"`) {
			(*res)[i] = true
		}
	}
	return nil
}

func searchUntranslatedKeyInCall(path string, fset *token.FileSet, astf *ast.File, call *ast.CallExpr, arg ast.Expr, keys []string) string {
	if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		key := strings.Trim(lit.Value, `"`)
		if !slices.Contains(keys, key) {
			return key
		}
		return ""
	}

	var lastCg *ast.CommentGroup
	for _, cg := range astf.Comments {
		if cg.End() < call.Pos() {
			if lastCg == nil || cg.End() > lastCg.End() {
				lastCg = cg
			}
		}
	}
	if lastCg == nil {
		fmt.Printf("no comment found for a dynamic translation key: %s:%d\n", path, fset.Position(call.Pos()).Line)
		os.Exit(1)
		return ""
	}

	transKeyMatch, ok := strings.CutPrefix(lastCg.Text(), "i18n-check:")
	if !ok {
		fmt.Printf("no comment found for a dynamic translation key: %s:%d\n", path, fset.Position(call.Pos()).Line)
		os.Exit(1)
		return ""
	}
	transKeyMatch = strings.TrimSpace(transKeyMatch)
	switch transKeyMatch {
	case "ignore": // i18n-check: ignore
		return ""
	default: // i18n-check: <transKeyMatch>
		g := glob.MustCompile(transKeyMatch)
		found := false
		for _, key := range keys {
			if g.Match(key) {
				found = true
				break
			}
		}
		if !found {
			return transKeyMatch
		}
	}
	return ""
}

func searchUntranslatedKeyInGoFile(dir, path string, keys []string) ([]string, error) {
	if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
		return nil, nil
	}

	var untranslated []string
	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if funIdent, ok := call.Fun.(*ast.SelectorExpr); ok {
			switch funIdent.Sel.Name {
			case "Tr", "TrString":
				if len(call.Args) >= 1 {
					if key := searchUntranslatedKeyInCall(path, fs, node, call, call.Args[0], keys); key != "" {
						untranslated = append(untranslated, key)
					}
				}
			case "TrN":
				if len(call.Args) >= 3 {
					if key := searchUntranslatedKeyInCall(path, fs, node, call, call.Args[1], keys); key != "" {
						untranslated = append(untranslated, key)
					}
					if key := searchUntranslatedKeyInCall(path, fs, node, call, call.Args[2], keys); key != "" {
						untranslated = append(untranslated, key)
					}
				}
			}
		}
		return true
	})

	return untranslated, err
}

func extractI18nKeys(node parse.Node) []string {
	switch n := node.(type) {
	case *parse.ListNode:
		var keys []string
		for _, sub := range n.Nodes {
			keys = append(keys, extractI18nKeys(sub)...)
		}
		return keys
	case *parse.ActionNode:
		return extractI18nKeys(n.Pipe)
	case *parse.PipeNode:
		var keys []string
		for _, cmd := range n.Cmds {
			keys = append(keys, extractI18nKeys(cmd)...)
		}
		return keys
	case *parse.CommandNode:
		if len(n.Args) >= 2 {
			if ident, ok := n.Args[0].(*parse.IdentifierNode); ok && ident.Ident == "ctx.locale.Tr" {
				if str, ok := n.Args[1].(*parse.StringNode); ok {
					return []string{str.Text}
				}
			}
		}
	}
	return nil
}

func searchUntranslatedKeyInTemplateFile(dir, path string, keys []string) ([]string, error) {
	if filepath.Ext(path) != ".tmpl" {
		return nil, nil
	}

	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// The template parser requires the function map otherwise it will return failure
	t, err := template.New("test").Funcs(templates.NewFuncMap()).Parse(string(bs))
	if err != nil {
		return nil, err
	}

	untranslatedKeys := []string{}
	keysFoundInTempl := extractI18nKeys(t.Root)
	for _, key := range keysFoundInTempl {
		if !slices.Contains(keys, key) {
			untranslatedKeys = append(untranslatedKeys, key)
		}
	}
	return untranslatedKeys, nil
}

func searchUnTranslatedKeyInFile(dir, path string, keys []string) ([]string, error) {
	untranslatedKeys, err := searchUntranslatedKeyInGoFile(dir, path, keys)
	if err != nil {
		return nil, err
	}

	untranslatedKeysInTmpl, err := searchUntranslatedKeyInTemplateFile(dir, path, keys)
	if err != nil {
		return nil, err
	}
	return append(untranslatedKeys, untranslatedKeysInTmpl...), nil
}

func main() {
	if len(os.Args) != 1 {
		println("usage: clean-locales")
		os.Exit(1)
	}

	iniFile, err := setting.NewConfigProviderForLocale("options/locale/locale_en-US.ini")
	if err != nil {
		panic(err)
	}

	keys := []string{}
	for _, section := range iniFile.Sections() {
		for _, key := range section.Keys() {
			var trKey string
			if section.Name() == "" || section.Name() == "DEFAULT" {
				trKey = key.Name()
			} else {
				trKey = section.Name() + "." + key.Name()
			}
			keys = append(keys, trKey)
		}
	}

	results, untranslatedKeys, err := searchTranslationKeyInDirs(keys)
	if err != nil {
		panic(err)
	}

	var found bool
	for i, result := range results {
		if !result {
			if !found {
				println("unused locale keys found\n---")
				found = true
			}
			println(keys[i])
		}
	}

	if len(untranslatedKeys) > 0 {
		found = true
		println("\nuntranslated locale keys found\n---")
	}
	for _, key := range untranslatedKeys {
		println(key)
	}
	println()

	if found {
		os.Exit(1) // exit with error if any unused locale key is found
	}
}
