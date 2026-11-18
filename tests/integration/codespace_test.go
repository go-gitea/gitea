// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	"gitea.dev/codespace-proto-go/codespace/v1/codespacev1connect"
	auth_model "gitea.dev/models/auth"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	codespace_service "gitea.dev/services/codespace"
	repo_service "gitea.dev/services/repository"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"connectrpc.com/connect/v2/connectproto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestCodespaceRoutes(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		insertIntegrationDevContainerTemplate(t)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/-/codespaces"), http.StatusSeeOther)

		user2Session := loginUser(t, "user2")
		user2Session.MakeRequest(t, NewRequest(t, http.MethodGet, "/user/settings/codespaces/managers"), http.StatusOK)
		user2Session.MakeRequest(t, NewRequest(t, http.MethodGet, "/user/settings/codespaces/dev-container-templates"), http.StatusOK)
		user2Session.MakeRequest(t, NewRequest(t, http.MethodGet, "/user/settings/codespaces/permissions"), http.StatusOK)
		now := time.Now().Unix()
		authorization := &codespace_model.PermissionAuthorization{
			UserID: 2, SourceRepoID: 1, RequestHash: "settings-route",
			CreatedUnix: now, UpdatedUnix: now,
		}
		require.NoError(t, db.Insert(t.Context(), authorization))
		rule := &codespace_model.PermissionRepository{
			AuthorizationID: authorization.ID, TargetRepoID: 2, UnitType: unit.TypeCode,
			RequestedMode: perm.AccessModeWrite, GrantedMode: perm.AccessModeWrite,
		}
		require.NoError(t, db.Insert(t.Context(), rule))
		permissionForm := map[string]string{
			"action": "reduce", "authorization_id": strconv.FormatInt(authorization.ID, 10),
			"target_repo_id": strconv.FormatInt(rule.TargetRepoID, 10), "unit_type": strconv.Itoa(int(rule.UnitType)), "mode": "read",
		}
		user2Session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, "/user/settings/codespaces/permissions", permissionForm), http.StatusSeeOther)
		loadedRule := new(codespace_model.PermissionRepository)
		ruleExists, err := db.GetEngine(t.Context()).
			Where("authorization_id = ? AND target_repo_id = ? AND unit_type = ?", rule.AuthorizationID, rule.TargetRepoID, rule.UnitType).
			Get(loadedRule)
		require.NoError(t, err)
		require.True(t, ruleExists)
		assert.Equal(t, perm.AccessModeRead, loadedRule.GrantedMode)
		loginUser(t, "user4").MakeRequest(t, NewRequestWithValues(t, http.MethodPost, "/user/settings/codespaces/permissions", permissionForm), http.StatusNotFound)
		manager := &codespace_model.Manager{
			Name:           "integration-manager",
			UserID:         2,
			RuntimeState:   codespace_model.ManagerRuntimeStateOnline,
			TagsJSON:       `[{"tag":"default"}]`,
			LastOnlineUnix: time.Now().Unix(),
			CreatedUnix:    time.Now().Unix(),
		}
		manager.GenerateManagerSecret()
		require.NoError(t, db.Insert(t.Context(), manager))
		user2Session.MakeRequest(t, NewRequestf(t, http.MethodGet, "/user/settings/codespaces/managers/%d", manager.ID), http.StatusOK)
		loginUser(t, "user4").MakeRequest(t, NewRequestf(t, http.MethodGet, "/user/settings/codespaces/managers/%d", manager.ID), http.StatusNotFound)

		secretSettings := "/user/settings/codespaces/secrets"
		user2Session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, secretSettings, map[string]string{
			"name": "PROJECT_TOKEN", "value": "saved-secret-value",
		}), http.StatusOK)
		saved := unittest.AssertExistsAndLoadBean(t, &codespace_model.UserSecret{UserID: 2, Name: "PROJECT_TOKEN"})
		secretPage := user2Session.MakeRequest(t, NewRequest(t, http.MethodGet, secretSettings), http.StatusOK)
		assert.Contains(t, secretPage.Body.String(), saved.Name)
		assert.NotContains(t, secretPage.Body.String(), "saved-secret-value")
		assert.NotContains(t, secretPage.Body.String(), saved.DataEncrypted)
		_, err = db.GetEngine(t.Context()).Where("user_id = ? AND name = ?", 0, "Default").Cols("content").Update(&codespace_model.DevContainerTemplate{
			Content: `{"image":"ubuntu:24.04","secrets":{"PROJECT_TOKEN":{"description":"Project API token"}}}`,
		})
		require.NoError(t, err)
		confirmation := user2Session.MakeRequest(t, NewRequest(t, http.MethodGet, "/user2/repo1/codespaces/new?ref_type=branch&ref_name=master"), http.StatusOK)
		secretRow := NewHTMLParser(t, confirmation.Body).doc.Find(`[data-codespace-secret-list] [data-codespace-secret-row]`).First()
		assert.Equal(t, "PROJECT_TOKEN", secretRow.Find(`[name="secret_name"]`).AttrOr("value", ""))
		assert.Equal(t, "none", secretRow.Find(`[name="secret_source"] option[selected]`).AttrOr("value", ""))
		assert.False(t, secretRow.Find(`[name="secret_source"] option[value="saved"]`).Is("[disabled]"))

		created := createCodespaceFromRepository(t, user2Session, "/user2/repo1/codespaces", "branch", "master", nil)
		location := created.Header().Get("Location")
		require.True(t, strings.HasPrefix(location, "/-/codespaces/"))
		instance := unittest.AssertExistsAndLoadBean(t, &codespace_model.Codespace{ID: integrationCodespaceIDFromLocation(t, location)})
		assert.Empty(t, instance.SecretsEncrypted)
		user2Session.MakeRequest(t, NewRequestf(t, http.MethodPost, "%s/%d/delete", secretSettings, saved.ID), http.StatusOK)
		user2Session.MakeRequest(t, NewRequest(t, http.MethodGet, location), http.StatusOK)
		loginUser(t, "user4").MakeRequest(t, NewRequest(t, http.MethodGet, location), http.StatusNotFound)
		forceDeleteCodespace := &codespace_model.Codespace{Status: codespace_model.StatusFailed, OperationRVersion: 1}
		insertIntegrationCodespace(t, 0, forceDeleteCodespace)

		adminSession := loginUser(t, "user1")
		adminSession.MakeRequest(t, NewRequest(t, http.MethodGet, "/-/admin/codespaces/managers"), http.StatusOK)
		adminSession.MakeRequest(t, NewRequest(t, http.MethodGet, "/-/admin/codespaces/dev-container-templates"), http.StatusOK)
		adminSession.MakeRequest(t, NewRequestf(t, http.MethodGet, "/-/admin/codespaces/managers/%d", manager.ID), http.StatusOK)
		user2Session.MakeRequest(t, NewRequest(t, http.MethodGet, "/-/admin/codespaces/managers"), http.StatusForbidden)
		user2Session.MakeRequest(t, NewRequest(t, http.MethodGet, "/org/org3/settings/codespaces"), http.StatusNotFound)

		forceDeleteURL := fmt.Sprintf("/-/admin/codespaces/managers/unassigned/%d/force-delete", forceDeleteCodespace.ID)
		adminSession.MakeRequest(t, NewRequest(t, http.MethodPost, forceDeleteURL), http.StatusSeeOther)
		adminSession.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, forceDeleteURL, map[string]string{
			"confirm": "force-delete",
		}), http.StatusSeeOther)
	})
}

