// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"testing"

	"gitea.dev/models/organization"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/structs"
	org_service "gitea.dev/services/org"

	"github.com/stretchr/testify/require"
)

func TestDeleteUserPurgeWithSubgroupHierarchy(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// Create a dedicated sole owner whose root org sorts alphabetically BEFORE its child & grandchild subgroups.
	purgeOwner := &user_model.User{
		Name:     "purge-hierarchy-owner",
		Email:    "purge-hierarchy-owner@example.com",
		Passwd:   "password",
		IsActive: true,
	}
	require.NoError(t, user_model.CreateUser(ctx, purgeOwner, &user_model.Meta{}, &user_model.CreateUserOverwriteOptions{
		AllowCreateOrganization: optional.Some(true),
	}))

	rootOrg := &organization.Organization{
		Name:       "acme-purge",
		Visibility: structs.VisibleTypePublic,
		IsActive:   true,
	}
	require.NoError(t, organization.CreateOrganization(ctx, rootOrg, purgeOwner))

	midSub := &organization.Organization{
		Name:       "acme-purge-infra",
		Visibility: structs.VisibleTypeLimited,
		IsActive:   true,
	}
	require.NoError(t, org_service.CreateSubgroup(ctx, rootOrg, midSub, purgeOwner))

	leafSub := &organization.Organization{
		Name:       "acme-purge-vault",
		Visibility: structs.VisibleTypePrivate,
		IsActive:   true,
	}
	require.NoError(t, org_service.CreateSubgroup(ctx, midSub, leafSub, purgeOwner))

	require.NoError(t, DeleteUser(ctx, purgeOwner, true))

	unittest.AssertNotExistsBean(t, &user_model.User{ID: purgeOwner.ID})
	unittest.AssertNotExistsBean(t, &organization.Organization{ID: leafSub.ID})
	unittest.AssertNotExistsBean(t, &organization.Organization{ID: midSub.ID})
	unittest.AssertNotExistsBean(t, &organization.Organization{ID: rootOrg.ID})
	unittest.CheckConsistencyFor(t, &user_model.User{}, &organization.Team{})
}
