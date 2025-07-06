// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import (
	"strconv"
	"testing"

	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/assert"
)

func Test_UpdateBadgeColName(t *testing.T) {
	type Badge struct {
		ID          int64 `xorm:"pk autoincr"`
		Description string
		ImageURL    string
	}

	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(Badge))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	oldBadges := []*Badge{
		{Description: "Test Badge 1", ImageURL: "https://example.com/badge1.png"},
		{Description: "Test Badge 2", ImageURL: "https://example.com/badge2.png"},
		{Description: "Test Badge 3", ImageURL: "https://example.com/badge3.png"},
	}

	for _, badge := range oldBadges {
		_, err := x.Insert(badge)
		assert.NoError(t, err)
	}

	if err := UseSlugInsteadOfIDForBadges(x); err != nil {
		assert.NoError(t, err)
		return
	}

	got := []BadgeUnique{}
	if err := x.Table("badge").Asc("id").Find(&got); !assert.NoError(t, err) {
		return
	}

	for i, e := range oldBadges {
		got := got[i+1] // 1 is in the badge.yml
		assert.Equal(t, e.ID, got.ID)
		assert.Equal(t, strconv.FormatInt(e.ID, 10), got.Slug)
	}

	// TODO: check if badges have been updated
}
