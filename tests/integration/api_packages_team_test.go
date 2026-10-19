// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	"gitea.dev/models/packages"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	container_module "gitea.dev/modules/packages/container"
	api "gitea.dev/modules/structs"
	org_service "gitea.dev/services/org"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeamPackages(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2}) // team1 in org3
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})  // org owner
	member := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}) // member of team1
	nonMember := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	packageName := "team-generic-pkg"
	packageVersion := "1.0.0"
	filename := "file.bin"
	content := []byte("team-package-content")

	uploadURL := fmt.Sprintf("/api/packages/%s/teams/%s/generic/%s/%s/%s", org.Name, team.Name, packageName, packageVersion, filename)
	downloadURL := uploadURL

	t.Run("RegistryAccessAndPermissions", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// Non-member cannot upload
		req := NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader(content)).
			AddBasicAuth(nonMember.Name)
		MakeRequest(t, req, http.StatusUnauthorized)

		// Team member can upload
		req = NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader(content)).
			AddBasicAuth(member.Name)
		MakeRequest(t, req, http.StatusCreated)

		// Duplicate upload conflicts
		req = NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader(content)).
			AddBasicAuth(member.Name)
		MakeRequest(t, req, http.StatusConflict)

		// Team member can download
		req = NewRequest(t, "GET", downloadURL).
			AddBasicAuth(member.Name)
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, content, resp.Body.Bytes())

		// Org owner can download
		req = NewRequest(t, "GET", downloadURL).
			AddBasicAuth(owner.Name)
		resp = MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, content, resp.Body.Bytes())

		// Database model check: package is scoped to team.ID
		tp, err := packages.GetTeamPackageByName(t.Context(), org.ID, team.ID, packages.TypeGeneric, packageName)
		require.NoError(t, err)
		assert.Equal(t, org.ID, tp.OwnerID)
		assert.Equal(t, team.ID, tp.TeamID)

		// Cannot find at org level (team_id = 0)
		_, err = packages.GetPackageByName(t.Context(), org.ID, packages.TypeGeneric, packageName)
		assert.ErrorIs(t, err, packages.ErrPackageNotExist)
	})

	t.Run("TeamVisibilityPermissions", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		ownerSession := loginUser(t, owner.Name)
		adminToken := getTokenForLoggedInUser(t, ownerSession, auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeWritePackage)

		// Create private team
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams", org.Name), &api.CreateTeamOption{
			Name:                    "private-team",
			Permission:              api.RepoWritePermission("write"),
			IncludesAllRepositories: false,
		}).AddTokenAuth(adminToken)
		resp := MakeRequest(t, req, http.StatusCreated)
		privateTeam := DecodeJSON(t, resp, &api.Team{})

		// Update visibility to private
		privateTeamBean := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: privateTeam.ID})
		privateTeamBean.Visibility = api.VisibleTypePrivate
		_, err := db.GetEngine(t.Context()).ID(privateTeamBean.ID).Cols("visibility").Update(privateTeamBean)
		require.NoError(t, err)

		// Upload package to private team
		req = NewRequestWithBody(t, "PUT", fmt.Sprintf("/api/packages/%s/teams/%s/generic/private-pkg/1.0.0/test.bin", org.Name, privateTeam.Name), bytes.NewReader([]byte("secret"))).
			AddBasicAuth(owner.Name)
		MakeRequest(t, req, http.StatusCreated)

		// Non-member of org cannot download (401)
		req = NewRequest(t, "GET", fmt.Sprintf("/api/packages/%s/teams/%s/generic/private-pkg/1.0.0/test.bin", org.Name, privateTeam.Name)).
			AddBasicAuth(nonMember.Name)
		MakeRequest(t, req, http.StatusUnauthorized)

		// Org member (member is in team1, NOT in private-team) cannot download
		req = NewRequest(t, "GET", fmt.Sprintf("/api/packages/%s/teams/%s/generic/private-pkg/1.0.0/test.bin", org.Name, privateTeam.Name)).
			AddBasicAuth(member.Name)
		MakeRequest(t, req, http.StatusUnauthorized)

		// Create limited team
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams", org.Name), &api.CreateTeamOption{
			Name:                    "limited-team",
			Permission:              api.RepoWritePermission("write"),
			IncludesAllRepositories: false,
		}).AddTokenAuth(adminToken)
		resp = MakeRequest(t, req, http.StatusCreated)
		limitedTeam := DecodeJSON(t, resp, &api.Team{})

		limitedTeamBean := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: limitedTeam.ID})
		limitedTeamBean.Visibility = api.VisibleTypeLimited
		_, err = db.GetEngine(t.Context()).ID(limitedTeamBean.ID).Cols("visibility").Update(limitedTeamBean)
		require.NoError(t, err)

		// Upload package to limited team
		req = NewRequestWithBody(t, "PUT", fmt.Sprintf("/api/packages/%s/teams/%s/generic/limited-pkg/1.0.0/test.bin", org.Name, limitedTeam.Name), bytes.NewReader([]byte("limited"))).
			AddBasicAuth(owner.Name)
		MakeRequest(t, req, http.StatusCreated)

		// Non-member of org cannot download
		req = NewRequest(t, "GET", fmt.Sprintf("/api/packages/%s/teams/%s/generic/limited-pkg/1.0.0/test.bin", org.Name, limitedTeam.Name)).
			AddBasicAuth(nonMember.Name)
		MakeRequest(t, req, http.StatusUnauthorized)

		// Org member (member is in org3) CAN download
		req = NewRequest(t, "GET", fmt.Sprintf("/api/packages/%s/teams/%s/generic/limited-pkg/1.0.0/test.bin", org.Name, limitedTeam.Name)).
			AddBasicAuth(member.Name)
		resp = MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, []byte("limited"), resp.Body.Bytes())
	})

	t.Run("APIv1", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		ownerSession := loginUser(t, owner.Name)
		ownerToken := getTokenForLoggedInUser(t, ownerSession, auth_model.AccessTokenScopeReadPackage)

		// List team packages via API v1
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/packages/%s/teams/%s", org.Name, team.Name)).
			AddTokenAuth(ownerToken)
		resp := MakeRequest(t, req, http.StatusOK)
		apiPackages := DecodeJSON(t, resp, []*api.Package{})
		require.Len(t, apiPackages, 1)
		assert.Equal(t, packageName, apiPackages[0].Name)
		assert.Equal(t, packageVersion, apiPackages[0].Version)

		// Org-level packages does not list the team package
		req = NewRequest(t, "GET", "/api/v1/packages/"+org.Name).
			AddTokenAuth(ownerToken)
		resp = MakeRequest(t, req, http.StatusOK)
		orgPackages := DecodeJSON(t, resp, []*api.Package{})
		assert.Empty(t, orgPackages)

		// Get package via team API
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/packages/%s/teams/%s/generic/%s/%s", org.Name, team.Name, packageName, packageVersion)).
			AddTokenAuth(ownerToken)
		MakeRequest(t, req, http.StatusOK)

		// Get package via org API returns 404
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/packages/%s/generic/%s/%s", org.Name, packageName, packageVersion)).
			AddTokenAuth(ownerToken)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("WebUI", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		session := loginUser(t, member.Name)

		// Team packages tab list
		req := NewRequest(t, "GET", fmt.Sprintf("/org/%s/teams/%s/packages", org.Name, team.Name))
		resp := session.MakeRequest(t, req, http.StatusOK)
		assert.Contains(t, resp.Body.String(), packageName)

		// Team package details page
		req = NewRequest(t, "GET", fmt.Sprintf("/org/%s/teams/%s/packages/generic/%s/%s", org.Name, team.Name, packageName, packageVersion))
		resp = session.MakeRequest(t, req, http.StatusOK)
		assert.Contains(t, resp.Body.String(), packageName)

		// Org packages list must NOT show team package
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/-/packages", org.Name))
		resp = session.MakeRequest(t, req, http.StatusOK)
		assert.NotContains(t, resp.Body.String(), packageName)
	})

	t.Run("ContainerRegistry", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		blobDigest := "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
		blobContent, _ := base64.StdEncoding.DecodeString(`H4sIAAAJbogA/2IYBaNgFIxYAAgAAP//Lq+17wAEAAA=`)
		image := "teamimage"

		url := fmt.Sprintf("/v2/%s/teams/%s/%s", org.Name, team.Name, image)

		// Non-member cannot upload blob
		req := NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", url, blobDigest), bytes.NewReader(blobContent)).
			AddBasicAuth(nonMember.Name)
		MakeRequest(t, req, http.StatusUnauthorized)

		// Team member can upload blob
		req = NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", url, blobDigest), bytes.NewReader(blobContent)).
			AddBasicAuth(member.Name)
		resp := MakeRequest(t, req, http.StatusCreated)
		assert.Equal(t, fmt.Sprintf("/v2/%s/teams/%s/%s/blobs/%s", org.Name, team.Name, image, blobDigest), resp.Header().Get("Location"))

		// Verify internal upload version exists in database for this team
		pv, err := packages.GetTeamInternalVersionByNameAndVersion(t.Context(), org.ID, team.ID, packages.TypeContainer, image, container_module.UploadVersion)
		require.NoError(t, err)
		assert.NotNil(t, pv)
	})

	t.Run("TeamDeletionReparentPackages", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		ownerSession := loginUser(t, owner.Name)
		adminToken := getTokenForLoggedInUser(t, ownerSession, auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeWritePackage)

		// Create a new team
		teamName := "reparent-team"
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams", org.Name), &api.CreateTeamOption{
			Name:                    teamName,
			Permission:              api.RepoWritePermission("write"),
			IncludesAllRepositories: false,
		}).AddTokenAuth(adminToken)
		resp := MakeRequest(t, req, http.StatusCreated)
		newTeam := DecodeJSON(t, resp, &api.Team{})

		// Upload package to new team
		pkgName := "reparented-pkg"
		pkgVersion := "2.0.0"
		uploadURL := fmt.Sprintf("/api/packages/%s/teams/%s/generic/%s/%s/test.bin", org.Name, teamName, pkgName, pkgVersion)
		req = NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader([]byte("test"))).
			AddBasicAuth(owner.Name)
		MakeRequest(t, req, http.StatusCreated)

		// Verify package exists under team
		_, err := packages.GetTeamPackageByName(t.Context(), org.ID, newTeam.ID, packages.TypeGeneric, pkgName)
		require.NoError(t, err)

		// Delete team with delete_packages=false (default)
		req = NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/teams/%d?delete_packages=false", newTeam.ID)).
			AddTokenAuth(adminToken)
		MakeRequest(t, req, http.StatusNoContent)

		// Team bean should be gone
		unittest.AssertNotExistsBean(t, &organization.Team{ID: newTeam.ID})

		// Package should now be reparented to org (TeamID = 0)
		reparentedPkg, err := packages.GetPackageByName(t.Context(), org.ID, packages.TypeGeneric, pkgName)
		require.NoError(t, err)
		assert.Equal(t, int64(0), reparentedPkg.TeamID)
		assert.Equal(t, org.ID, reparentedPkg.OwnerID)

		// Org packages list via API should now show the reparented package
		req = NewRequest(t, "GET", "/api/v1/packages/"+org.Name).
			AddTokenAuth(adminToken)
		resp = MakeRequest(t, req, http.StatusOK)
		orgPackages := DecodeJSON(t, resp, []*api.Package{})
		require.Len(t, orgPackages, 1)
		assert.Equal(t, pkgName, orgPackages[0].Name)

		// Test merging versions when org already owns the package with a different version
		mergeTeamName := "merge-team"
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams", org.Name), &api.CreateTeamOption{
			Name:                    mergeTeamName,
			Permission:              api.RepoWritePermission("write"),
			IncludesAllRepositories: false,
		}).AddTokenAuth(adminToken)
		resp = MakeRequest(t, req, http.StatusCreated)
		mergeTeam := DecodeJSON(t, resp, &api.Team{})

		// Upload v1.0.0 directly to org
		req = NewRequestWithBody(t, "PUT", fmt.Sprintf("/api/packages/%s/generic/shared-pkg/1.0.0/test.bin", org.Name), bytes.NewReader([]byte("v1"))).
			AddBasicAuth(owner.Name)
		MakeRequest(t, req, http.StatusCreated)

		// Upload v2.0.0 to merge-team
		req = NewRequestWithBody(t, "PUT", fmt.Sprintf("/api/packages/%s/teams/%s/generic/shared-pkg/2.0.0/test.bin", org.Name, mergeTeamName), bytes.NewReader([]byte("v2"))).
			AddBasicAuth(owner.Name)
		MakeRequest(t, req, http.StatusCreated)

		// Delete merge-team: should merge v2.0.0 into org's existing shared-pkg
		req = NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/teams/%d?delete_packages=false", mergeTeam.ID)).
			AddTokenAuth(adminToken)
		MakeRequest(t, req, http.StatusNoContent)

		_, err = packages.GetVersionByNameAndVersion(t.Context(), org.ID, packages.TypeGeneric, "shared-pkg", "1.0.0")
		require.NoError(t, err)
		_, err = packages.GetVersionByNameAndVersion(t.Context(), org.ID, packages.TypeGeneric, "shared-pkg", "2.0.0")
		require.NoError(t, err)

		// Test version collision rollback: create conflict-team with version 1.0.0 of shared-pkg
		conflictTeamName := "conflict-team"
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams", org.Name), &api.CreateTeamOption{
			Name:                    conflictTeamName,
			Permission:              api.RepoWritePermission("write"),
			IncludesAllRepositories: false,
		}).AddTokenAuth(adminToken)
		resp = MakeRequest(t, req, http.StatusCreated)
		conflictTeam := DecodeJSON(t, resp, &api.Team{})

		req = NewRequestWithBody(t, "PUT", fmt.Sprintf("/api/packages/%s/teams/%s/generic/shared-pkg/1.0.0/test.bin", org.Name, conflictTeamName), bytes.NewReader([]byte("collision"))).
			AddBasicAuth(owner.Name)
		MakeRequest(t, req, http.StatusCreated)

		// Deleting conflict-team with delete_packages=false must fail because of version collision
		req = NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/teams/%d?delete_packages=false", conflictTeam.ID)).
			AddTokenAuth(adminToken)
		MakeRequest(t, req, http.StatusInternalServerError)

		// Team and team package still exist intact due to atomic rollback
		unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: conflictTeam.ID})
		_, err = packages.GetTeamVersionByNameAndVersion(t.Context(), org.ID, conflictTeam.ID, packages.TypeGeneric, "shared-pkg", "1.0.0")
		require.NoError(t, err)
	})

	t.Run("TeamDeletionDeletePackages", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		ownerSession := loginUser(t, owner.Name)
		adminToken := getTokenForLoggedInUser(t, ownerSession, auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeWritePackage)

		// Create another team
		teamName := "purge-team"
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams", org.Name), &api.CreateTeamOption{
			Name:                    teamName,
			Permission:              api.RepoWritePermission("write"),
			IncludesAllRepositories: false,
		}).AddTokenAuth(adminToken)
		resp := MakeRequest(t, req, http.StatusCreated)
		purgeTeam := DecodeJSON(t, resp, &api.Team{})

		// Upload package to purge team
		pkgName := "purged-pkg"
		pkgVersion := "3.0.0"
		uploadURL := fmt.Sprintf("/api/packages/%s/teams/%s/generic/%s/%s/test.bin", org.Name, teamName, pkgName, pkgVersion)
		req = NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader([]byte("test"))).
			AddBasicAuth(owner.Name)
		MakeRequest(t, req, http.StatusCreated)

		// Verify package exists under purge team
		_, err := packages.GetTeamPackageByName(t.Context(), org.ID, purgeTeam.ID, packages.TypeGeneric, pkgName)
		require.NoError(t, err)

		// Delete team with delete_packages=true
		req = NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/teams/%d?delete_packages=true", purgeTeam.ID)).
			AddTokenAuth(adminToken)
		MakeRequest(t, req, http.StatusNoContent)

		// Team bean should be gone
		unittest.AssertNotExistsBean(t, &organization.Team{ID: purgeTeam.ID})

		// Package should be deleted (not found under team or org)
		_, err = packages.GetTeamPackageByName(t.Context(), org.ID, purgeTeam.ID, packages.TypeGeneric, pkgName)
		assert.ErrorIs(t, err, packages.ErrPackageNotExist)

		_, err = packages.GetPackageByName(t.Context(), org.ID, packages.TypeGeneric, pkgName)
		assert.ErrorIs(t, err, packages.ErrPackageNotExist)
	})

	t.Run("OrgDeletionBlockedByTeamPackages", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// org3 currently has team packages (team1 has team-generic-pkg)
		err := org_service.DeleteOrganization(t.Context(), org, true)
		assert.ErrorIs(t, err, packages.ErrUserOwnPackages{UID: org.ID})
	})
}
