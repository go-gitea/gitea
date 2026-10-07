// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"
	"time"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestPushMirrorsIterate(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	now := timeutil.TimeStampNow()

	db.Insert(t.Context(), &repo_model.PushMirror{
		RemoteName:     "test-1",
		LastUpdateUnix: now,
		Interval:       1,
	})

	long, _ := time.ParseDuration("24h")
	db.Insert(t.Context(), &repo_model.PushMirror{
		RemoteName:     "test-2",
		LastUpdateUnix: now,
		Interval:       long,
	})

	db.Insert(t.Context(), &repo_model.PushMirror{
		RemoteName:     "test-3",
		LastUpdateUnix: now,
		Interval:       0,
	})

	repo_model.PushMirrorsIterate(t.Context(), 1, func(idx int, bean any) error {
		m, ok := bean.(*repo_model.PushMirror)
		assert.True(t, ok)
		assert.Equal(t, "test-1", m.RemoteName)
		assert.Equal(t, m.RemoteName, m.GetRemoteName())
		return nil
	})
}

func TestPushMirrorConfigAndHistory(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	filters, err := repo_model.ParsePushMirrorBranchFilters(" main, release/*\n\nfeature/** ")
	assert.NoError(t, err)
	assert.Equal(t, []string{"main", "release/*", "feature/**"}, filters)
	_, err = repo_model.ParsePushMirrorBranchFilters("[")
	assert.Error(t, err)

	cfg := &repo_model.PushMirrorConfig{BranchFilters: filters}
	assert.True(t, cfg.MatchBranch("main"))
	assert.True(t, cfg.MatchBranch("release/1.0"))
	assert.False(t, cfg.MatchBranch("dev"))
	assert.True(t, (&repo_model.PushMirrorConfig{}).MatchBranch("anything"))

	m := &repo_model.PushMirror{RemoteName: "cfg-test", Config: repo_model.PushMirrorConfig{KeepRemoteTags: true, BranchFilters: filters}}
	assert.NoError(t, db.Insert(t.Context(), m))
	got := unittest.AssertExistsAndLoadBean(t, &repo_model.PushMirror{ID: m.ID})
	assert.Equal(t, m.Config, got.Config)

	for i := range 5 {
		assert.NoError(t, repo_model.AddPushMirrorHistory(t.Context(), &repo_model.PushMirrorHistory{
			PushMirrorID: m.ID,
			Status:       repo_model.PushMirrorStatusPartial,
			Result:       repo_model.PushMirrorResult{Pushed: i, Failed: []repo_model.PushMirrorRefError{{Ref: "refs/heads/x", Reason: "rejected"}}},
		}, 3))
	}
	list, err := repo_model.GetPushMirrorHistory(t.Context(), m.ID)
	assert.NoError(t, err)
	if assert.Len(t, list, 3) {
		assert.Equal(t, 4, list[0].Result.Pushed)
		assert.Equal(t, "rejected", list[0].Result.Failed[0].Reason)
		assert.Equal(t, 2, list[2].Result.Pushed)
	}
}
