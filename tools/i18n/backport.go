// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		println("usage: backport-locales <to-ref>")
		println("eg: backport-locales release/v1.19")
		os.Exit(1)
	}

	mustNoErr := func(err error) {
		if err != nil {
			panic(err)
		}
	}
	collectJSONs := func(ref string) map[string]map[string]string {
		translates := map[string]map[string]string{} // locale -> key -> value
		err := filepath.WalkDir("options/locale", func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
				return nil
			}
			var translate = make(map[string]string)
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if err = json.Unmarshal(data, &translate); err != nil {
				return err
			}

			translates[filepath.Base(d.Name())] = translate

			fmt.Printf("collecting: %s @ %s\n", path, ref)
			return nil
		})
		mustNoErr(err)
		return translates
	}

	// collect new locales from current working directory
	translatesNew := collectJSONs("HEAD")

	// switch to the target ref, and collect the old locales
	cmd := exec.Command("git", "checkout", os.Args[1])
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	mustNoErr(cmd.Run())
	translatesOld := collectJSONs(os.Args[1])

	// use old en-US as the base, and copy the new translations to the old locales
	enUsOld := translatesOld["locale_en-US.json"]
	for path, translateOld := range translatesOld {
		jsonNew := translatesNew[path]
		if len(jsonNew) == 0 {
			continue
		}
		updated := false
		for keyEnUS, _ := range enUsOld {
			v, ok := jsonNew[keyEnUS]
			if ok && v != translateOld[keyEnUS] {
				translateOld[keyEnUS] = v
				updated = true
			}
		}
		if updated {

		}
	}
}
