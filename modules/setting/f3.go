// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"code.gitea.io/gitea/modules/log"
)

// Friendly Forge Format (F3) settings
var (
	F3 = struct {
		Enabled bool
	}{
		Enabled: true,
	}
)

func newF3Service() {
	if err := Cfg.Section("F3").MapTo(&F3); err != nil {
		log.Fatal("Failed to map F3 settings: %v", err)
	}
}
