// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	deploykey_model "gitea.dev/models/deploykey"
	"gitea.dev/models/organization"
	pages_model "gitea.dev/models/pages"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	pages_content "gitea.dev/modules/pages"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/contexttest"
	"gitea.dev/services/forms"
	mirror_service "gitea.dev/services/mirror"
	pages_service "gitea.dev/services/pages"
	repo_service "gitea.dev/services/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPagesUploadArchiveStagesAndCommits(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "index.html"), []byte("Pages upload"), 0o600))
	var archive bytes.Buffer
	require.NoError(t, pages_content.PackDirectory(t.Context(), root, &archive, pages_content.DefaultLimits()))

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	file, err := form.CreateFormFile("file", "site.tar.gz")
	require.NoError(t, err)
	_, err = io.Copy(file, &archive)
	require.NoError(t, err)
	require.NoError(t, form.Close())

	stageCtx, stageResp := contexttest.MockContext(t, "POST /user2/repo1/settings/pages/versions/upload")
	request := httptest.NewRequest(http.MethodPost, "/user2/repo1/settings/pages/versions/upload", &body).WithContext(stageCtx.Req.Context())
	request.Header.Set("Content-Type", form.FormDataContentType())
	stageCtx.Req = request
	PagesUploadArchive(stageCtx)
	require.Equal(t, http.StatusOK, stageResp.Code)
	var staged struct {
		UUID string `json:"uuid"`
	}
	require.NoError(t, json.Unmarshal(stageResp.Body.Bytes(), &staged))
	require.NotEmpty(t, staged.UUID)

	ctx, _ := contexttest.MockContext(t, "POST /user2/repo1/settings/pages/versions")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.MockRequestPostForm(ctx.Req, url.Values{"action": {"upload"}, "files": {staged.UUID}})
	PagesVersionsPost(ctx)

	assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
	site, has, err := pages_model.GetSiteByRepoID(t.Context(), 1)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, pages_model.PublishSourceArtifact, site.SourceType)
	assert.False(t, site.Enabled)
	assert.NotZero(t, site.CurrentVersionID)
	_, err = repo_model.GetUploadByUUID(t.Context(), staged.UUID)
	assert.True(t, repo_model.IsErrUploadNotExist(err))
}

func TestPagesPostRestoresVersion(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	versions := make([]*pages_model.Version, 0, 2)
	for _, content := range [][]byte{[]byte("old version"), []byte("current version")} {
		digest := sha256.Sum256(content)
		version := &pages_model.Version{RepoID: 1, TarGzSHA256: hex.EncodeToString(digest[:]), OriginType: pages_model.VersionOriginUpload}
		require.NoError(t, pages_model.InsertVersion(t.Context(), version))
		require.NoError(t, pages_content.NewContentStore().Save(1, version.TarGzSHA256, bytes.NewReader(content), int64(len(content))))
		versions = append(versions, version)
	}
	require.NoError(t, pages_model.InsertSite(t.Context(), &pages_model.Site{
		RepoID: 1, Enabled: true, SourceType: pages_model.PublishSourceArtifact,
		CurrentVersionID: versions[1].ID, PagesUpdatedUnixNano: 1,
	}))

	ctx, _ := contexttest.MockContext(t, "POST /user2/repo1/settings/pages/versions")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.MockRequestPostForm(ctx.Req, url.Values{
		"action": {"restore-version"}, "version_id": {strconv.FormatInt(versions[0].ID, 10)},
	})
	PagesVersionsPost(ctx)
	require.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
	site, has, err := pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, versions[0].ID, site.CurrentVersionID)
}

func TestPagesVersionsPostDeletesHistoricalVersion(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	versions := make([]*pages_model.Version, 0, 2)
	for _, content := range [][]byte{[]byte("history"), []byte("current")} {
		digest := sha256.Sum256(content)
		version := &pages_model.Version{RepoID: 1, TarGzSHA256: hex.EncodeToString(digest[:]), OriginType: pages_model.VersionOriginUpload}
		require.NoError(t, pages_model.InsertVersion(t.Context(), version))
		require.NoError(t, pages_content.NewContentStore().Save(1, version.TarGzSHA256, bytes.NewReader(content), int64(len(content))))
		versions = append(versions, version)
	}
	require.NoError(t, pages_model.InsertSite(t.Context(), &pages_model.Site{
		RepoID: 1, SourceType: pages_model.PublishSourceArtifact, CurrentVersionID: versions[1].ID, PagesUpdatedUnixNano: 1,
	}))

	ctx, _ := contexttest.MockContext(t, "POST /user2/repo1/settings/pages/versions")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.MockRequestPostForm(ctx.Req, url.Values{
		"action": {"delete-version"}, "version_id": {strconv.FormatInt(versions[0].ID, 10)},
	})
	PagesVersionsPost(ctx)
	require.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
	_, has, err := pages_model.GetVersionByID(t.Context(), versions[0].ID)
	require.NoError(t, err)
	require.False(t, has)
}

