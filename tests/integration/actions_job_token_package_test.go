// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	packages_model "gitea.dev/models/packages"
	repo_model "gitea.dev/models/repo"
	unit_model "gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	container_module "gitea.dev/modules/packages/container"
	"gitea.dev/modules/setting"
	"gitea.dev/tests"

	"github.com/stretchr/testify/require"
)

// TestActionsJobTokenPackagePush verifies that the automatic Actions token (GITEA_TOKEN)
// can log in to the container registry and push to its own owner's registry, honoring the
// workflow permissions/clamps: permissive grants write, restricted/fork/cross-owner do not.
func TestActionsJobTokenPackagePush(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	otherOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2, OwnerID: owner.ID})

	// repo 2 has no Actions unit in fixtures; create one so we can flip its token mode.
	require.NoError(t, db.Insert(t.Context(), &repo_model.RepoUnit{
		RepoID: repo.ID,
		Type:   unit_model.TypeActions,
		Config: &repo_model.ActionsConfig{},
	}))

	setTokenMode := func(t *testing.T, mode repo_model.ActionsTokenPermissionMode) {
		r := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
		u := r.MustGetUnit(t.Context(), unit_model.TypeActions)
		cfg := u.ActionsConfig()
		cfg.OverrideOwnerConfig = true
		cfg.TokenPermissionMode = mode
		cfg.MaxTokenPermissions = nil
		require.NoError(t, repo_model.UpdateRepoUnitConfig(t.Context(), u))
	}

	blobDigest := "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
	blobContent, _ := base64.StdEncoding.DecodeString(`H4sIAAAJbogA/2IYBaNgFIxYAAgAAP//Lq+17wAEAAA=`)
	configDigest := "sha256:4607e093bec406eaadb6f3a340f63400c9d3a7038680744c406903766b938f0d"
	configContent := `{"architecture":"amd64","config":{"Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],"Cmd":["/true"],"ArgsEscaped":true,"Image":"sha256:9bd8b88dc68b80cffe126cc820e4b52c6e558eb3b37680bfee8e5f3ed7b8c257"},"container":"b89fe92a887d55c0961f02bdfbfd8ac3ddf66167db374770d2d9e9fab3311510","container_config":{"Hostname":"b89fe92a887d","Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],"Cmd":["/bin/sh","-c","#(nop) ","CMD [\"/true\"]"],"ArgsEscaped":true,"Image":"sha256:9bd8b88dc68b80cffe126cc820e4b52c6e558eb3b37680bfee8e5f3ed7b8c257"},"created":"2022-01-01T00:00:00.000000000Z","docker_version":"20.10.12","history":[{"created":"2022-01-01T00:00:00.000000000Z","created_by":"/bin/sh -c #(nop) COPY file:0e7589b0c800daaf6fa460d2677101e4676dd9491980210cb345480e513f3602 in /true "},{"created":"2022-01-01T00:00:00.000000001Z","created_by":"/bin/sh -c #(nop)  CMD [\"/true\"]","empty_layer":true}],"os":"linux","rootfs":{"type":"layers","diff_ids":["sha256:0ff3b91bdf21ecdf2f2f3d4372c2098a14dbe06cd678e8f0a85fd4902d00e2e2"]}}`
	manifestContent := `{"schemaVersion":2,"mediaType":"` + container_module.ContentTypeDockerDistributionManifestV2 + `","config":{"mediaType":"application/vnd.docker.container.image.v1+json","digest":"` + configDigest + `","size":1069},"layers":[{"mediaType":"application/vnd.docker.image.rootfs.diff.tar.gzip","digest":"` + blobDigest + `","size":32}]}`

	// dockerLogin performs the "GET /v2/token" exchange with the task token as basic auth,
	// mirroring what `docker login -u gitea-actions -p $GITEA_TOKEN` does.
	dockerLogin := func(t *testing.T, task *actions_model.ActionTask) (bearer string, status int) {
		req := NewRequest(t, "GET", setting.AppURL+"v2/token").AddBasicAuth(user_model.ActionsUserName, task.Token)
		resp := MakeRequest(t, req, NoExpectedStatus)
		if resp.Code != http.StatusOK {
			return "", resp.Code
		}
		var tokenResponse struct {
			Token string `json:"token"`
		}
		DecodeJSON(t, resp, &tokenResponse)
		require.NotEmpty(t, tokenResponse.Token)
		return "Bearer " + tokenResponse.Token, resp.Code
	}

	uploadBlob := func(t *testing.T, bearer, ownerName, image, digest, content string) int {
		req := NewRequestWithBody(t, "POST", fmt.Sprintf("%sv2/%s/%s/blobs/uploads?digest=%s", setting.AppURL, ownerName, image, digest), strings.NewReader(content)).
			AddTokenAuth(bearer)
		return MakeRequest(t, req, NoExpectedStatus).Code
	}

	t.Run("PermissiveCanPush", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		setTokenMode(t, repo_model.ActionsTokenPermissionModePermissive)
		task := createActionTask(t, repo.ID, false)

		bearer, status := dockerLogin(t, task)
		require.Equal(t, http.StatusOK, status)

		image := "actions-image"
		require.Equal(t, http.StatusCreated, uploadBlob(t, bearer, owner.Name, image, configDigest, configContent))
		require.Equal(t, http.StatusCreated, uploadBlob(t, bearer, owner.Name, image, blobDigest, string(blobContent)))

		req := NewRequestWithBody(t, "PUT", fmt.Sprintf("%sv2/%s/%s/manifests/latest", setting.AppURL, owner.Name, image), strings.NewReader(manifestContent)).
			AddTokenAuth(bearer).
			SetHeader("Content-Type", container_module.ContentTypeDockerDistributionManifestV2)
		MakeRequest(t, req, http.StatusCreated)

		_, err := packages_model.GetInternalVersionByNameAndVersion(t.Context(), owner.ID, packages_model.TypeContainer, image, container_module.UploadVersion)
		require.NoError(t, err)
	})

	t.Run("RestrictedCannotPush", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		setTokenMode(t, repo_model.ActionsTokenPermissionModeRestricted)
		task := createActionTask(t, repo.ID, false)

		bearer, status := dockerLogin(t, task)
		require.Equal(t, http.StatusOK, status) // login still succeeds (read scope)
		require.Equal(t, http.StatusUnauthorized, uploadBlob(t, bearer, owner.Name, "restricted-image", blobDigest, string(blobContent)))
	})

	t.Run("ForkPullRequestCannotPush", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		setTokenMode(t, repo_model.ActionsTokenPermissionModePermissive)
		task := createActionTask(t, repo.ID, true) // fork PR is read-only even under permissive

		bearer, status := dockerLogin(t, task)
		require.Equal(t, http.StatusOK, status)
		require.Equal(t, http.StatusUnauthorized, uploadBlob(t, bearer, owner.Name, "fork-image", blobDigest, string(blobContent)))
	})

	t.Run("CrossOwnerCannotPush", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		setTokenMode(t, repo_model.ActionsTokenPermissionModePermissive)
		task := createActionTask(t, repo.ID, false)

		bearer, status := dockerLogin(t, task)
		require.Equal(t, http.StatusOK, status)
		// Packages are owner-scoped: a token may only write to its own owner's registry.
		require.Equal(t, http.StatusUnauthorized, uploadBlob(t, bearer, otherOwner.Name, "cross-image", blobDigest, string(blobContent)))
	})
}
