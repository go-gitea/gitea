// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"testing"

	"code.gitea.io/gitea/models"

	"github.com/stretchr/testify/assert"
)

func TestPullRequest_APIFormat(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	pr := models.AssertExistsAndLoadBean(t, &models.PullRequest{ID: 1}).(*models.PullRequest)
	assert.NoError(t, pr.LoadAttributes())
	assert.NoError(t, pr.LoadIssue())
	apiPullRequest := ToAPIPullRequest(pr)
	assert.NotNil(t, apiPullRequest)
	assert.Nil(t, apiPullRequest.Head)
}
