// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

func main() {
	if len(os.Args) != 2 {
		println("usage: backport-locales <to-ref>")
		println("eg: backport-locales release/v1.19")
		os.Exit(1)
	}

	ini.PrettyFormat = false
	mustNoErr := func(err error) {
		if err != nil {
			panic(err)
		}
	}
	collectInis := func(ref string) map[string]*ini.File {
		inis := map[string]*ini.File{}
		err := filepath.WalkDir("options/locale", func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".ini") {
				return nil
			}
			cfg, err := ini.LoadSources(ini.LoadOptions{
				IgnoreInlineComment:         true,
				UnescapeValueCommentSymbols: true,
			}, path)
			mustNoErr(err)
			inis[path] = cfg
			fmt.Printf("collecting: %s @ %s\n", path, ref)
			return nil
		})
		mustNoErr(err)
		return inis
	}

	// collect new locales from current working directory
	inisNew := collectInis("HEAD")

	// switch to the target ref, and collect the old locales
	cmd := exec.Command("git", "checkout", os.Args[1])
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	mustNoErr(cmd.Run())
	inisOld := collectInis(os.Args[1])

	// use old en-US as the base, and copy the new translations to the old locales
	enUsOld := inisOld["options/locale/locale_en-US.ini"]
	brokenWarned := map[string]bool{}
	for path, iniOld := range inisOld {
		if iniOld == enUsOld {
			continue
		}
		iniNew := inisNew[path]
		if iniNew == nil {
			continue
		}
		for _, secEnUS := range enUsOld.Sections() {
			secOld := iniOld.Section(secEnUS.Name())
			secNew := iniNew.Section(secEnUS.Name())
			for _, keyEnUs := range secEnUS.Keys() {
				if secNew.HasKey(keyEnUs.Name()) {
					oldStr := secOld.Key(keyEnUs.Name()).String()
					newStr := secNew.Key(keyEnUs.Name()).String()
					broken := oldStr != "" && strings.Count(oldStr, "%") != strings.Count(newStr, "%")
					broken = broken || strings.Contains(oldStr, "\n") || strings.Contains(oldStr, "\n")
					if broken {
						brokenWarned[secOld.Name()+"."+keyEnUs.Name()] = true
						fmt.Println("----")
						fmt.Printf("WARNING: skip broken locale: %s , [%s] %s\n", path, secEnUS.Name(), keyEnUs.Name())
						fmt.Printf("\told: %s\n", strings.ReplaceAll(oldStr, "\n", "\\n"))
						fmt.Printf("\tnew: %s\n", strings.ReplaceAll(newStr, "\n", "\\n"))
						continue
					}
					secOld.Key(keyEnUs.Name()).SetValue(newStr)
				}
			}
		}
		mustNoErr(iniOld.SaveTo(path))
	}

	fmt.Println("========")

	for path, iniNew := range inisNew {
		for _, sec := range iniNew.Sections() {
			for _, key := range sec.Keys() {
				str := sec.Key(key.Name()).String()
				broken := strings.Contains(str, "\n")
				broken = broken || strings.HasPrefix(str, "`") != strings.HasSuffix(str, "`")
				broken = broken || strings.HasPrefix(str, "\"`")
				broken = broken || strings.HasPrefix(str, "`\"")
				broken = broken || strings.Count(str, `"`)%2 == 1
				broken = broken || strings.Count(str, "`")%2 == 1
				if broken && !brokenWarned[sec.Name()+"."+key.Name()] {
					fmt.Printf("WARNING: found broken locale: %s , [%s] %s\n", path, sec.Name(), key.Name())
					fmt.Printf("\tstr: %s\n", strings.ReplaceAll(str, "\n", "\\n"))
					fmt.Println("----")
				}
			}
		}
	}
}
