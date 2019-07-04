// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import "code.gitea.io/gitea/modules/log"

// Migration settings
var Migration = struct {
	SaveBatchSize int
}{
	SaveBatchSize: 100,
}

func configMigration() {
	if err := Cfg.Section("migration").MapTo(&Migration); err != nil {
		log.Fatal("Failed to map Migration settings: %v", err)
	}
}
