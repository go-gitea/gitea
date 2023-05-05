// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"strconv"
	"testing"
	"time"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestPullRequest_AddToTaskQueue(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	idChan := make(chan int64, 10)

	testHandler := func(items ...string) []string {
		for _, s := range items {
			id, _ := strconv.ParseInt(s, 10, 64)
			idChan <- id
		}
		return nil
	}
	iniCfg := setting.QueueSettings{Length: 10, BatchLength: 1, MaxWorkers: 1}
	prPatchCheckerQueue = queue.NewWorkerPoolQueueBySetting("pr_patch_checker", iniCfg, testHandler, true)

	var queueShutdown []func()
	var queueTerminate []func()

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	AddToTaskQueue(pr)

	assert.Eventually(t, func() bool {
		pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
		return pr.Status == issues_model.PullRequestStatusChecking
	}, 1*time.Second, 100*time.Millisecond)

	has, err := prPatchCheckerQueue.Has(strconv.FormatInt(pr.ID, 10))
	assert.True(t, has)
	assert.NoError(t, err)

	go prPatchCheckerQueue.Run(func(shutdown func()) {
		queueShutdown = append(queueShutdown, shutdown)
	}, func(terminate func()) {
		queueTerminate = append(queueTerminate, terminate)
	})

	select {
	case id := <-idChan:
		assert.EqualValues(t, pr.ID, id)
	case <-time.After(time.Second):
		assert.Fail(t, "Timeout: nothing was added to pullRequestQueue")
	}

	has, err = prPatchCheckerQueue.Has(strconv.FormatInt(pr.ID, 10))
	assert.False(t, has)
	assert.NoError(t, err)

	pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.Equal(t, issues_model.PullRequestStatusChecking, pr.Status)

	for _, callback := range queueShutdown {
		callback()
	}
	for _, callback := range queueTerminate {
		callback()
	}

	prPatchCheckerQueue = nil
}
