// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"testing"

	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestBaseLevelDB(t *testing.T) {
	_, err := newBaseLevelQueueGeneric(&BaseConfig{ConnStr: "redis://"}, false)
	assert.ErrorContains(t, err, "invalid leveldb connection string")

	_, err = newBaseLevelQueueGeneric(&BaseConfig{DataFullDir: "relative"}, false)
	assert.ErrorContains(t, err, "invalid leveldb data dir")

	optsSimple := testQueueBasicOptions{NotifiableQueue: true}
	optsUnique := testQueueBasicOptions{UniqueQueue: true, NotifiableQueue: true}
	testQueueBasic(t, newBaseLevelQueueSimple, toBaseConfig("baseLevelQueue", setting.QueueSettings{Datadir: t.TempDir() + "/queue-test", Length: 10}), optsSimple)
	testQueueBasic(t, newBaseLevelQueueUnique, toBaseConfig("baseLevelQueueUnique", setting.QueueSettings{ConnStr: "leveldb://" + t.TempDir() + "/queue-test", Length: 10}), optsUnique)
}