func TestCodespaceTemplateSettings(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		session := loginUser(t, "user2")
		base := "/user/settings/codespaces/dev-container-templates"
		content := "{\n// Keep this comment\n\"image\": \"debian:12\",\n}"
		response := session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, base, map[string]string{"name": "Test template", "content": content}), http.StatusOK)
		var saved struct{ Redirect string }
		DecodeJSON(t, response, &saved)
		assert.Equal(t, base, saved.Redirect)
		template := unittest.AssertExistsAndLoadBean(t, &codespace_model.DevContainerTemplate{UserID: 2, Name: "Test template"})
		assert.Equal(t, content, template.Content)
		editURL := fmt.Sprintf("%s/%d", base, template.ID)
		response = session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, editURL, map[string]string{"name": "Test template", "content": "{"}), http.StatusBadRequest)
		assert.Contains(t, response.Header().Get("Content-Type"), "application/json")
		assert.Contains(t, response.Body.String(), "content")
		session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, editURL, map[string]string{"name": " ", "content": content}), http.StatusBadRequest)
		other := loginUser(t, "user4")
		other.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, editURL, map[string]string{"name": "Other", "content": content}), http.StatusNotFound)
		other.MakeRequest(t, NewRequest(t, http.MethodPost, editURL+"/delete"), http.StatusNotFound)
		session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, editURL, map[string]string{"name": "Updated template", "content": content}), http.StatusOK)
		session.MakeRequest(t, NewRequest(t, http.MethodGet, base), http.StatusOK)
		session.MakeRequest(t, NewRequest(t, http.MethodPost, editURL+"/delete"), http.StatusOK)
	})
}

func TestCodespaceManagerSettings(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		session := loginUser(t, "user2")
		base := "/user/settings/codespaces/managers"
		settings, err := codespace_service.ListManagerSettings(t.Context(), codespace_service.ManagerSettingsOptions{
			Scope: codespace_service.ManagerSettingsScopeUser, UserID: 2,
		})
		require.NoError(t, err)
		require.Len(t, settings.RegistrationToken, 40)
		response := session.MakeRequest(t, NewRequest(t, http.MethodGet, base), http.StatusOK)
		assert.Contains(t, response.Body.String(), settings.RegistrationToken)

		manager, secret, err := codespace_service.RegisterManager(t.Context(), settings.RegistrationToken, "New Manager")
		require.NoError(t, err)
		require.NotEmpty(t, secret)
		managerURL := base + "/" + strconv.FormatInt(manager.ID, 10)
		session.MakeRequest(t, NewRequest(t, http.MethodGet, managerURL), http.StatusOK)
		session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, managerURL, map[string]string{"name": "Renamed"}), http.StatusOK)

		response = session.MakeRequest(t, NewRequest(t, http.MethodPost, base+"/registration-token/reset").SetHeader("X-Gitea-Fetch-Action", "1"), http.StatusOK)
		var reset struct{ Redirect string }
		DecodeJSON(t, response, &reset)
		assert.Equal(t, base, reset.Redirect)
		rotated, err := codespace_service.ListManagerSettings(t.Context(), codespace_service.ManagerSettingsOptions{
			Scope: codespace_service.ManagerSettingsScopeUser, UserID: 2,
		})
		require.NoError(t, err)
		assert.NotEqual(t, settings.RegistrationToken, rotated.RegistrationToken)

		other := loginUser(t, "user4")
		other.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, managerURL, map[string]string{"name": "Other"}), http.StatusNotFound)
		response = session.MakeRequest(t, NewRequest(t, http.MethodGet, managerURL), http.StatusOK)
		assert.Contains(t, response.Body.String(), "Renamed")
		admin := loginUser(t, "user1")
		adminURL := strings.Replace(managerURL, base, "/-/admin/codespaces/managers", 1)
		admin.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, adminURL, map[string]string{"name": "Admin renamed"}), http.StatusOK)
		response = session.MakeRequest(t, NewRequest(t, http.MethodPost, managerURL+"/delete").SetHeader("X-Gitea-Fetch-Action", "1"), http.StatusBadRequest)
		var deletionError struct{ ErrorMessage string }
		DecodeJSON(t, response, &deletionError)
		assert.NotEmpty(t, deletionError.ErrorMessage)
		other.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, managerURL+"/delete", map[string]string{"confirm": "delete-manager"}).SetHeader("X-Gitea-Fetch-Action", "1"), http.StatusNotFound)
		session.MakeRequest(t, NewRequest(t, http.MethodGet, managerURL), http.StatusOK)
		response = session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, managerURL+"/delete", map[string]string{"confirm": "delete-manager"}).SetHeader("X-Gitea-Fetch-Action", "1"), http.StatusOK)
		var deleted struct{ Redirect string }
		DecodeJSON(t, response, &deleted)
		assert.Equal(t, base, deleted.Redirect)
		session.MakeRequest(t, NewRequest(t, http.MethodGet, managerURL), http.StatusNotFound)
	})
}

