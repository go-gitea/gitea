// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"sync"
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubgroupServiceLifecycleAndPermissions(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	rootOrg := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3}) // owned by user2
	otherRoot := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 6})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	// Add user2 as owner of otherRoot (org6) so we can test reparenting across two root orgs.
	org6OwnerTeam, err := otherRoot.GetOwnerTeam(ctx)
	require.NoError(t, err)
	require.NoError(t, organization.AddOrgUser(ctx, otherRoot.ID, user2.ID))
	_, err = db.GetEngine(ctx).Insert(&organization.TeamUser{
		UID:    user2.ID,
		OrgID:  otherRoot.ID,
		TeamID: org6OwnerTeam.ID,
	})
	require.NoError(t, err)
	_, err = db.GetEngine(ctx).ID(org6OwnerTeam.ID).Incr("num_members").Update(new(organization.Team))
	require.NoError(t, err)

	// 1. Create child & grandchild subgroups: org3 -> sg-infra (public) -> sg-sec (limited).
	sgInfra := &organization.Organization{
		Name:       "sg-infra",
		Visibility: structs.VisibleTypePublic,
		IsActive:   true,
	}
	require.NoError(t, CreateSubgroup(ctx, rootOrg, sgInfra, user2))

	// Make user4 direct owner of sg-infra so user4 creates sg-sec (where user2 is NOT a direct member).
	infraOwnerTeam, err := sgInfra.GetOwnerTeam(ctx)
	require.NoError(t, err)
	require.NoError(t, organization.AddOrgUser(ctx, sgInfra.ID, user4.ID))
	_, err = db.GetEngine(ctx).Insert(&organization.TeamUser{
		UID:    user4.ID,
		OrgID:  sgInfra.ID,
		TeamID: infraOwnerTeam.ID,
	})
	require.NoError(t, err)
	_, err = db.GetEngine(ctx).ID(infraOwnerTeam.ID).Incr("num_members").Update(new(organization.Team))
	require.NoError(t, err)

	sgSec := &organization.Organization{
		Name:       "sg-sec",
		Visibility: structs.VisibleTypeLimited,
		IsActive:   true,
	}
	require.NoError(t, CreateSubgroup(ctx, sgInfra, sgSec, user4))

	// Insert a private repository owned by grandchild subgroup sg-sec and recalculate accesses.
	secRepo := &repo_model.Repository{
		OwnerID:   sgSec.ID,
		OwnerName: sgSec.Name,
		LowerName: "vault-config",
		Name:      "vault-config",
		IsPrivate: true,
	}
	require.NoError(t, db.Insert(ctx, secRepo))
	require.NoError(t, db.Insert(ctx, &repo_model.RepoUnit{
		RepoID: secRepo.ID,
		Type:   unit.TypeCode,
	}))
	_, err = db.GetEngine(ctx).ID(sgSec.ID).Incr("num_repos").Update(new(user_model.User))
	require.NoError(t, err)
	require.NoError(t, secRepo.LoadOwner(ctx))
	require.NoError(t, access_model.RecalculateAccesses(ctx, secRepo))

	// Root org owner user2 inherits Owner access on grandchild subgroup's private repo!
	lvl, err := access_model.AccessLevel(ctx, user2, secRepo)
	require.NoError(t, err)
	assert.Equal(t, perm.AccessModeOwner, lvl)

	// 2. DeleteOrganization must refuse to delete sgInfra while child sgSec exists.
	err = DeleteOrganization(ctx, sgInfra, false)
	require.True(t, organization.IsErrOrgHasSubgroups(err))

	// 3. Visibility cascade: making sgInfra private cascades Private visibility to child sgSec,
	// and sgSec cannot be made Limited while sgInfra is Private.
	require.NoError(t, ChangeOrganizationVisibility(ctx, sgInfra, structs.VisibleTypePrivate))
	reloadedSec := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: sgSec.ID})
	assert.Equal(t, structs.VisibleTypePrivate, reloadedSec.Visibility)

	err = ChangeOrganizationVisibility(ctx, reloadedSec, structs.VisibleTypeLimited)
	require.True(t, organization.IsErrSubgroupVisibilityExceedsParent(err))

	// 4. MoveSubgroup cycle guard: cannot move sgInfra under its own child reloadedSec.
	err = MoveSubgroup(ctx, sgInfra, reloadedSec, user2)
	require.True(t, organization.IsErrSubgroupCycle(err))

	// Move sgInfra (along with child sgSec) under otherRoot (org6).
	require.NoError(t, MoveSubgroup(ctx, sgInfra, otherRoot, user2))
	pathAfterMove, err := reloadedSec.FullOrgPath(ctx)
	require.NoError(t, err)
	assert.Equal(t, "org6/sg-infra/sg-sec", pathAfterMove)

	// Detach sgInfra to a top-level organization.
	require.NoError(t, MoveSubgroup(ctx, sgInfra, nil, user2))
	assert.Equal(t, int64(0), sgInfra.ParentOrgID)
	pathAfterDetach, err := reloadedSec.FullOrgPath(ctx)
	require.NoError(t, err)
	assert.Equal(t, "sg-infra/sg-sec", pathAfterDetach)

	// Clean up repo and leaf subgroup, then verify parent subgroup deletes cleanly.
	_, err = db.GetEngine(ctx).Where("repo_id = ?", secRepo.ID).Delete(new(repo_model.RepoUnit))
	require.NoError(t, err)
	_, err = db.GetEngine(ctx).ID(secRepo.ID).Delete(new(repo_model.Repository))
	require.NoError(t, err)
	_, err = db.GetEngine(ctx).Where("repo_id = ?", secRepo.ID).Delete(new(access_model.Access))
	require.NoError(t, err)
	_, err = db.GetEngine(ctx).ID(reloadedSec.ID).Decr("num_repos").Update(new(user_model.User))
	require.NoError(t, err)

	require.NoError(t, DeleteOrganization(ctx, reloadedSec, false))
	require.NoError(t, DeleteOrganization(ctx, sgInfra, false))
	unittest.CheckConsistencyFor(t, &user_model.User{}, &organization.Team{})
}

