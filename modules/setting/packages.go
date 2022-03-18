// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import "code.gitea.io/gitea/modules/log"

// Package registry settings
var (
	Packages = struct {
		Storage
		Enabled bool
	}{
		Enabled: true,
	}
)

func newPackages() {
	if err := Cfg.Section("packages").MapTo(&Packages); err != nil {
		log.Fatal("Failed to map Packages settings: %v", err)
	}
}
