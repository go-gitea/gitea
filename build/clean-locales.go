// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/setting"

	"github.com/gobwas/glob"
)

func searchTranslationKeyInDirs(key string) (bool, error) {
	for _, dir := range []string{
		"cmd",
		"models",
		"modules",
		"routers",
		"services",
		"templates",
	} {
		found, err := searchTranslationKeyInDir(dir, key)
		if err != nil {
			return false, err
		}
		if found {
			return true, nil
		}
	}
	return false, nil
}

func searchTranslationKeyInDir(dir, key string) (bool, error) {
	errFound := errors.New("found")
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
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
		if strings.Contains(string(bs), `"`+key+`"`) {
			return errFound
		}

		return nil
	})
	if err == errFound {
		return true, nil
	}
	return false, err
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

			found, err := searchTranslationKeyInDirs(trKey)
			if err != nil {
				panic(err)
			}
			if !found {
				println("unused locale key:", trKey)
			}
		}
	}
}
