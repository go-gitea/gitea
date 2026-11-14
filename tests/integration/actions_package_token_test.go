// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	packages_model "gitea.dev/models/packages"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	packages_service "gitea.dev/services/packages"
	"gitea.dev/tests"

	"github.com/stretchr/testify/require"
)

func prepareActionsPackageToken(t *testing.T) (*actions_model.ActionTask, *actions_model.ActionRunJob, *repo_model.Repository) {
	t.Helper()
	ctx := t.Context()
	task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 47})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: task.RepoID})
	require.NoError(t, repo.LoadOwner(ctx))
	if !repo.UnitEnabled(ctx, unit.TypePackages) {
		require.NoError(t, db.Insert(ctx, &repo_model.RepoUnit{RepoID: repo.ID, Type: unit.TypePackages, Config: &repo_model.UnitConfig{}}))
	}
	task.GenerateAndFillToken()
	task.Status = actions_model.StatusRunning
	require.NoError(t, actions_model.UpdateTask(ctx, task, "token_hash", "token_salt", "token_last_eight", "status"))
	job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: task.JobID})
	permissions := repo_model.MakeActionsTokenPermissions(perm.AccessModeNone)
	permissions.UnitAccessModes[unit.TypePackages] = perm.AccessModeWrite
	job.TokenPermissions = &permissions
	_, err := db.GetEngine(ctx).ID(job.ID).Cols("token_permissions").Update(job)
	require.NoError(t, err)
	return task, job, repo
}

func TestActionsPackageTokenFormats(t *testing.T) {
	for _, packageType := range []packages_model.Type{packages_model.TypeGeneric, packages_model.TypeMaven, packages_model.TypeNpm, packages_model.TypePyPI} {
		t.Run(string(packageType), func(t *testing.T) {
			defer tests.PrepareTestEnv(t)()
			task, _, repo := prepareActionsPackageToken(t)
			root := "/api/packages/" + repo.Owner.LowerName
			name := "actions-package"
			request := func() *RequestWrapper {
				switch packageType {
				case packages_model.TypeGeneric:
					return NewRequestWithBody(t, http.MethodPut, root+"/generic/"+name+"/1.0.0/test.txt", strings.NewReader("test"))
				case packages_model.TypeMaven:
					name = "com.gitea:actions-package"
					return NewRequestWithBody(t, http.MethodPut, root+"/maven/com/gitea/actions-package/1.0.0/artifact.jar", strings.NewReader("test"))
				case packages_model.TypeNpm:
					digest := sha512.Sum512([]byte("test"))
					body := fmt.Sprintf(`{"name":%q,"dist-tags":{"latest":"1.0.0"},"versions":{"1.0.0":{"name":%q,"version":"1.0.0","dist":{"integrity":"sha512-%s"}}},"_attachments":{"actions-package-1.0.0.tgz":{"data":"dGVzdA=="}}}`, name, name, base64.StdEncoding.EncodeToString(digest[:]))
					return NewRequestWithBody(t, http.MethodPut, root+"/npm/"+name, strings.NewReader(body)).SetHeader("Content-Type", "application/json")
				case packages_model.TypePyPI:
					body := &bytes.Buffer{}
					writer := multipart.NewWriter(body)
					part, err := writer.CreateFormFile("content", "actions_package-1.0.0.tar.gz")
					require.NoError(t, err)
					_, err = part.Write([]byte("test"))
					require.NoError(t, err)
					digest := sha256.Sum256([]byte("test"))
					require.NoError(t, writer.WriteField("name", name))
					require.NoError(t, writer.WriteField("version", "1.0.0"))
					require.NoError(t, writer.WriteField("sha256_digest", hex.EncodeToString(digest[:])))
					require.NoError(t, writer.Close())
					return NewRequestWithBody(t, http.MethodPost, root+"/pypi", body).SetHeader("Content-Type", writer.FormDataContentType())
				default:
					t.Fatal("unsupported test package type")
					return nil
				}
			}
			upload := request()
			upload.SetBasicAuth(repo.Owner.Name, task.Token)
			MakeRequest(t, upload, http.StatusCreated)
			pkg := unittest.AssertExistsAndLoadBean(t, &packages_model.Package{OwnerID: repo.OwnerID, Type: packageType, Name: name})
			require.Equal(t, repo.ID, pkg.RepoID)
			_, err := db.GetEngine(t.Context()).ID(pkg.ID).Cols("repo_id").Update(&packages_model.Package{RepoID: repo.ID + 1})
			require.NoError(t, err)
			upload = request()
			upload.SetBasicAuth(repo.Owner.Name, task.Token)
			MakeRequest(t, upload, http.StatusForbidden)
			unittest.AssertCount(t, &packages_model.PackageVersion{PackageID: pkg.ID}, 1)
		})
	}
}

func TestActionsPackageTokenPermissions(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	task, job, repo := prepareActionsPackageToken(t)
	root := "/api/packages/" + repo.Owner.LowerName + "/generic/actions-package/1.0.0/"
	registryToken, err := packages_service.CreateAuthorizationToken(user_model.NewActionsUserWithTaskID(task.ID), auth_model.AccessTokenScopeWritePackage)
	require.NoError(t, err)
	credentials := []string{"basic", task.Token, registryToken}
	upload := func(credential, filename string, status int) {
		t.Helper()
		request := NewRequestWithBody(t, http.MethodPut, root+filename, strings.NewReader("test"))
		if credential == "basic" {
			request.SetBasicAuth(repo.Owner.Name, task.Token)
		} else {
			request.AddTokenAuth("Bearer " + credential)
		}
		MakeRequest(t, request, status)
	}
	for index, credential := range credentials {
		upload(credential, fmt.Sprintf("test%d.txt", index), http.StatusCreated)
	}
	for _, mode := range []perm.AccessMode{perm.AccessModeRead, perm.AccessModeNone} {
		job.TokenPermissions.UnitAccessModes[unit.TypePackages] = mode
		_, err := db.GetEngine(t.Context()).ID(job.ID).Cols("token_permissions").Update(job)
		require.NoError(t, err)
		for _, credential := range credentials {
			upload(credential, "denied.txt", http.StatusUnauthorized)
		}
	}
	job.TokenPermissions.UnitAccessModes[unit.TypePackages] = perm.AccessModeWrite
	_, err = db.GetEngine(t.Context()).ID(job.ID).Cols("token_permissions").Update(job)
	require.NoError(t, err)
	task.IsForkPullRequest = true
	require.NoError(t, actions_model.UpdateTask(t.Context(), task, "is_fork_pull_request"))
	upload("basic", "fork.txt", http.StatusUnauthorized)
	task.IsForkPullRequest = false
	task.Status = actions_model.StatusSuccess
	require.NoError(t, actions_model.UpdateTask(t.Context(), task, "is_fork_pull_request", "status"))
	for _, credential := range credentials {
		upload(credential, "finished.txt", http.StatusUnauthorized)
	}
	unittest.AssertCount(t, &packages_model.PackageFile{}, 3)
}
