// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"
)

func TestBaseLevelDB(t *testing.T) {
	oldAppDataPath := setting.AppDataPath
	setting.AppDataPath = t.TempDir() + "/queues"
	defer func() {
		setting.AppDataPath = oldAppDataPath
	}()

	testQueueBasic(t, newBaseLevelQueueSimple, toBaseConfig("baseLevelQueue", &IniConfig{Length: 10}), false)
	testQueueBasic(t, newBaseLevelQueueUnique, toBaseConfig("baseLevelQueueUnique", &IniConfig{Length: 10}), true)
}
