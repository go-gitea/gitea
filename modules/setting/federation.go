// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import "code.gitea.io/gitea/modules/log"

// Federation settings
var (
	Federation = struct {
		Enabled             bool
		ShareUserStatistics bool
	}{
		Enabled:             true,
		ShareUserStatistics: true,
	}
)

func newFederationService() {
	if err := Cfg.Section("federation").MapTo(&Federation); err != nil {
		log.Fatal("Failed to map Federation settings: %v", err)
	}
}
