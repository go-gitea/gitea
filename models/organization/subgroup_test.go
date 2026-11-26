// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization_test

import (
	"fmt"
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationSubgroupsModel(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	rootOrg := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3}) // "org3", owned by user2
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	// 1. Non-owner cannot create a subgroup under org3.
	unauthorized := &organization.Organization{
		Name:        "org3-unauth",
		ParentOrgID: rootOrg.ID,
		Visibility:  structs.VisibleTypePublic,
	}
	err := organization.CreateOrganization(ctx, unauthorized, user5)
	require.True(t, organization.IsErrUserNotAllowedCreateOrg(err))

	// 2. Non-organization user ID cannot be used as ParentOrgID.
	badParent := &organization.Organization{
		Name:        "bad-parent-sub",
		ParentOrgID: user2.ID,
		Visibility:  structs.VisibleTypePublic,
	}
	err = organization.CreateOrganization(ctx, badParent, user2)
	require.True(t, organization.IsErrInvalidParentOrg(err))

	// 3. Create multi-level hierarchy: org3 -> org3-platform (limited) -> org3-payments (private).
	platform := &organization.Organization{
		Name:        "org3-platform",
		FullName:    "Platform Engineering",
		ParentOrgID: rootOrg.ID,
		Visibility:  structs.VisibleTypeLimited,
		IsActive:    true,
	}
	require.NoError(t, organization.CreateOrganization(ctx, platform, user2))
	assert.True(t, platform.IsSubgroup())

	// Add user4 to platform's Owners team; user4 (not a direct owner of org3) creates a private child subgroup,
	// while user2 (root owner of org3) retains cascading owner rights over grandchild!
	platformOwnerTeam, err := platform.GetOwnerTeam(ctx)
	require.NoError(t, err)
	require.NoError(t, organization.AddOrgUser(ctx, platform.ID, user4.ID))
	_, err = db.GetEngine(ctx).Insert(&organization.TeamUser{
		UID:    user4.ID,
		OrgID:  platform.ID,
		TeamID: platformOwnerTeam.ID,
	})
	require.NoError(t, err)
	_, err = db.GetEngine(ctx).ID(platformOwnerTeam.ID).Incr("num_members").Update(new(organization.Team))
	require.NoError(t, err)

	// Child subgroup cannot be more permissive (public) than limited parent.
	tooPublic := &organization.Organization{
		Name:        "org3-too-public",
		ParentOrgID: platform.ID,
		Visibility:  structs.VisibleTypePublic,
	}
	err = organization.CreateOrganization(ctx, tooPublic, user4)
	require.True(t, organization.IsErrSubgroupVisibilityExceedsParent(err))

	payments := &organization.Organization{
		Name:        "org3-payments",
		FullName:    "Payments Core",
		ParentOrgID: platform.ID,
		Visibility:  structs.VisibleTypePrivate,
		IsActive:    true,
	}
	require.NoError(t, organization.CreateOrganization(ctx, payments, user4))

	// Verify FullOrgPath & GetAncestorOrgs.
	fullPath, err := payments.FullOrgPath(ctx)
	require.NoError(t, err)
	assert.Equal(t, "org3/org3-platform/org3-payments", fullPath)

	ancestors, err := organization.GetAncestorOrgs(ctx, payments.ID)
	require.NoError(t, err)
	require.Len(t, ancestors, 2)
	assert.Equal(t, rootOrg.ID, ancestors[0].ID)
	assert.Equal(t, platform.ID, ancestors[1].ID)

	// Verify cascading ancestor ownership for user2 (owner of root org3, not direct member of payments).
	isDirectMember, err := payments.IsOrgMember(ctx, user2.ID)
	require.NoError(t, err)
	assert.False(t, isDirectMember)

	isCascadingOwner, err := payments.IsOwnedBy(ctx, user2.ID)
	require.NoError(t, err)
	assert.True(t, isCascadingOwner)

	maxAuth, err := payments.GetOrgUserMaxAuthorizeLevel(ctx, user2.ID)
	require.NoError(t, err)
	assert.Equal(t, perm.AccessModeOwner, maxAuth)

	canCreateRepo, err := payments.CanCreateOrgRepo(ctx, user2.ID)
	require.NoError(t, err)
	assert.True(t, canCreateRepo)

	// Private grandchild subgroup is visible to root owner user2 and direct owner user4, but hidden from user5 and anon.
	assert.True(t, organization.HasOrgOrUserVisible(ctx, payments.AsUser(), user2))
	assert.True(t, organization.HasOrgOrUserVisible(ctx, payments.AsUser(), user4))
	assert.False(t, organization.HasOrgOrUserVisible(ctx, payments.AsUser(), user5))
	assert.False(t, organization.HasOrgOrUserVisible(ctx, payments.AsUser(), nil))

	// Verify GetSubgroups & GetDescendantOrgs visibility filtering.
	anonSubs, anonTotal, err := organization.GetSubgroups(ctx, rootOrg.ID, nil, db.ListOptions{})
	require.NoError(t, err)
	assert.Empty(t, anonSubs)
	assert.Equal(t, int64(0), anonTotal)

	ownerDesc, err := organization.GetDescendantOrgs(ctx, rootOrg.ID, user2)
	require.NoError(t, err)
	require.Len(t, ownerDesc, 2)
	assert.Equal(t, platform.ID, ownerDesc[0].ID)
	assert.Equal(t, payments.ID, ownerDesc[1].ID)

	// 4. Enforce MaxSubgroupDepth (root=0, d1=1, d2=2, d3=3, d4=4, d5=5 -> d6 must fail).
	curParent := payments // already at depth 2
	for depth := 3; depth <= organization.MaxSubgroupDepth; depth++ {
		next := &organization.Organization{
			Name:        fmt.Sprintf("org3-depth-%d", depth),
			ParentOrgID: curParent.ID,
			Visibility:  structs.VisibleTypePrivate,
			IsActive:    true,
		}
		require.NoError(t, organization.CreateOrganization(ctx, next, user2))
		curParent = next
	}

	beyondMax := &organization.Organization{
		Name:        "org3-depth-overflow",
		ParentOrgID: curParent.ID,
		Visibility:  structs.VisibleTypePrivate,
		IsActive:    true,
	}
	err = organization.CreateOrganization(ctx, beyondMax, user2)
	require.True(t, organization.IsErrSubgroupMaxDepthExceeded(err))

	// Self-parenting validation check.
	_, err = organization.ValidateParentOrg(ctx, platform.ID, platform.ID, platform.Visibility)
	require.True(t, organization.IsErrSubgroupCycle(err))

	// Cycle validation check (trying to place rootOrg under descendant payments).
	_, err = organization.ValidateParentOrg(ctx, rootOrg.ID, payments.ID, structs.VisibleTypePrivate)
	require.True(t, organization.IsErrSubgroupCycle(err))

	unittest.CheckConsistencyFor(t, &user_model.User{}, &organization.Team{})
}
