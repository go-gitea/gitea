// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"net/http"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// create or check .well-known directory exist
func wellKnownWebDir() http.Dir {
	wellKnownDir := http.Dir(filepath.Join(setting.CustomPath, "web-well-known"))
	makeDirExist(string(wellKnownDir))

	return wellKnownDir
}

// check directory exist, create if not
func makeDirExist(dirName string) {
	_, err := os.Stat(dirName)
	if err == nil {
		return
	}
	if os.IsNotExist(err) {
		err := os.Mkdir(dirName, os.FileMode(0o755))
		if err != nil {
			log.Fatal(".well-known directory (%s) mkdir error %v", dirName, err)
		}
	} else {
		log.Warn(".well-known directory (%s) check error %v", dirName, err)
	}
}