func TestRevokeInheritedSubgroupRepoAccessOnOwnerRemoval(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	rootOrg := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	// Create a limited subgroup under org3 containing a private repository.
	sgLimited := &organization.Organization{
		Name:       "sg-revoke-sub",
		Visibility: structs.VisibleTypeLimited,
		IsActive:   true,
	}
	require.NoError(t, CreateSubgroup(ctx, rootOrg, sgLimited, user2))

	privRepo := &repo_model.Repository{
		OwnerID:   sgLimited.ID,
		OwnerName: sgLimited.Name,
		LowerName: "private-ledger",
		Name:      "private-ledger",
		IsPrivate: true,
	}
	require.NoError(t, db.Insert(ctx, privRepo))
	require.NoError(t, db.Insert(ctx, &repo_model.RepoUnit{
		RepoID: privRepo.ID,
		Type:   unit.TypeCode,
	}))
	_, err := db.GetEngine(ctx).ID(sgLimited.ID).Incr("num_repos").Update(new(user_model.User))
	require.NoError(t, err)
	require.NoError(t, privRepo.LoadOwner(ctx))
	require.NoError(t, access_model.RecalculateAccesses(ctx, privRepo))

	// Add user5 to root org3's Owners team (user5 has no direct membership in sgLimited).
	rootOwnerTeam, err := rootOrg.GetOwnerTeam(ctx)
	require.NoError(t, err)
	require.NoError(t, AddTeamMember(ctx, rootOwnerTeam, user5))

	lvl, err := access_model.AccessLevel(ctx, user5, privRepo)
	require.NoError(t, err)
	assert.Equal(t, perm.AccessModeOwner, lvl)
	unittest.AssertExistsAndLoadBean(t, &access_model.Access{UserID: user5.ID, RepoID: privRepo.ID, Mode: perm.AccessModeOwner})

	require.NoError(t, repo_model.WatchRepoAuto(ctx, user5, privRepo, true))
	unittest.AssertExistsAndLoadBean(t, &repo_model.Watch{UserID: user5.ID, RepoID: privRepo.ID})

	// Removing user5 from root Owners team must immediately revoke cached AccessModeOwner and watch on privRepo.
	require.NoError(t, RemoveTeamMember(ctx, rootOwnerTeam, user5))

	lvlAfterRemove, err := access_model.AccessLevel(ctx, user5, privRepo)
	require.NoError(t, err)
	assert.Equal(t, perm.AccessModeNone, lvlAfterRemove)
	unittest.AssertNotExistsBean(t, &access_model.Access{UserID: user5.ID, RepoID: privRepo.ID})
	unittest.AssertNotExistsBean(t, &repo_model.Watch{UserID: user5.ID, RepoID: privRepo.ID})

	// Re-add user5 to root Owners team, then remove via RemoveOrgUser and verify access is revoked again.
	rootOwnerTeam, err = rootOrg.GetOwnerTeam(ctx)
	require.NoError(t, err)
	require.NoError(t, AddTeamMember(ctx, rootOwnerTeam, user5))
	lvlReadded, err := access_model.AccessLevel(ctx, user5, privRepo)
	require.NoError(t, err)
	assert.Equal(t, perm.AccessModeOwner, lvlReadded)

	require.NoError(t, RemoveOrgUser(ctx, rootOrg, user5))
	lvlAfterOrgRemove, err := access_model.AccessLevel(ctx, user5, privRepo)
	require.NoError(t, err)
	assert.Equal(t, perm.AccessModeNone, lvlAfterOrgRemove)
	unittest.AssertNotExistsBean(t, &access_model.Access{UserID: user5.ID, RepoID: privRepo.ID})
}