func TestCodespaceTokenAPIRoutePolicy(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		defer test.MockVariableValue(&setting.Codespace.Enabled, true)()

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		token, codespaceID := createRunningCodespaceTokenForRepo(t, repo)

		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/user").AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/version").AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/signing-key.pub").AddTokenAuth(token), http.StatusNotFound)
		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s", repo.OwnerName, repo.Name).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/branches", repo.OwnerName, repo.Name).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/code/search?q=README", repo.OwnerName, repo.Name).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/commits/master/status", repo.OwnerName, repo.Name).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/forks", repo.OwnerName, repo.Name).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/actions/runs", repo.OwnerName, repo.Name).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/actions/workflows/missing/dispatches", repo.OwnerName, repo.Name), map[string]string{}).AddTokenAuth(token), http.StatusUnprocessableEntity)
		MakeRequest(t, NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/markdown", repo.OwnerName, repo.Name), &api.MarkdownOption{Text: "codespace"}).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user5/repo4").AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/user2/repo1/info/refs?service=git-upload-pack").AddBasicAuth("codespace", token), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/user2/repo1.git/info/lfs/locks").AddBasicAuth("codespace", token).SetHeader("Accept", lfs.MediaType), http.StatusOK)

		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repositories/%d", repo.ID).AddTokenAuth(token), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/user/orgs").AddTokenAuth(token), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/issues/search").AddTokenAuth(token), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/users/user2/tokens").AddTokenAuth(token), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo2").AddTokenAuth(token), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo2/code/search?q=README").AddTokenAuth(token), http.StatusForbidden)
		MakeRequest(t, NewRequestf(t, http.MethodPost, "/api/v1/repos/%s/%s/forks", repo.OwnerName, repo.Name).AddTokenAuth(token), http.StatusForbidden)
		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/actions/secrets", repo.OwnerName, repo.Name).AddTokenAuth(token), http.StatusForbidden)
		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/hooks/1/deliveries", repo.OwnerName, repo.Name).AddTokenAuth(token), http.StatusForbidden)
		personalToken := getTokenForLoggedInUser(t, loginUser(t, "user2"), auth_model.AccessTokenScopeWriteRepository)
		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/actions/secrets", repo.OwnerName, repo.Name).AddTokenAuth(personalToken), http.StatusOK)

		now := time.Now().Unix()
		pullUnit := unittest.AssertExistsAndLoadBean(t, &repo_model.RepoUnit{RepoID: repo.ID, Type: unit.TypePullRequests})
		pullUnit.ID = 0
		pullUnit.RepoID = 2
		require.NoError(t, db.Insert(t.Context(), pullUnit))
		authorization := &codespace_model.PermissionAuthorization{
			UserID: 2, SourceRepoID: repo.ID, RequestHash: "integration-repository-grant",
			CreatedUnix: now, UpdatedUnix: now,
		}
		require.NoError(t, db.Insert(t.Context(), authorization))
		rule := &codespace_model.PermissionRepository{
			AuthorizationID: authorization.ID, TargetRepoID: 2, UnitType: unit.TypeCode,
			RequestedMode: perm.AccessModeWrite, GrantedMode: perm.AccessModeWrite,
		}
		require.NoError(t, db.Insert(t.Context(), rule))
		for _, unitType := range []unit.Type{unit.TypeIssues, unit.TypePullRequests, unit.TypeWiki, unit.TypeReleases, unit.TypeActions} {
			require.NoError(t, db.Insert(t.Context(), &codespace_model.PermissionRepository{
				AuthorizationID: authorization.ID, TargetRepoID: 2, UnitType: unitType,
				RequestedMode: perm.AccessModeRead, GrantedMode: perm.AccessModeRead,
			}))
		}
		require.NoError(t, db.Insert(t.Context(), &codespace_model.PermissionRepository{
			AuthorizationID: authorization.ID, TargetRepoID: 4, UnitType: unit.TypeCode,
			RequestedMode: perm.AccessModeWrite, GrantedMode: perm.AccessModeWrite,
		}))
		updated, err := db.GetEngine(t.Context()).Where("id = ?", codespaceID).Cols("permission_authorization_id").Update(&codespace_model.Codespace{PermissionAuthorizationID: authorization.ID})
		require.NoError(t, err)
		require.EqualValues(t, 1, updated)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo2").AddTokenAuth(token), http.StatusOK)
		for _, tt := range []struct {
			path   string
			status int
		}{
			{path: "/api/v1/repos/user2/repo2/issues", status: http.StatusOK},
			{path: "/api/v1/repos/user2/repo2/pulls", status: http.StatusOK},
			{path: "/api/v1/repos/user2/repo2/wiki/pages", status: http.StatusNotFound}, // Permission passes before the absent wiki repository is reported.
			{path: "/api/v1/repos/user2/repo2/releases", status: http.StatusOK},
			{path: "/api/v1/repos/user2/repo2/actions/runs", status: http.StatusOK},
		} {
			MakeRequest(t, NewRequest(t, http.MethodGet, tt.path).AddTokenAuth(token), tt.status)
		}
		for _, path := range []string{
			"/api/v1/repos/user2/repo2/issues",
			"/api/v1/repos/user2/repo2/pulls",
			"/api/v1/repos/user2/repo2/wiki/new",
			"/api/v1/repos/user2/repo2/releases",
			"/api/v1/repos/user2/repo2/actions/workflows/missing/dispatches",
		} {
			MakeRequest(t, NewRequestWithJSON(t, http.MethodPost, path, map[string]string{}).AddTokenAuth(token), http.StatusForbidden)
		}
		snapshot, err := codespace_service.ResolveGiteaToken(t.Context(), token)
		require.NoError(t, err)
		require.True(t, snapshot.CodespaceTokenAllowsRepository(4, unit.TypeCode, perm.AccessModeWrite))
		MakeRequest(t, NewRequestWithJSON(t, http.MethodPost, "/api/v1/repos/user5/repo4/branches", map[string]string{}).AddTokenAuth(token), http.StatusForbidden)

		require.NoError(t, codespace_service.ReducePermissionRepository(t.Context(), 2, authorization.ID, rule.TargetRepoID, rule.UnitType, perm.AccessModeNone))
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo2").AddTokenAuth(token), http.StatusForbidden)

		authorization = &codespace_model.PermissionAuthorization{
			UserID: 2, SourceRepoID: repo.ID, RequestHash: "integration-repository-revoke",
			CreatedUnix: now, UpdatedUnix: now,
		}
		require.NoError(t, db.Insert(t.Context(), authorization))
		require.NoError(t, db.Insert(t.Context(), &codespace_model.PermissionRepository{
			AuthorizationID: authorization.ID, TargetRepoID: 2, UnitType: unit.TypeCode,
			RequestedMode: perm.AccessModeRead, GrantedMode: perm.AccessModeRead,
		}))
		updated, err = db.GetEngine(t.Context()).Where("id = ?", codespaceID).Cols("permission_authorization_id").Update(&codespace_model.Codespace{PermissionAuthorizationID: authorization.ID})
		require.NoError(t, err)
		require.EqualValues(t, 1, updated)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo2").AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo2/releases").AddTokenAuth(token), http.StatusForbidden)
		require.NoError(t, codespace_service.RevokePermissionAuthorization(t.Context(), 2, authorization.ID))
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo2").AddTokenAuth(token), http.StatusForbidden)

		privateRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
		collaborator := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		require.NoError(t, repo_service.AddOrUpdateCollaborator(t.Context(), privateRepo, collaborator, perm.AccessModeWrite))
		collaboratorToken, collaboratorCodespaceID := createRunningCodespaceToken(t, privateRepo, collaborator)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo2").AddTokenAuth(collaboratorToken), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/user2/repo2/info/refs?service=git-upload-pack").AddBasicAuth("codespace", collaboratorToken), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/user2/repo2/info/refs?service=git-receive-pack").AddBasicAuth("codespace", collaboratorToken), http.StatusOK)

		require.NoError(t, repo_service.AddOrUpdateCollaborator(t.Context(), privateRepo, collaborator, perm.AccessModeRead))
		currentPermission, err := access_model.GetDoerRepoPermission(t.Context(), privateRepo, collaborator)
		require.NoError(t, err)
		require.False(t, currentPermission.CanWrite(unit.TypeCode))
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo2").AddTokenAuth(collaboratorToken), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/user2/repo2/info/refs?service=git-upload-pack").AddBasicAuth("codespace", collaboratorToken), http.StatusOK)
		require.NoError(t, git.CreateDelegateHooks(t.Context(), privateRepo))
		pushURL := *giteaURL
		pushURL.Path = "/user2/repo2.git"
		pushURL.User = url.UserPassword("codespace", collaboratorToken)
		pushRef := "HEAD:refs/heads/codespace-read-only-" + uuid.NewV4().String()
		_, _, pushErr := gitcmd.NewCommand("push").AddDynamicArguments(pushURL.String(), pushRef).WithRepo(privateRepo).RunStdString(t.Context())
		require.Error(t, pushErr)
		assert.Contains(t, pushErr.Stderr(), "User permission denied for writing.")

		require.NoError(t, repo_service.DeleteCollaboration(t.Context(), privateRepo, collaborator))
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo2").AddTokenAuth(collaboratorToken), http.StatusNotFound)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/user2/repo2/info/refs?service=git-upload-pack").AddBasicAuth("codespace", collaboratorToken), http.StatusNotFound)
		collaboratorCodespace := unittest.AssertExistsAndLoadBean(t, &codespace_model.Codespace{ID: collaboratorCodespaceID})
		_, err = codespace_service.GetCreatorCodespace(t.Context(), codespace_service.CreatorDetailOptions{UserID: collaborator.ID, CodespaceID: collaboratorCodespace.ID})
		require.NoError(t, err)

		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/actions/artifacts/999999/zip/raw", repo.OwnerName, repo.Name).AddTokenAuth("gcs_invalid"), http.StatusNotFound)

		require.NoError(t, repo_model.SetArchiveRepoState(t.Context(), repo, true))
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo1/issues/1/reactions").AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodDelete, "/api/v1/repos/user2/repo1/issues/comments/1").AddTokenAuth(token), http.StatusLocked)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo1/releases/1").AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodDelete, "/api/v1/repos/user2/repo1/releases/1").AddTokenAuth(token), http.StatusLocked)
	})
}

