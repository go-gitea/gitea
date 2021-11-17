// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package stats

import (
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
	"gopkg.in/ini.v1"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, filepath.Join("..", "..", ".."))
}

func TestRepoStatsIndex(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	setting.Cfg = ini.Empty()

	setting.NewQueueService()

	err := Init()
	assert.NoError(t, err)

	time.Sleep(5 * time.Second)

	repo, err := models.GetRepositoryByID(1)
	assert.NoError(t, err)
	status, err := repo.GetIndexerStatus(models.RepoIndexerTypeStats)
	assert.NoError(t, err)
	assert.Equal(t, "65f1bf27bc3bf70f64657658635e66094edbcb4d", status.CommitSha)
	langs, err := repo.GetTopLanguageStats(5)
	assert.NoError(t, err)
	assert.Empty(t, langs)
}