func TestPagesVersionsPostUpdatesMetadata(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	version := &pages_model.Version{
		RepoID: 1, TarGzSHA256: strings.Repeat("c", 64), OriginType: pages_model.VersionOriginUpload,
	}
	require.NoError(t, pages_model.InsertVersion(t.Context(), version))
	require.NoError(t, pages_model.InsertSite(t.Context(), &pages_model.Site{
		RepoID: 1, SourceType: pages_model.PublishSourceArtifact,
		CurrentVersionID: version.ID, PagesUpdatedUnixNano: 10,
	}))

	ctx, _ := contexttest.MockContext(t, "POST /user2/repo1/settings/pages/versions")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.MockRequestPostForm(ctx.Req, url.Values{
		"action": {"update-version"}, "version_id": {strconv.FormatInt(version.ID, 10)},
		"comment": {" release candidate "}, "keep_forever": {"true"},
	})
	PagesVersionsPost(ctx)
	require.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
	version, has, err := pages_model.GetVersionByID(t.Context(), version.ID)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, "release candidate", version.Comment)
	require.True(t, version.KeepForever)
	site, has, err := pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, int64(10), site.PagesUpdatedUnixNano)
}

func TestLoadPagesSettingsDataIncludesDomainTimes(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	_, err := pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{Enabled: true, SourceType: pages_model.PublishSourceArtifact})
	require.NoError(t, err)
	require.NoError(t, pages_model.InsertDomain(t.Context(), &pages_model.Domain{
		RepoID: 1, Host: "pages.example.test", TXTToken: "token", State: pages_model.DomainStateActive,
		TXTResult: pages_model.TXTResultInvalid, TXTLastSuccessUnixNano: 1_700_000_100 * int64(time.Second),
		TXTLastCheckUnix: 1_700_000_000, CreatedUnixNano: 1,
	}))

	ctx, _ := contexttest.MockContext(t, "GET /user2/repo1/settings/pages")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 1)
	require.True(t, loadPagesSettingsData(ctx))
	domain, ok := ctx.Data["PagesActiveDomain"].(*pagesDomainView)
	require.True(t, ok)
	require.Equal(t, timeutil.TimeStamp(1_700_000_000), domain.TXTLastCheckUnix)
	require.Equal(t, timeutil.TimeStamp(1_700_000_100), domain.TXTLastSuccessUnix)
	require.Equal(t, true, ctx.Data["PageIsSettingsPagesSettings"])
}

func TestLoadPagesVersionsDataUsesStablePagination(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.UI.ExplorePagingNum, 1)()
	site, err := pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{Enabled: true, SourceType: pages_model.PublishSourceArtifact})
	require.NoError(t, err)
	run := &actions_model.ActionRun{RepoID: 1, Index: 1001, WorkflowID: "pages.yaml", Status: actions_model.StatusSuccess}
	require.NoError(t, db.Insert(t.Context(), run))
	history := &pages_model.Version{RepoID: 1, TarGzSHA256: strings.Repeat("a", 64), OriginType: pages_model.VersionOriginActions, OriginActionRunID: run.ID}
	current := &pages_model.Version{
		RepoID: 1, TarGzSHA256: strings.Repeat("b", 64), OriginType: pages_model.VersionOriginUpload,
		Comment: "current release", KeepForever: true,
	}
	require.NoError(t, db.Insert(t.Context(), history, current))
	site.CurrentVersionID = current.ID
	require.NoError(t, pages_model.UpdateSite(t.Context(), site))

	ctx, _ := contexttest.MockContext(t, "GET /user2/repo1/settings/pages/versions?page=1")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 1)
	require.True(t, loadPagesVersionsData(ctx))
	versionViews, ok := ctx.Data["PagesVersions"].([]*pagesVersionView)
	require.True(t, ok)
	require.Len(t, versionViews, 1)
	require.True(t, versionViews[0].Current)
	require.Equal(t, current.ID, versionViews[0].ID)
	require.Equal(t, current.Comment, versionViews[0].Comment)
	require.True(t, versionViews[0].KeepForever)
	require.EqualValues(t, 2, ctx.Data["Total"])
	require.Equal(t, true, ctx.Data["PageIsSettingsPagesVersions"])

	ctx, _ = contexttest.MockContext(t, "GET /user2/repo1/settings/pages/versions")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 1)
	ctx.Req.Form.Set("page", "2")
	require.True(t, loadPagesVersionsData(ctx))
	versionViews, ok = ctx.Data["PagesVersions"].([]*pagesVersionView)
	require.True(t, ok)
	require.Len(t, versionViews, 1)
	require.False(t, versionViews[0].Current)
	require.Equal(t, history.ID, versionViews[0].ID)
	require.Equal(t, "/user2/repo1/actions/runs/"+strconv.FormatInt(run.ID, 10), versionViews[0].OriginURL)
}