func TestCodespaceLifecycleStateMachineIntegration(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		insertIntegrationDevContainerTemplate(t)
		manager, secret := createIntegrationManager(t)
		client := codespacev1connect.NewManagerServiceClient(connect.NewClient(connecthttp.NewTransport(http.DefaultClient,
			strings.TrimRight(giteaURL.String(), "/")+"/api/codespace")),
		)

		user2Session := loginUser(t, "user2")
		created := createCodespaceFromRepository(t, user2Session, "/user2/repo1/codespaces", "branch", "master", map[string]string{
			"secret_name": "PROJECT_TOKEN", "secret_source": "custom", "secret_value": " first\nsecond\n",
		})
		codespaceID := integrationCodespaceIDFromLocation(t, created.Header().Get("Location"))
		codespacePath := "/-/codespaces/" + strconv.FormatInt(codespaceID, 10)

		row := loadIntegrationCodespace(t, codespaceID)
		require.Equal(t, codespace_model.StatusCreating, row.Status)
		require.Equal(t, codespace_model.OperationCreate, codespace_model.ActiveOperationType(row))
		require.True(t, codespace_model.IsOperationQueued(row))
		require.Equal(t, codespace_model.OperationTriggerUser, row.OperationTrigger)
		require.EqualValues(t, 1, row.OperationRVersion)
		require.Empty(t, row.DevContainerPath)
		require.JSONEq(t, `{"image":"mcr.microsoft.com/devcontainers/base:ubuntu"}`, row.DevContainerContent)

		fetched, err := client.FetchOperations(codespaceManagerContext(t.Context(), manager.ID, secret), &codespacev1.FetchOperationsRequest{
			StartupCapacityAvailable: 1,
			AcceptedCreateTags:       []string{"default"},
		})
		require.NoError(t, err)
		require.Len(t, fetched.GetOperations(), 1)
		operation := fetched.GetOperations()[0]
		assert.NotNil(t, operation.GetCreate())
		require.Equal(t, codespaceID, operation.GetCodespaceId())

		_, err = client.FinalizeOperation(codespaceManagerContext(t.Context(), manager.ID, secret), createFinalRequest(codespaceID, 1, codespacev1.FinalStatus_FINAL_STATUS_DONE))
		require.Error(t, err)
		assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
		assert.Equal(t, "gitea_token_required", integrationFailureDetailCategory(t, err))

		tokenResponse, err := client.RequestRuntimeAccess(codespaceManagerContext(t.Context(), manager.ID, secret), &codespacev1.RequestRuntimeAccessRequest{
			CodespaceId:       codespaceID,
			OperationRversion: 1,
			GitSshPublicKey:   integrationGitSSHPublicKey(t),
		})
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(tokenResponse.GetAccess().GetGiteaToken(), "gcs_"))
		require.Len(t, tokenResponse.GetAccess().GetSecrets(), 1)
		assert.Equal(t, "PROJECT_TOKEN", tokenResponse.GetAccess().GetSecrets()[0].GetName())
		assert.Equal(t, " first\nsecond\n", tokenResponse.GetAccess().GetSecrets()[0].GetValue())

		finalCreate, err := client.FinalizeOperation(codespaceManagerContext(t.Context(), manager.ID, secret), createFinalRequest(codespaceID, 1, codespacev1.FinalStatus_FINAL_STATUS_DONE))
		require.NoError(t, err)
		assert.False(t, finalCreate.GetResourceAbsent())
		row = loadIntegrationCodespace(t, codespaceID)
		assert.Equal(t, codespace_model.StatusRunning, row.Status)
		assert.EqualValues(t, 1, row.RuntimeOperationRVersion)
		assert.Empty(t, codespace_model.ActiveOperationType(row))
		assertIntegrationExists(t, new(codespace_model.GiteaToken), "codespace_id = ?", codespaceID)

		readyMetadata := integrationRuntimeMetadata(1)
		readyMetadata.ResourceUsage = nil
		_, err = client.ReportRuntimeMetadata(codespaceManagerContext(t.Context(), manager.ID, secret), &codespacev1.ReportRuntimeMetadataRequest{
			CodespaceId:        codespaceID,
			MetadataGeneration: 1,
			Metadata:           readyMetadata,
		})
		require.NoError(t, err)

		autoStopResponse := user2Session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, codespacePath+"/auto-stop", map[string]string{
			"mode":          "custom",
			"timeout_value": "30",
			"timeout_unit":  "minutes",
			"return_to":     codespacePath,
		}), http.StatusOK)
		assert.JSONEq(t, `{"redirect":"`+codespacePath+`"}`, autoStopResponse.Body.String())
		row = loadIntegrationCodespace(t, codespaceID)
		assert.Equal(t, codespace_model.AutoStopModeCustom, row.AutoStopMode)
		assert.EqualValues(t, 30*60, row.AutoStopTimeoutSeconds)

		idleStop, err := client.RequestIdleStop(codespaceManagerContext(t.Context(), manager.ID, secret), &codespacev1.RequestIdleStopRequest{
			CodespaceId: codespaceID,
			ObservedSettings: &codespacev1.EffectiveCodespaceRuntimeSettings{
				IdleTimeoutSeconds:    int64(setting.Codespace.AutoStopDefaultTimeout / time.Second),
				InteractionGeneration: row.InteractionGeneration,
			},
		})
		require.NoError(t, err)
		require.NotNil(t, idleStop.GetPending())

		user2Session.MakeRequest(t, NewRequest(t, http.MethodPost, codespacePath+"/continue"), http.StatusSeeOther)
		row = loadIntegrationCodespace(t, codespaceID)
		assert.Equal(t, codespace_model.StatusRunning, row.Status)
		assert.Empty(t, codespace_model.ActiveOperationType(row))
		assert.EqualValues(t, 1, row.InteractionGeneration)

		for cycle, trigger := range []string{codespace_model.OperationTriggerUser, codespace_model.OperationTriggerIdle} {
			if trigger == codespace_model.OperationTriggerIdle {
				idleStop, err = client.RequestIdleStop(codespaceManagerContext(t.Context(), manager.ID, secret), &codespacev1.RequestIdleStopRequest{
					CodespaceId: codespaceID,
					ObservedSettings: &codespacev1.EffectiveCodespaceRuntimeSettings{
						IdleTimeoutSeconds:    row.AutoStopTimeoutSeconds,
						InteractionGeneration: row.InteractionGeneration,
					},
				})
				require.NoError(t, err)
				require.NotNil(t, idleStop.GetPending())
			} else {
				user2Session.MakeRequest(t, NewRequest(t, http.MethodPost, codespacePath+"/stop"), http.StatusSeeOther)
			}
			fetched, err = client.FetchOperations(codespaceManagerContext(t.Context(), manager.ID, secret), &codespacev1.FetchOperationsRequest{
				CleanupCapacityAvailable: 1,
			})
			require.NoError(t, err)
			require.Len(t, fetched.GetOperations(), 1)
			assert.NotNil(t, fetched.GetOperations()[0].GetStop())
			stopVersion := fetched.GetOperations()[0].GetOperationRversion()
			finalStop, err := client.FinalizeOperation(codespaceManagerContext(t.Context(), manager.ID, secret), createFinalRequest(codespaceID, stopVersion, codespacev1.FinalStatus_FINAL_STATUS_DONE))
			require.NoError(t, err)
			assert.False(t, finalStop.GetResourceAbsent())
			row = loadIntegrationCodespace(t, codespaceID)
			assert.Equal(t, codespace_model.StatusStopped, row.Status)
			assert.Empty(t, codespace_model.ActiveOperationType(row))
			assertIntegrationNotExists(t, new(codespace_model.GiteaToken), "codespace_id = ?", codespaceID)

			user2Session.MakeRequest(t, NewRequest(t, http.MethodPost, codespacePath+"/resume"), http.StatusSeeOther)
			row = loadIntegrationCodespace(t, codespaceID)
			assert.Equal(t, codespace_model.OperationResume, codespace_model.ActiveOperationType(row))
			assert.EqualValues(t, cycle+2, row.InteractionGeneration)
			fetched, err = client.FetchOperations(codespaceManagerContext(t.Context(), manager.ID, secret), &codespacev1.FetchOperationsRequest{
				StartupCapacityAvailable: 1,
			})
			require.NoError(t, err)
			require.Len(t, fetched.GetOperations(), 1)
			assert.NotNil(t, fetched.GetOperations()[0].GetResume())
			resumeVersion := fetched.GetOperations()[0].GetOperationRversion()
			_, err = client.RequestRuntimeAccess(codespaceManagerContext(t.Context(), manager.ID, secret), &codespacev1.RequestRuntimeAccessRequest{
				CodespaceId:       codespaceID,
				OperationRversion: resumeVersion,
				GitSshPublicKey:   integrationGitSSHPublicKey(t),
			})
			require.NoError(t, err)
			_, err = client.ReportRuntimeMetadata(codespaceManagerContext(t.Context(), manager.ID, secret), &codespacev1.ReportRuntimeMetadataRequest{
				CodespaceId:        codespaceID,
				MetadataGeneration: int64(cycle + 2),
				Metadata:           integrationRuntimeMetadata(resumeVersion),
			})
			require.NoError(t, err)
			finalResume, err := client.FinalizeOperation(codespaceManagerContext(t.Context(), manager.ID, secret), createFinalRequest(codespaceID, resumeVersion, codespacev1.FinalStatus_FINAL_STATUS_DONE))
			require.NoError(t, err)
			assert.False(t, finalResume.GetResourceAbsent())
			row = loadIntegrationCodespace(t, codespaceID)
			assert.Equal(t, codespace_model.StatusRunning, row.Status)
		}

		user2Session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, codespacePath+"/delete", map[string]string{
			"return_to": "/-/codespaces",
		}), http.StatusSeeOther)
		assertIntegrationNotExists(t, new(codespace_model.GiteaToken), "codespace_id = ?", codespaceID)
		fetched, err = client.FetchOperations(codespaceManagerContext(t.Context(), manager.ID, secret), &codespacev1.FetchOperationsRequest{
			CleanupCapacityAvailable: 1,
		})
		require.NoError(t, err)
		require.Len(t, fetched.GetOperations(), 1)
		assert.NotNil(t, fetched.GetOperations()[0].GetDelete())
		deleteVersion := fetched.GetOperations()[0].GetOperationRversion()
		finalDelete, err := client.FinalizeOperation(codespaceManagerContext(t.Context(), manager.ID, secret), createFinalRequest(codespaceID, deleteVersion, codespacev1.FinalStatus_FINAL_STATUS_DONE))
		require.NoError(t, err)
		assert.False(t, finalDelete.GetResourceAbsent())
		assertIntegrationNotExists(t, new(codespace_model.Codespace), "id = ?", codespaceID)
	})
}

