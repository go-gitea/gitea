// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"context"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestManager(t *testing.T) {
	oldAppDataPath := setting.AppDataPath
	setting.AppDataPath = t.TempDir()
	defer func() {
		setting.AppDataPath = oldAppDataPath
	}()

	cfgProvider, err := setting.NewConfigProviderFromData(`
[queue]
TYPE = channel
DATADIR = queues-dir1
LENGTH = 100
BATCH_LENGTH = 20
CONN_STR = "addrs=127.0.0.1:6379 db=0"
QUEUE_NAME = "_queue1"
SET_NAME = "_unique1"
WORKERS = 1

[queue.sub]
TYPE = level
DATADIR = queues-dir2
LENGTH = 102
BATCH_LENGTH = 22
CONN_STR =
QUEUE_NAME = "q2"
SET_NAME = "u2"
WORKERS = 2
`)

	assert.NoError(t, err)

	q1 := createWorkerPoolQueue[string]("default", cfgProvider, nil, false)
	assert.Equal(t, "default", q1.GetName())
	assert.Equal(t, "dummy", q1.GetType()) // no handler
	assert.Equal(t, filepath.Join(setting.AppDataPath, "queues-dir1"), q1.baseConfig.DataFullDir)
	assert.Equal(t, 100, q1.baseConfig.Length)
	assert.Equal(t, 20, q1.batchLength)
	assert.Equal(t, "addrs=127.0.0.1:6379 db=0", q1.baseConfig.ConnStr)
	assert.Equal(t, "default_queue1", q1.baseConfig.QueueFullName)
	assert.Equal(t, "default_unique1", q1.baseConfig.SetFullName)
	assert.Equal(t, 1, q1.GetWorkerMaxNumber())
	assert.Equal(t, 0, q1.GetWorkerNumber())
	assert.Equal(t, 0, q1.GetWorkerActiveNumber())
	assert.Equal(t, 0, q1.GetQueueItemNumber())
	assert.Equal(t, "string", q1.GetItemTypeName())
	qid1 := GetManager().qidCounter

	q2 := createWorkerPoolQueue("sub", cfgProvider, func(s ...int) (unhandled []int) { return nil }, false)
	assert.Equal(t, "sub", q2.GetName())
	assert.Equal(t, "levelqueue", q2.GetType()) // no handler
	assert.Equal(t, filepath.Join(setting.AppDataPath, "queues-dir2"), q2.baseConfig.DataFullDir)
	assert.Equal(t, 102, q2.baseConfig.Length)
	assert.Equal(t, 22, q2.batchLength)
	assert.Equal(t, "", q2.baseConfig.ConnStr)
	assert.Equal(t, "q2", q2.baseConfig.QueueFullName)
	assert.Equal(t, "u2", q2.baseConfig.SetFullName)
	assert.Equal(t, 2, q2.GetWorkerMaxNumber())
	assert.Equal(t, 0, q2.GetWorkerNumber())
	assert.Equal(t, 0, q2.GetWorkerActiveNumber())
	assert.Equal(t, 0, q2.GetQueueItemNumber())
	assert.Equal(t, "int", q2.GetItemTypeName())
	qid2 := GetManager().qidCounter

	assert.Equal(t, q1, GetManager().ManagedQueues()[qid1])

	GetManager().GetManagedQueue(qid1).SetWorkerMaxNumber(120)
	assert.Equal(t, 120, q1.workerMaxNum)

	stop := runWorkerPoolQueue(q2)
	assert.NoError(t, GetManager().GetManagedQueue(qid2).FlushWithContext(context.Background(), 0))
	assert.NoError(t, GetManager().FlushAll(context.Background(), 0))
	stop()
}
