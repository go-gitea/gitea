// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models"
	pull_service "code.gitea.io/gitea/services/pull"

	"github.com/stretchr/testify/assert"
)

func TestPullUpdate(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {

		pr := models.AssertExistsAndLoadBean(t, &models.PullRequest{ID: 5}).(*models.PullRequest)
		user := models.AssertExistsAndLoadBean(t, &models.User{ID: 1}).(*models.User)

		//Test GetDiverging
		diffCount, err := pull_service.GetDiverging(pr)
		assert.NoError(t, err)
		assert.EqualValues(t, 1, diffCount.Behind)
		assert.EqualValues(t, 1, diffCount.Ahead)

		message := fmt.Sprintf("Merge branch '%s' into %s", pr.BaseBranch, pr.HeadBranch)
		err = pull_service.Update(pr,user, message)
		assert.NoError(t, err)

		//Test GetDiverging after update
		diffCount, err = pull_service.GetDiverging(pr)
		assert.NoError(t, err)
		assert.EqualValues(t, 0, diffCount.Behind)
		assert.EqualValues(t, 2, diffCount.Ahead)

	})
}