func insertIntegrationDevContainerTemplate(t *testing.T) {
	t.Helper()
	now := time.Now().Unix()
	require.NoError(t, db.Insert(t.Context(), &codespace_model.DevContainerTemplate{
		UserID:      0,
		Name:        "Default",
		Content:     `{"image":"mcr.microsoft.com/devcontainers/base:ubuntu"}`,
		CreatedUnix: now,
		UpdatedUnix: now,
	}))
}

func TestCodespaceInventoryStateMachineIntegration(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		manager, secret := createIntegrationManager(t)
		client := codespacev1connect.NewManagerServiceClient(connect.NewClient(connecthttp.NewTransport(http.DefaultClient,
			strings.TrimRight(giteaURL.String(), "/")+"/api/codespace")),
		)

		running := &codespace_model.Codespace{
			Status:                codespace_model.StatusRunning,
			OperationRVersion:     11,
			InteractionGeneration: 21,
		}
		insertIntegrationCodespace(t, manager.ID, running)
		runningID := running.ID

		inventory, err := client.ReportInstances(codespaceManagerContext(t.Context(), manager.ID, secret), &codespacev1.ReportInstancesRequest{
			InventoryGeneration: 1,
			Instances: []*codespacev1.RuntimeInstanceRef{
				{CodespaceId: runningID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_RUNNING},
			},
		})
		require.NoError(t, err)
		require.Len(t, inventory.GetResults(), 1)
		assert.Equal(t, runningID, inventory.GetResults()[0].GetCodespaceId())
		assert.NotNil(t, inventory.GetResults()[0].GetRuntimeSettings())
		assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_UNSPECIFIED, inventory.GetResults()[0].GetAction())
		assert.EqualValues(t, 21, inventory.GetResults()[0].GetRuntimeSettings().GetInteractionGeneration())
		assert.EqualValues(t, 1, loadIntegrationManager(t, manager.ID).InventoryGeneration)

		_, err = client.ReportInstances(codespaceManagerContext(t.Context(), manager.ID, secret), &codespacev1.ReportInstancesRequest{
			InventoryGeneration: 2,
			Instances: []*codespacev1.RuntimeInstanceRef{{
				CodespaceId:               runningID,
				RuntimeState:              codespacev1.RuntimeState_RUNTIME_STATE_RUNNING,
				ObservedOperationRversion: 12,
			}},
		})
		require.Error(t, err)
		assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
		assert.Equal(t, "state_history_conflict", integrationFailureDetailCategory(t, err))
		assert.EqualValues(t, 1, loadIntegrationManager(t, manager.ID).InventoryGeneration)
		assert.Equal(t, codespace_model.StatusRunning, loadIntegrationCodespace(t, runningID).Status)
	})
}