func TestAddDeployKey(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	t.Run("ReadOnly", func(t *testing.T) {
		const testKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICV0MGX/W9IvLA4FXpIuUcdDcbj5KX4syHgsTy7soVgf\n"
		ctx, _ := contexttest.MockContext(t, "POST /user2/repo1/settings/keys")
		contexttest.MockRequestPostForm(ctx.Req, url.Values{"title": {"read-only"}, "content": {testKey}})
		contexttest.LoadRepo(t, ctx, 2)
		DeployKeysPost(ctx)
		assert.Equal(t, http.StatusOK, ctx.Resp.WrittenStatus())
		unittest.AssertExistsAndLoadBean(t, &deploykey_model.DeployKey{Name: "read-only", Mode: perm.AccessModeRead})
	})
	t.Run("ReadWrite", func(t *testing.T) {
		const testKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEHjnNEfE88W1pvBLdV3otv28x760gdmPao3lVD5uAt9\n"
		ctx, _ := contexttest.MockContext(t, "POST /user2/repo1/settings/keys")
		contexttest.MockRequestPostForm(ctx.Req, url.Values{"title": {"read-write"}, "content": {testKey}, "is_writable": {"on"}})
		contexttest.LoadRepo(t, ctx, 2)
		DeployKeysPost(ctx)
		assert.Equal(t, http.StatusOK, ctx.Resp.WrittenStatus())
		unittest.AssertExistsAndLoadBean(t, &deploykey_model.DeployKey{Name: "read-write", Mode: perm.AccessModeWrite})
	})
}

func TestCollaborationPost(t *testing.T) {
	for _, c := range []struct {
		name, collaborator string
		userID             int64
	}{
		{"AddCollaboratorTwice", "user4", 4},
		{"InactiveUser", "user9", 9},
		{"NonExistentUser", "user34", 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert.NoError(t, unittest.PrepareTestDatabase())
			ctx, _ := contexttest.MockContext(t, "user2/repo1/issues/labels")
			contexttest.LoadUser(t, ctx, 2)
			if c.userID != 0 {
				contexttest.LoadUser(t, ctx, c.userID)
			}
			contexttest.LoadRepo(t, ctx, 1)
			ctx.Req.Form.Set("collaborator", c.collaborator)

			owner := &user_model.User{LowerName: "user2", Type: user_model.UserTypeIndividual}
			ctx.Repo = &context.Repository{Owner: owner}
			if c.userID == 4 {
				ctx.Repo.Repository = &repo_model.Repository{ID: 2, Owner: owner}
			}

			CollaborationPost(ctx)
			assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())

			if c.userID == 4 {
				exists, err := repo_model.IsCollaborator(ctx, ctx.Repo.Repository.ID, 4)
				assert.NoError(t, err)
				assert.True(t, exists)

				// Try adding the same collaborator again
				CollaborationPost(ctx)
				assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
			}
			assert.NotEmpty(t, ctx.Flash.ErrorMsg)
		})
	}
}

