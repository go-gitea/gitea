// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization_test

import (
	"testing"

	"gitea.dev/models/badges"
	"gitea.dev/models/organization"
	"gitea.dev/models/unittest"
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