func createCodespaceFromRepository(t *testing.T, session *TestSession, path, refType, refName string, secretFields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	values := map[string]string{"ref_type": refType, "ref_name": refName}
	maps.Copy(values, secretFields)
	query := url.Values{}
	query.Set("ref_type", refType)
	query.Set("ref_name", refName)
	confirmation := session.MakeRequest(t, NewRequest(t, http.MethodGet, path+"/new?"+query.Encode()), http.StatusOK)
	doc := NewHTMLParser(t, confirmation.Body).doc
	requestHash, ok := doc.Find(`input[name="request_hash"]`).Attr("value")
	require.True(t, ok)
	require.NotEmpty(t, requestHash)
	environmentTag, ok := doc.Find(`#codespace-environment-tag .menu .item`).First().Attr("data-value")
	require.True(t, ok)
	require.NotEmpty(t, environmentTag)
	devContainer, ok := doc.Find(`#codespace-dev-container .menu .item`).First().Attr("data-value")
	require.True(t, ok)
	require.NotEmpty(t, devContainer)
	values["request_hash"] = requestHash
	values["environment_tag"] = environmentTag
	values["dev_container"] = devContainer
	return session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, path, values), http.StatusSeeOther)
}

func createRunningCodespaceTokenForRepo(t *testing.T, repo *repo_model.Repository) (string, int64) {
	t.Helper()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	return createRunningCodespaceToken(t, repo, user)
}

