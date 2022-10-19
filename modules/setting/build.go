// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"code.gitea.io/gitea/modules/log"
)

// Builds settings
var (
	Builds = struct {
		Storage
		Enabled bool
	}{
		Enabled: true,
	}
)

func newBuilds() {
	sec := Cfg.Section("builds")
	if err := sec.MapTo(&Builds); err != nil {
		log.Fatal("Failed to map Builds settings: %v", err)
	}

	Builds.Storage = getStorage("builds", "", nil)
}
