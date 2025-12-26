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

	"code.gitea.io/gitea/modules/glob"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/translation/i18n"
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

		switch {
		case d.IsDir():
			return nil
		// check unused and untranslated keys in the file
		case strings.HasSuffix(d.Name(), ".go") && !strings.HasSuffix(d.Name(), "_test.go"):
			untranslatedKeys, err := checkTranslationKeysInGoFile(dir, path, keys, res)
			if err != nil {
				return err
			}
			untranslatedSum = append(untranslatedSum, untranslatedKeys...)
		case strings.HasSuffix(d.Name(), ".tmpl"):
			fmt.Println("----checking template file:", path)
			untranslatedKeysInTmpl, err := checkTranslationKeysInTemplateFile(dir, path, keys, res)
			if err != nil {
				return err
			}
			untranslatedSum = append(untranslatedSum, untranslatedKeysInTmpl...)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return untranslatedSum, nil
}

func checkTranslationKeysInCall(path string, fset *token.FileSet, astf *ast.File, call *ast.CallExpr, arg ast.Expr, keys []string, res *[]bool) string {
	if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		key := strings.Trim(lit.Value, `"`)
		idx := slices.Index(keys, key)
		if idx == -1 {
			return key
		}
		(*res)[idx] = true
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
		for i, key := range keys {
			if g.Match(key) {
				(*res)[i] = true
				found = true
			}
		}
		if !found {
			return transKeyMatch
		}
	}
	return ""
}

func checkTranslationKeysInGoFile(dir, path string, keys []string, res *[]bool) ([]string, error) {
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
					if key := checkTranslationKeysInCall(path, fs, node, call, call.Args[0], keys, res); key != "" {
						untranslated = append(untranslated, key)
					}
				}
			case "TrN":
				if len(call.Args) >= 3 {
					if key := checkTranslationKeysInCall(path, fs, node, call, call.Args[1], keys, res); key != "" {
						untranslated = append(untranslated, key)
					}
					if key := checkTranslationKeysInCall(path, fs, node, call, call.Args[2], keys, res); key != "" {
						untranslated = append(untranslated, key)
					}
				}
			}
		}
		return true
	})

	return untranslated, err
}

func checkTranslationKeysInTemplateFile(dir, path string, keys []string, res *[]bool) ([]string, error) {
	untranslatedKeys := []string{}
	keysFoundInTempl, err := templates.FindTemplateKeys(path)
	if err != nil {
		return nil, err
	}
	fmt.Println("==== keys found in template:", keysFoundInTempl)
	for _, key := range keysFoundInTempl.Values() {
		idx := slices.Index(keys, key)
		if idx == -1 {
			found := false
			for _, uk := range keys {
				if glob.MustCompile(key).Match(uk) {
					found = true
					break
				}
			}
			if !found {
				untranslatedKeys = append(untranslatedKeys, key)
			}
		} else {
			(*res)[idx] = true
		}
	}
	return untranslatedKeys, nil
}

func main() {
	if len(os.Args) != 1 {
		println("usage: clean-locales")
		os.Exit(1)
	}

	fileContent, err := os.ReadFile("options/locale/locale_en-US.json")
	if err != nil {
		panic(err)
	}
	store := i18n.NewLocaleStore()
	if err = store.AddLocaleByJSON("English", "en-US", fileContent, nil); err != nil {
		log.Error("Failed to read translation from en-US: %v", err)
	}

	keys := store.Keys()
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