func TestConcurrentMoveSubgroupPreventsCycle(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	orgA := &organization.Organization{
		Name:       "conc-org-a",
		Visibility: structs.VisibleTypePublic,
		IsActive:   true,
	}
	orgB := &organization.Organization{
		Name:       "conc-org-b",
		Visibility: structs.VisibleTypePublic,
		IsActive:   true,
	}
	require.NoError(t, organization.CreateOrganization(ctx, orgA, user2))
	require.NoError(t, organization.CreateOrganization(ctx, orgB, user2))

	start := make(chan struct{})
	var wg sync.WaitGroup
	var errAB, errBA error

	wg.Go(func() {
		<-start
		aCopy := *orgA
		bCopy := *orgB
		errAB = MoveSubgroup(ctx, &aCopy, &bCopy, user2)
	})
	wg.Go(func() {
		<-start
		aCopy := *orgA
		bCopy := *orgB
		errBA = MoveSubgroup(ctx, &bCopy, &aCopy, user2)
	})
	close(start)
	wg.Wait()

	// Exactly one move commits and the concurrent opposing move must fail with ErrSubgroupCycle.
	if errAB == nil {
		require.True(t, organization.IsErrSubgroupCycle(errBA), "expected cycle error, got: %v", errBA)
	} else {
		require.NoError(t, errBA)
		require.True(t, organization.IsErrSubgroupCycle(errAB), "expected cycle error, got: %v", errAB)
	}

	_, err := organization.GetAncestorOrgs(ctx, orgA.ID)
	require.NoError(t, err)
	_, err = organization.GetAncestorOrgs(ctx, orgB.ID)
	require.NoError(t, err)
	unittest.CheckConsistencyFor(t, &user_model.User{}, &organization.Team{})
}

func TestChangeOrganizationVisibilityReloadsStaleParentUnderLock(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	privParent := &organization.Organization{
		Name:       "vis-priv-parent",
		Visibility: structs.VisibleTypePrivate,
		IsActive:   true,
	}
	staleChild := &organization.Organization{
		Name:       "vis-stale-child",
		Visibility: structs.VisibleTypePrivate,
		IsActive:   true,
	}
	require.NoError(t, organization.CreateOrganization(ctx, privParent, user2))
	require.NoError(t, organization.CreateOrganization(ctx, staleChild, user2))
	require.Zero(t, staleChild.ParentOrgID)

	// Simulate a concurrent MoveSubgroup reparenting the child under privParent while staleChild still holds ParentOrgID == 0.
	movedCopy := *staleChild
	require.NoError(t, MoveSubgroup(ctx, &movedCopy, privParent, user2))
	assert.Zero(t, staleChild.ParentOrgID)

	err := ChangeOrganizationVisibility(ctx, staleChild, structs.VisibleTypePublic)
	require.Error(t, err)
	assert.True(t, organization.IsErrSubgroupVisibilityExceedsParent(err))

	reloaded, err := organization.GetOrgByID(ctx, staleChild.ID)
	require.NoError(t, err)
	assert.Equal(t, structs.VisibleTypePrivate, reloaded.Visibility)
	assert.Equal(t, privParent.ID, reloaded.ParentOrgID)
	unittest.CheckConsistencyFor(t, &user_model.User{}, &organization.Team{})
}

