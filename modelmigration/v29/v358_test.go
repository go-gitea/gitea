// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
)

func TestAddRepositoryAndOrganizationBadges(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(OrgBadge), new(RepoBadge))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	assert.NoError(t, AddRepositoryAndOrganizationBadges(t.Context(), x))
	assert.NoError(t, x.Sync(new(OrgBadge), new(RepoBadge)))

	_, err := x.Insert(&OrgBadge{BadgeID: 1, OrgID: 1})
	assert.NoError(t, err)
	_, err = x.Insert(&OrgBadge{BadgeID: 1, OrgID: 1})
	assert.Error(t, err)
	_, err = x.Insert(&RepoBadge{BadgeID: 1, RepoID: 1})
	assert.NoError(t, err)
	_, err = x.Insert(&RepoBadge{BadgeID: 1, RepoID: 1})
	assert.Error(t, err)
}

type BadgeBeforeColor struct {
	ID          int64  `xorm:"pk autoincr"`
	Slug        string `xorm:"UNIQUE"`
	Description string
}

func (BadgeBeforeColor) TableName() string {
	return "badge"
}

func TestAddColorToBadges(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(BadgeBeforeColor))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	assert.NoError(t, AddColorToBadges(t.Context(), x))
	assert.NoError(t, x.Sync(new(BadgeBeforeColor)))

	_, err := x.Exec("INSERT INTO badge (slug, description, color) VALUES (?, ?, ?)", "trusted-project", "Trusted project", "#6e7781")
	assert.NoError(t, err)
}
