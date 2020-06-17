// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build bindata

package themes

import (
	"path"
	"path/filepath"

	"code.gitea.io/gitea/modules/public"
	"code.gitea.io/gitea/modules/setting"

	"github.com/gobwas/glob"
)

// Discover locates installed themes
func Discover() []string {
	themes := []string{"gitea"}

	glob := glob.MustCompile("css/theme-*.css")
	for _, file := range public.AssetNames() {
		if glob.Match(file) {
			filename := path.Base(file)
			themes = append(themes, filename[6:len(filename)-4]) // chop off "theme-" and ".css"
		}
	}

	customFiles, _ := filepath.Glob(path.Join(setting.CustomPath, "public", "css", "theme-*.css"))
	for _, file := range customFiles {
		filename := path.Base(file)
		themes = append(themes, filename[6:len(filename)-4])
	}

	return themes
}
