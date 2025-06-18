// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/setting"

	"github.com/gobwas/glob"
)

func searchTranslationKeyInDirs(keys []string) ([]bool, error) {
	res := make([]bool, len(keys))
	for _, dir := range []string{
		"cmd",
		"models",
		"modules",
		"routers",
		"services",
		"templates",
	} {
		if err := searchTranslationKeyInDir(dir, keys, &res); err != nil {
			return nil, err
		}
	}
	return res, nil
}

func searchTranslationKeyInDir(dir string, keys []string, res *[]bool) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() ||
			(!strings.HasSuffix(d.Name(), ".go") && !strings.HasSuffix(d.Name(), ".tmpl")) ||
			strings.HasSuffix(d.Name(), "_test.go") { // don't search in test files
			return nil
		}

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
	})
}

var whitelist = []string{
	"repo.signing.wont_sign.*",
	"repo.issues.role.*",
	"repo.commitstatus.*",
	"admin.dashboard.*",
	"admin.dashboard.cron.*",
	"admin.dashboard.task.*",
	"repo.migrate.*.description",
	"actions.runners.status.*",
	"projects.*.display_name",
	"admin.notices.*",
	"form.NewBranchName", // FIXME: used in integration tests only
}

func isWhitelisted(key string) bool {
	for _, w := range whitelist {
		if glob.MustCompile(w).Match(key) {
			return true
		}
	}
	return false
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
			if isWhitelisted(trKey) {
				continue
			}
			keys = append(keys, trKey)
		}
	}

	results, err := searchTranslationKeyInDirs(keys)
	if err != nil {
		panic(err)
	}

	var found bool
	for i, result := range results {
		if !result {
			found = true
			println("unused locale key:", keys[i])
		}
	}
	if found {
		os.Exit(1) // exit with error if any unused locale key is found
	}
}
