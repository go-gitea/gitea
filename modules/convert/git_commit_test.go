// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git/providers/native"
	"code.gitea.io/gitea/modules/git/service"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestToCommitMeta(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	headRepo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)

	sha1 := native.StringHash("0000000000000000000000000000000000000000")
	signature := &service.Signature{Name: "Test Signature", Email: "test@email.com", When: time.Unix(0, 0)}
	tag := native.NewTag(
		native.NewObject(sha1, nil),
		"Test Tag",
		sha1,
		"Test Type",
		signature,
		"Test Message",
		nil)

	commitMeta := ToCommitMeta(headRepo, tag)

	assert.NotNil(t, commitMeta)
	assert.EqualValues(t, &api.CommitMeta{
		SHA:     "0000000000000000000000000000000000000000",
		URL:     util.URLJoin(headRepo.APIURL(), "git/commits", "0000000000000000000000000000000000000000"),
		Created: time.Unix(0, 0),
	}, commitMeta)
}