func TestDeleteOrganizationSerializesWithSubgroupCreation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	targetOrg := &organization.Organization{
		Name:       "race-del-parent",
		Visibility: structs.VisibleTypePublic,
		IsActive:   true,
	}
	require.NoError(t, organization.CreateOrganization(ctx, targetOrg, user2))

	start := make(chan struct{})
	var wg sync.WaitGroup
	var errDel, errCreate error

	wg.Go(func() {
		<-start
		parentCopy := *targetOrg
		errDel = DeleteOrganization(ctx, &parentCopy, false)
	})
	wg.Go(func() {
		<-start
		parentCopy := *targetOrg
		childSub := &organization.Organization{
			Name:       "race-del-child",
			Visibility: structs.VisibleTypePublic,
			IsActive:   true,
		}
		errCreate = CreateSubgroup(ctx, &parentCopy, childSub, user2)
	})
	close(start)
	wg.Wait()

	if errDel == nil {
		require.Error(t, errCreate)
		unittest.AssertNotExistsBean(t, &organization.Organization{ID: targetOrg.ID})
		unittest.AssertNotExistsBean(t, &organization.Organization{Name: "race-del-child"})
	} else {
		require.True(t, organization.IsErrOrgHasSubgroups(errDel), "expected ErrOrgHasSubgroups, got: %v", errDel)
		require.NoError(t, errCreate)
		unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: targetOrg.ID})
	}
	unittest.CheckConsistencyFor(t, &user_model.User{}, &organization.Team{})
}