func TestAddTeamPost(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 26})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 43})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 11})
	repo.Owner = org

	testAddTeamPost := func(t *testing.T, teamName string, repoAdminChangeTeamAccess bool) *context.Context {
		ctx, _ := contexttest.MockContext(t, "org26/repo43")
		ctx.Req.Form.Set("team", teamName)
		org.RepoAdminChangeTeamAccess = repoAdminChangeTeamAccess
		ctx.Repo = &context.Repository{
			Permission: access_model.Permission{AccessMode: perm.AccessModeAdmin},
			Owner:      repo.Owner,
			Repository: repo,
		}
		ctx.Doer = &user_model.User{ID: 1, IsAdmin: true}
		AddTeamPost(ctx)
		return ctx
	}

	t.Run("NonExisting", func(t *testing.T) {
		ctx := testAddTeamPost(t, "team-not-exist", true)
		assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
		assert.Contains(t, ctx.Flash.ErrorMsg, "form.team_not_exist")
	})
	t.Run("NotAllowed", func(t *testing.T) {
		ctx := testAddTeamPost(t, team.Name, false)
		assert.False(t, repo_service.HasRepository(t.Context(), team, repo.ID))
		assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
		assert.Contains(t, ctx.Flash.ErrorMsg, "repo.settings.change_team_access_not_allowed")
	})
	t.Run("Allowed", func(t *testing.T) {
		ctx := testAddTeamPost(t, team.Name, true)
		assert.True(t, repo_service.HasRepository(t.Context(), team, repo.ID))
		assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
		assert.Empty(t, ctx.Flash.ErrorMsg)
		t.Run("Twice", func(t *testing.T) {
			ctx := testAddTeamPost(t, team.Name, true)
			assert.True(t, repo_service.HasRepository(t.Context(), team, repo.ID))
			assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
			assert.Contains(t, ctx.Flash.ErrorMsg, "repo.settings.add_team_duplicate")
		})
	})
}

func TestDeleteTeam(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx, _ := contexttest.MockContext(t, "org3/team1/repo3")

	ctx.Req.Form.Set("id", "2")
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	repo.Owner = org
	org.RepoAdminChangeTeamAccess = true
	ctx.Repo = &context.Repository{
		Permission: access_model.Permission{AccessMode: perm.AccessModeAdmin},
		Owner:      repo.Owner,
		Repository: repo,
	}
	ctx.Doer = &user_model.User{ID: 1, IsAdmin: true}

	assert.True(t, repo_service.HasRepository(t.Context(), team, repo.ID))
	DeleteTeam(ctx)
	assert.False(t, repo_service.HasRepository(t.Context(), team, repo.ID))
}

func TestHandleSettingsPostMirrorPreservesExistingUsername(t *testing.T) {
	defer test.MockVariableValue(&setting.Mirror.Enabled, true)()

	unittest.PrepareTestEnv(t)

	// Use the existing fixture mirror repo (org3/repo5) which has a git repo on disk.
	mirrorRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 5})
	mirror := unittest.AssertExistsAndLoadBean(t, &repo_model.Mirror{RepoID: 5})

	require.NoError(t, mirror_service.UpdateAddress(t.Context(), mirror, "https://existing-user:existing-password@example.com/user2/repo1.git"))

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	ctx, _ := contexttest.MockContext(t, mirrorRepo.Link()+"/settings")
	contexttest.LoadUser(t, ctx, user.ID)
	contexttest.LoadRepo(t, ctx, mirrorRepo.ID)

	web.SetForm(ctx, &forms.RepoSettingForm{
		Interval:       "8h",
		MirrorAddress:  "https://example.com/user2/repo1.git",
		MirrorPassword: "updated-password",
	})

	handleSettingsPostMirror(ctx)

	assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())

	updatedMirror := unittest.AssertExistsAndLoadBean(t, &repo_model.Mirror{RepoID: mirrorRepo.ID})
	assert.Equal(t, "https://example.com/user2/repo1.git", updatedMirror.RemoteAddress)

	updatedRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: mirrorRepo.ID})
	assert.Equal(t, "https://example.com/user2/repo1.git", updatedRepo.OriginalURL)

	remoteURL, err := git.ParseRemoteAddressURL(t.Context(), updatedRepo, updatedMirror.GetRemoteName())
	require.NoError(t, err)
	require.NotNil(t, remoteURL.User)
	assert.Equal(t, "existing-user", remoteURL.User.Username())
	password, ok := remoteURL.User.Password()
	require.True(t, ok)
	assert.Equal(t, "updated-password", password)
}
