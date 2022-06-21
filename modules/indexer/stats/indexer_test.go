// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package stats

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"

	_ "code.gitea.io/gitea/models"

	"github.com/stretchr/testify/assert"
	"gopkg.in/ini.v1"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		GiteaRootPath: filepath.Join("..", "..", ".."),
	})
}

func TestRepoStatsIndex(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	setting.Cfg = ini.Empty()

	setting.NewQueueService()

	err := Init()
	assert.NoError(t, err)

	repo, err := repo_model.GetRepositoryByID(1)
	assert.NoError(t, err)

	err = UpdateRepoIndexer(repo)
	assert.NoError(t, err)

	queue.GetManager().FlushAll(context.Background(), 5*time.Second)

	status, err := repo_model.GetIndexerStatus(db.DefaultContext, repo, repo_model.RepoIndexerTypeStats)
	assert.NoError(t, err)
	assert.Equal(t, "65f1bf27bc3bf70f64657658635e66094edbcb4d", status.CommitSha)
	langs, err := repo_model.GetTopLanguageStats(repo, 5)
	assert.NoError(t, err)
	assert.Empty(t, langs)
}
