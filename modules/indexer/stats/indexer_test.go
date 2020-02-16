// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package stats

import (
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"

	"gopkg.in/ini.v1"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	models.MainTest(m, filepath.Join("..", "..", ".."))
}

func TestRepoStatsIndex(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	setting.Cfg = ini.Empty()

	setting.NewQueueService()

	err := Init()
	assert.NoError(t, err)

	time.Sleep(5 * time.Second)

	repo, err := models.GetRepositoryByID(1)
	assert.NoError(t, err)
	langs, err := repo.GetTopLanguageStats(5)
	assert.NoError(t, err)
	assert.Len(t, langs, 1)
	assert.Equal(t, "other", langs[0].Language)
	assert.Equal(t, float32(100), langs[0].Percentage)
}