func createRunningCodespaceToken(t *testing.T, repo *repo_model.Repository, user *user_model.User) (string, int64) {
	t.Helper()
	manager := &codespace_model.Manager{
		Name:           "codespace-token-manager",
		RuntimeState:   codespace_model.ManagerRuntimeStateOnline,
		TagsJSON:       `[{"tag":"default"}]`,
		LastOnlineUnix: time.Now().Unix(),
		CreatedUnix:    time.Now().Unix(),
	}
	manager.GenerateManagerSecret()
	require.NoError(t, db.Insert(t.Context(), manager))

	codespace := &codespace_model.Codespace{
		UserID:                   user.ID,
		RepoID:                   repo.ID,
		RefType:                  "branch",
		RefName:                  repo.DefaultBranch,
		EnvironmentTag:           "default",
		CommitSHA:                "0123456789abcdef0123456789abcdef01234567",
		ManagerID:                manager.ID,
		Status:                   codespace_model.StatusRunning,
		OperationRVersion:        1,
		RuntimeOperationRVersion: 1,
		AutoStopMode:             codespace_model.AutoStopModeDefault,
		CreatedUnix:              time.Now().Unix(),
		UpdatedUnix:              time.Now().Unix(),
	}
	require.NoError(t, db.Insert(t.Context(), codespace))
	codespaceID := codespace.ID
	result, err := codespace_service.RequestRuntimeAccess(t.Context(), manager, codespace_service.RequestRuntimeAccessOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 1,
		GitSSHPublicKey:   integrationUniqueGitSSHPublicKey(t),
	})
	require.NoError(t, err)
	return result.Token, codespaceID
}

func integrationUniqueGitSSHPublicKey(t *testing.T) []byte {
	t.Helper()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	key, err := ssh.NewPublicKey(publicKey)
	require.NoError(t, err)
	return key.Marshal()
}

func integrationGitSSHPublicKey(t *testing.T) []byte {
	t.Helper()
	key, err := base64.StdEncoding.DecodeString("AAAAC3NzaC1lZDI1NTE5AAAAICV0MGX/W9IvLA4FXpIuUcdDcbj5KX4syHgsTy7soVgf")
	require.NoError(t, err)
	return key
}

