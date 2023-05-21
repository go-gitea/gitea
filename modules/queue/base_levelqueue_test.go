// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestBaseLevelDB(t *testing.T) {
	_, err := newBaseLevelQueueGeneric(&BaseConfig{ConnStr: "redis://"}, false)
	assert.ErrorContains(t, err, "invalid leveldb connection string")

	_, err = newBaseLevelQueueGeneric(&BaseConfig{DataFullDir: "relative"}, false)
	assert.ErrorContains(t, err, "invalid leveldb data dir")

	testQueueBasic(t, newBaseLevelQueueSimple, toBaseConfig("baseLevelQueue", setting.QueueSettings{Datadir: t.TempDir() + "/queue-test", Length: 10}), false)
	testQueueBasic(t, newBaseLevelQueueUnique, toBaseConfig("baseLevelQueueUnique", setting.QueueSettings{ConnStr: "leveldb://" + t.TempDir() + "/queue-test", Length: 10}), true)
}
