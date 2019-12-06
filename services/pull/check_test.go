// Copyright 2019 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPullRequest_AddToTaskQueue(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	pr := models.AssertExistsAndLoadBean(t, &models.PullRequest{ID: 1}).(*models.PullRequest)
	AddToTaskQueue(pr)

	select {
	case id := <-models.PullRequestQueue.Queue():
		assert.EqualValues(t, strconv.FormatInt(pr.ID, 10), id)
	case <-time.After(time.Second):
		assert.Fail(t, "Timeout: nothing was added to pullRequestQueue")
	}

	assert.True(t, models.PullRequestQueue.Exist(pr.ID))
	pr = AssertExistsAndLoadBean(t, &models.PullRequest{ID: 1}).(*models.PullRequest)
	assert.Equal(t, models.PullRequestStatusChecking, pr.Status)
}
