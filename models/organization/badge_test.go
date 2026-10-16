// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization_test

import (
	"testing"

	"gitea.dev/models/badges"
	"gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestOrganizationBadges(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	badge := &badges.Badge{Slug: "org-test-label", Description: "Organization test label"}
	assert.NoError(t, badges.CreateBadge(t.Context(), badge))

	assert.NoError(t, organization.AddOrgBadge(t.Context(), org, badge))
	err := organization.AddOrgBadge(t.Context(), org, badge)
	assert.ErrorIs(t, err, util.ErrAlreadyExist)

	got, err := organization.GetOrgBadges(t.Context(), org)
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, badge.Slug, got[0].Slug)

	assert.NoError(t, organization.RemoveOrgBadge(t.Context(), org, badge))
	got, err = organization.GetOrgBadges(t.Context(), org)
	assert.NoError(t, err)
	assert.Empty(t, got)
}

func TestDeleteBadgeRemovesAllAssignments(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	badge := &badges.Badge{Slug: "delete-all-assignments", Description: "Delete assignment test"}
	assert.NoError(t, badges.CreateBadge(t.Context(), badge))

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	assert.NoError(t, repo_model.AddRepoBadge(t.Context(), repo, badge))
	assert.NoError(t, organization.AddOrgBadge(t.Context(), org, badge))
	assert.NoError(t, user_model.AddUserBadge(t.Context(), user, badge))

	assert.NoError(t, badges.DeleteBadge(t.Context(), badge))

	unittest.AssertNotExistsBean(t, &repo_model.RepoBadge{RepoID: repo.ID, BadgeID: badge.ID})
	unittest.AssertNotExistsBean(t, &organization.OrgBadge{OrgID: org.ID, BadgeID: badge.ID})
	unittest.AssertNotExistsBean(t, &user_model.UserBadge{UserID: user.ID, BadgeID: badge.ID})
}