func insertIntegrationCodespace(t *testing.T, managerID int64, codespace *codespace_model.Codespace) {
	t.Helper()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	now := time.Now().Unix()
	codespace.UserID = user.ID
	codespace.RepoID = repo.ID
	if codespace.RefType == "" {
		codespace.RefType = "branch"
	}
	if codespace.RefName == "" {
		codespace.RefName = repo.DefaultBranch
	}
	if codespace.EnvironmentTag == "" {
		codespace.EnvironmentTag = "default"
	}
	if codespace.CommitSHA == "" {
		codespace.CommitSHA = "0123456789abcdef0123456789abcdef01234567"
	}
	if codespace.DevContainerPath == "" && codespace.DevContainerContent == "" {
		codespace.DevContainerPath = ".devcontainer/devcontainer.json"
	}
	codespace.ManagerID = managerID
	if codespace.Status == codespace_model.StatusRunning && codespace.RuntimeOperationRVersion == 0 {
		codespace.RuntimeOperationRVersion = codespace.OperationRVersion
	}
	if codespace.AutoStopMode == "" {
		codespace.AutoStopMode = codespace_model.AutoStopModeDefault
	}
	if codespace.CreatedUnix == 0 {
		codespace.CreatedUnix = now
	}
	if codespace.UpdatedUnix == 0 {
		codespace.UpdatedUnix = now
	}
	var err error
	err = db.Insert(t.Context(), codespace)
	require.NoError(t, err)
}

func createIntegrationManager(t *testing.T) (*codespace_model.Manager, string) {
	t.Helper()
	manager := &codespace_model.Manager{
		Name:           "integration-state-manager",
		GatewayURL:     "https://workspace.example.com",
		RuntimeState:   codespace_model.ManagerRuntimeStateOnline,
		TagsJSON:       `[{"tag":"default"}]`,
		LastOnlineUnix: time.Now().Unix(),
		CreatedUnix:    time.Now().Unix(),
	}
	secret := manager.GenerateManagerSecret()
	require.NoError(t, db.Insert(t.Context(), manager))
	return manager, secret
}

func loadIntegrationManager(t *testing.T, managerID int64) *codespace_model.Manager {
	t.Helper()
	manager := new(codespace_model.Manager)
	has, err := db.GetEngine(t.Context()).ID(managerID).Get(manager)
	require.NoError(t, err)
	require.True(t, has)
	return manager
}

func loadIntegrationCodespace(t *testing.T, codespaceID int64) *codespace_model.Codespace {
	t.Helper()
	row := new(codespace_model.Codespace)
	has, err := db.GetEngine(t.Context()).Where("id = ?", codespaceID).Get(row)
	require.NoError(t, err)
	require.True(t, has)
	return row
}

func integrationCodespaceIDFromLocation(t *testing.T, location string) int64 {
	t.Helper()
	require.True(t, strings.HasPrefix(location, "/-/codespaces/"))
	codespaceID, err := strconv.ParseInt(strings.TrimPrefix(location, "/-/codespaces/"), 10, 64)
	require.NoError(t, err)
	return codespaceID
}

func codespaceManagerContext(ctx context.Context, managerID int64, secret string) context.Context {
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("x-codespace-manager-id", strconv.FormatInt(managerID, 10))
	info.RequestHeader().Set("x-codespace-manager-secret", secret)
	return ctx
}

func createFinalRequest(codespaceID, operationRVersion int64, finalStatus codespacev1.FinalStatus) *codespacev1.FinalizeOperationRequest {
	return &codespacev1.FinalizeOperationRequest{
		CodespaceId: codespaceID,

		OperationRversion: operationRVersion,
		Status:            finalStatus,
	}
}

func integrationRuntimeMetadata(operationRVersion int64) *codespacev1.RuntimeMetadata {
	return &codespacev1.RuntimeMetadata{
		Endpoints:       []*codespacev1.RuntimeEndpoint{{EndpointId: "workspace", Label: "Workspace", Url: "https://reported.workspace.example.com/"}},
		WorkspaceFolder: "/workspaces/project",
		Boot: &codespacev1.RuntimeBoot{
			OperationRversion: operationRVersion,
			Stage:             codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY,
			StartedUnix:       100,
			LastUpdateUnix:    101,
		},
		ResourceUsage: &codespacev1.RuntimeResourceUsage{
			Cpu:          &codespacev1.RuntimeCPUUsage{UsedMillicores: 125, LimitMillicores: 1000},
			Memory:       &codespacev1.RuntimeMemoryUsage{UsedBytes: 256 * 1024 * 1024, LimitBytes: 1024 * 1024 * 1024},
			Disk:         &codespacev1.RuntimeDiskUsage{UsedBytes: 512 * 1024 * 1024, LimitBytes: 10 * 1024 * 1024 * 1024},
			ObservedUnix: 101,
		},
	}
}

func integrationFailureDetailCategory(t *testing.T, err error) string {
	t.Helper()
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	for _, detail := range connectErr.Details() {
		value, detailErr := connectproto.UnmarshalErrorDetail(detail)
		require.NoError(t, detailErr)
		if failure, ok := value.(*codespacev1.FailureDetail); ok {
			return failure.GetCategory()
		}
	}
	require.FailNow(t, "missing failure detail")
	return ""
}

func assertIntegrationExists(t *testing.T, bean any, query string, args ...any) {
	t.Helper()
	has, err := db.GetEngine(t.Context()).Where(query, args...).Exist(bean)
	require.NoError(t, err)
	assert.True(t, has)
}

func assertIntegrationNotExists(t *testing.T, bean any, query string, args ...any) {
	t.Helper()
	has, err := db.GetEngine(t.Context()).Where(query, args...).Exist(bean)
	require.NoError(t, err)
	assert.False(t, has)
}