func TestInheritedOwnerUnitQueriesAndConcurrentRevocation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	rootOrg := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3}) // owned by user2
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	rootOwnerTeam, err := rootOrg.GetOwnerTeam(ctx)
	require.NoError(t, err)
	require.NoError(t, AddTeamMember(ctx, rootOwnerTeam, user4))

	// Create a detached org owned by user2 with a private repo (Code & Actions units enabled), then concurrently move it under rootOrg while removing user4 from rootOwnerTeam.
	subOrg := &organization.Organization{
		Name:       "unit-query-sub",
		Visibility: structs.VisibleTypeLimited,
		IsActive:   true,
	}
	require.NoError(t, organization.CreateOrganization(ctx, subOrg, user2))

	privRepo := &repo_model.Repository{
		OwnerID:   subOrg.ID,
		OwnerName: subOrg.Name,
		Owner:     subOrg.AsUser(),
		Name:      "unit-scoped-secret",
		LowerName: "unit-scoped-secret",
		IsPrivate: true,
	}
	require.NoError(t, db.Insert(ctx, privRepo))
	_, err = db.GetEngine(ctx).ID(subOrg.ID).Incr("num_repos").Update(new(user_model.User))
	require.NoError(t, err)
	require.NoError(t, db.Insert(ctx,
		&repo_model.RepoUnit{RepoID: privRepo.ID, Type: unit.TypeCode},
		&repo_model.RepoUnit{RepoID: privRepo.ID, Type: unit.TypeActions},
		&repo_model.RepoUnit{RepoID: privRepo.ID, Type: unit.TypeProjects},
	))
	require.NoError(t, access_model.RecalculateTeamAccesses(ctx, privRepo, 0))

	start := make(chan struct{})
	var wg sync.WaitGroup
	var errRem, errMove error

	wg.Go(func() {
		<-start
		teamCopy := *rootOwnerTeam
		errRem = RemoveTeamMember(ctx, &teamCopy, user4)
	})
	wg.Go(func() {
		<-start
		subCopy := *subOrg
		errMove = MoveSubgroup(ctx, &subCopy, rootOrg, user2)
	})
	close(start)
	wg.Wait()
	require.NoError(t, errRem)
	require.NoError(t, errMove)

	// Regardless of ordering, user4 was removed from rootOrg Owners and must NOT retain cached AccessModeOwner on privRepo.
	permUser4, err := access_model.GetIndividualUserRepoPermission(ctx, privRepo, user4)
	require.NoError(t, err)
	assert.Equal(t, perm.AccessModeNone, permUser4.AccessMode)
	unittest.AssertNotExistsBean(t, &access_model.Access{UserID: user4.ID, RepoID: privRepo.ID})

	// Now make user4 sole direct owner of subOrg and remove user2 from subOrg's direct membership so user2 is purely an inherited owner.
	subOwnerTeam, err := subOrg.GetOwnerTeam(ctx)
	require.NoError(t, err)
	require.NoError(t, AddTeamMember(ctx, subOwnerTeam, user4))
	require.NoError(t, RemoveOrgUser(ctx, subOrg, user2))

	reloadedSub, err := organization.GetOrgByID(ctx, subOrg.ID)
	require.NoError(t, err)

	// 1. Organization.AnyRepoUnitPermission grants AccessModeOwner to inherited owner user2 (e.g. org projects).
	assert.Equal(t, perm.AccessModeOwner, reloadedSub.AnyRepoUnitPermission(ctx, user2, unit.TypeProjects))

	// 2. Unit-scoped AccessibleRepositoryCondition (Code, Actions) and UserOrgUnitRepoCond include privRepo for inherited owner user2.
	codeIDs, err := repo_model.SearchRepositoryIDsByCondition(ctx, repo_model.AccessibleRepositoryCondition(user2, unit.TypeCode))
	require.NoError(t, err)
	assert.Contains(t, codeIDs, privRepo.ID)

	actionIDs, err := repo_model.SearchRepositoryIDsByCondition(ctx, repo_model.UserActionsAccessibleOwnerRepoCond(subOrg.ID, user2))
	require.NoError(t, err)
	assert.Contains(t, actionIDs, privRepo.ID)

	orgUnitIDs, err := repo_model.SearchRepositoryIDsByCondition(ctx, repo_model.UserOrgUnitRepoCond("`repository`.id", user2.ID, subOrg.ID, unit.TypeIssues))
	require.NoError(t, err)
	assert.Contains(t, orgUnitIDs, privRepo.ID)

	// 3. GetOrgsCanCreateRepoByUserID includes subOrg via inherited ownership.
	creatableOrgs, err := organization.GetOrgsCanCreateRepoByUserID(ctx, user2.ID)
	require.NoError(t, err)
	var creatableOrgIDs []int64
	for _, o := range creatableOrgs {
		creatableOrgIDs = append(creatableOrgIDs, o.ID)
	}
	assert.Contains(t, creatableOrgIDs, subOrg.ID)

	// 4. Adding and then removing inherited owner user2 from a non-owner subgroup team preserves owner grants (including on non-private subgroup repos).
	pubRepo := &repo_model.Repository{
		OwnerID:   subOrg.ID,
		OwnerName: subOrg.Name,
		Owner:     subOrg.AsUser(),
		Name:      "unit-scoped-public",
		LowerName: "unit-scoped-public",
		IsPrivate: false,
	}
	require.NoError(t, db.Insert(ctx, pubRepo))
	_, err = db.GetEngine(ctx).ID(subOrg.ID).Incr("num_repos").Update(new(user_model.User))
	require.NoError(t, err)
	require.NoError(t, db.Insert(ctx, &repo_model.RepoUnit{RepoID: pubRepo.ID, Type: unit.TypeCode}))
	require.NoError(t, access_model.RecalculateTeamAccesses(ctx, pubRepo, 0))

	readTeam := &organization.Team{
		OrgID:      subOrg.ID,
		Name:       "readers-only",
		AccessMode: perm.AccessModeRead,
	}
	require.NoError(t, NewTeam(ctx, readTeam))
	require.NoError(t, AddTeamMember(ctx, readTeam, user2))
	require.NoError(t, RemoveTeamMember(ctx, readTeam, user2))

	unittest.AssertExistsAndLoadBean(t, &access_model.Access{UserID: user2.ID, RepoID: privRepo.ID, Mode: perm.AccessModeOwner})
	unittest.AssertExistsAndLoadBean(t, &access_model.Access{UserID: user2.ID, RepoID: pubRepo.ID, Mode: perm.AccessModeOwner})

	unittest.CheckConsistencyFor(t, &user_model.User{}, &organization.Team{})
}
