// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/storage"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/actions"
	actions_service "code.gitea.io/gitea/services/actions"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func toProtoJSON(m protoreflect.ProtoMessage) io.Reader {
	resp, _ := protojson.Marshal(m)
	buf := bytes.Buffer{}
	buf.Write(resp)
	return &buf
}

func TestActionsArtifactV4UploadSingleFile(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	token, err := actions_service.CreateAuthorizationToken(48, 792, 193)
	assert.NoError(t, err)

	// acquire artifact upload url
	req := NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/CreateArtifact", toProtoJSON(&actions.CreateArtifactRequest{
		Version:                 4,
		Name:                    "artifact",
		WorkflowRunBackendId:    "792",
		WorkflowJobRunBackendId: "193",
	})).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var uploadResp actions.CreateArtifactResponse
	protojson.Unmarshal(resp.Body.Bytes(), &uploadResp)
	assert.True(t, uploadResp.Ok)
	assert.Contains(t, uploadResp.SignedUploadUrl, "/twirp/github.actions.results.api.v1.ArtifactService/UploadArtifact")

	// get upload url
	idx := strings.Index(uploadResp.SignedUploadUrl, "/twirp/")
	url := uploadResp.SignedUploadUrl[idx:] + "&comp=block"

	// upload artifact chunk
	body := strings.Repeat("A", 1024)
	req = NewRequestWithBody(t, "PUT", url, strings.NewReader(body))
	MakeRequest(t, req, http.StatusCreated)

	t.Logf("Create artifact confirm")

	sha := sha256.Sum256([]byte(body))

	// confirm artifact upload
	req = NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/FinalizeArtifact", toProtoJSON(&actions.FinalizeArtifactRequest{
		Name:                    "artifact",
		Size:                    1024,
		Hash:                    wrapperspb.String("sha256:" + hex.EncodeToString(sha[:])),
		WorkflowRunBackendId:    "792",
		WorkflowJobRunBackendId: "193",
	})).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	var finalizeResp actions.FinalizeArtifactResponse
	protojson.Unmarshal(resp.Body.Bytes(), &finalizeResp)
	assert.True(t, finalizeResp.Ok)
}

func TestActionsArtifactV4UploadSingleFileWrongChecksum(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	token, err := actions_service.CreateAuthorizationToken(48, 792, 193)
	assert.NoError(t, err)

	// acquire artifact upload url
	req := NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/CreateArtifact", toProtoJSON(&actions.CreateArtifactRequest{
		Version:                 4,
		Name:                    "artifact-invalid-checksum",
		WorkflowRunBackendId:    "792",
		WorkflowJobRunBackendId: "193",
	})).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var uploadResp actions.CreateArtifactResponse
	protojson.Unmarshal(resp.Body.Bytes(), &uploadResp)
	assert.True(t, uploadResp.Ok)
	assert.Contains(t, uploadResp.SignedUploadUrl, "/twirp/github.actions.results.api.v1.ArtifactService/UploadArtifact")

	// get upload url
	idx := strings.Index(uploadResp.SignedUploadUrl, "/twirp/")
	url := uploadResp.SignedUploadUrl[idx:] + "&comp=block"

	// upload artifact chunk
	body := strings.Repeat("B", 1024)
	req = NewRequestWithBody(t, "PUT", url, strings.NewReader(body))
	MakeRequest(t, req, http.StatusCreated)

	t.Logf("Create artifact confirm")

	sha := sha256.Sum256([]byte(strings.Repeat("A", 1024)))

	// confirm artifact upload
	req = NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/FinalizeArtifact", toProtoJSON(&actions.FinalizeArtifactRequest{
		Name:                    "artifact-invalid-checksum",
		Size:                    1024,
		Hash:                    wrapperspb.String("sha256:" + hex.EncodeToString(sha[:])),
		WorkflowRunBackendId:    "792",
		WorkflowJobRunBackendId: "193",
	})).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusInternalServerError)
}

func TestActionsArtifactV4UploadSingleFileWithRetentionDays(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	token, err := actions_service.CreateAuthorizationToken(48, 792, 193)
	assert.NoError(t, err)

	// acquire artifact upload url
	req := NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/CreateArtifact", toProtoJSON(&actions.CreateArtifactRequest{
		Version:                 4,
		ExpiresAt:               timestamppb.New(time.Now().Add(5 * 24 * time.Hour)),
		Name:                    "artifactWithRetentionDays",
		WorkflowRunBackendId:    "792",
		WorkflowJobRunBackendId: "193",
	})).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var uploadResp actions.CreateArtifactResponse
	protojson.Unmarshal(resp.Body.Bytes(), &uploadResp)
	assert.True(t, uploadResp.Ok)
	assert.Contains(t, uploadResp.SignedUploadUrl, "/twirp/github.actions.results.api.v1.ArtifactService/UploadArtifact")

	// get upload url
	idx := strings.Index(uploadResp.SignedUploadUrl, "/twirp/")
	url := uploadResp.SignedUploadUrl[idx:] + "&comp=block"

	// upload artifact chunk
	body := strings.Repeat("A", 1024)
	req = NewRequestWithBody(t, "PUT", url, strings.NewReader(body))
	MakeRequest(t, req, http.StatusCreated)

	t.Logf("Create artifact confirm")

	sha := sha256.Sum256([]byte(body))

	// confirm artifact upload
	req = NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/FinalizeArtifact", toProtoJSON(&actions.FinalizeArtifactRequest{
		Name:                    "artifactWithRetentionDays",
		Size:                    1024,
		Hash:                    wrapperspb.String("sha256:" + hex.EncodeToString(sha[:])),
		WorkflowRunBackendId:    "792",
		WorkflowJobRunBackendId: "193",
	})).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	var finalizeResp actions.FinalizeArtifactResponse
	protojson.Unmarshal(resp.Body.Bytes(), &finalizeResp)
	assert.True(t, finalizeResp.Ok)
}

func TestActionsArtifactV4UploadSingleFileWithPotentialHarmfulBlockID(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	token, err := actions_service.CreateAuthorizationToken(48, 792, 193)
	assert.NoError(t, err)

	// acquire artifact upload url
	req := NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/CreateArtifact", toProtoJSON(&actions.CreateArtifactRequest{
		Version:                 4,
		Name:                    "artifactWithPotentialHarmfulBlockID",
		WorkflowRunBackendId:    "792",
		WorkflowJobRunBackendId: "193",
	})).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var uploadResp actions.CreateArtifactResponse
	protojson.Unmarshal(resp.Body.Bytes(), &uploadResp)
	assert.True(t, uploadResp.Ok)
	assert.Contains(t, uploadResp.SignedUploadUrl, "/twirp/github.actions.results.api.v1.ArtifactService/UploadArtifact")

	// get upload urls
	idx := strings.Index(uploadResp.SignedUploadUrl, "/twirp/")
	url := uploadResp.SignedUploadUrl[idx:] + "&comp=block&blockid=%2f..%2fmyfile"
	blockListURL := uploadResp.SignedUploadUrl[idx:] + "&comp=blocklist"

	// upload artifact chunk
	body := strings.Repeat("A", 1024)
	req = NewRequestWithBody(t, "PUT", url, strings.NewReader(body))
	MakeRequest(t, req, http.StatusCreated)

	// verify that the exploit didn't work
	_, err = storage.Actions.Stat("myfile")
	assert.Error(t, err)

	// upload artifact blockList
	blockList := &actions.BlockList{
		Latest: []string{
			"/../myfile",
		},
	}
	rawBlockList, err := xml.Marshal(blockList)
	assert.NoError(t, err)
	req = NewRequestWithBody(t, "PUT", blockListURL, bytes.NewReader(rawBlockList))
	MakeRequest(t, req, http.StatusCreated)

	t.Logf("Create artifact confirm")

	sha := sha256.Sum256([]byte(body))

	// confirm artifact upload
	req = NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/FinalizeArtifact", toProtoJSON(&actions.FinalizeArtifactRequest{
		Name:                    "artifactWithPotentialHarmfulBlockID",
		Size:                    1024,
		Hash:                    wrapperspb.String("sha256:" + hex.EncodeToString(sha[:])),
		WorkflowRunBackendId:    "792",
		WorkflowJobRunBackendId: "193",
	})).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	var finalizeResp actions.FinalizeArtifactResponse
	protojson.Unmarshal(resp.Body.Bytes(), &finalizeResp)
	assert.True(t, finalizeResp.Ok)
}

func TestActionsArtifactV4UploadSingleFileWithChunksOutOfOrder(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	token, err := actions_service.CreateAuthorizationToken(48, 792, 193)
	assert.NoError(t, err)

	// acquire artifact upload url
	req := NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/CreateArtifact", toProtoJSON(&actions.CreateArtifactRequest{
		Version:                 4,
		Name:                    "artifactWithChunksOutOfOrder",
		WorkflowRunBackendId:    "792",
		WorkflowJobRunBackendId: "193",
	})).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var uploadResp actions.CreateArtifactResponse
	protojson.Unmarshal(resp.Body.Bytes(), &uploadResp)
	assert.True(t, uploadResp.Ok)
	assert.Contains(t, uploadResp.SignedUploadUrl, "/twirp/github.actions.results.api.v1.ArtifactService/UploadArtifact")

	// get upload urls
	idx := strings.Index(uploadResp.SignedUploadUrl, "/twirp/")
	block1URL := uploadResp.SignedUploadUrl[idx:] + "&comp=block&blockid=block1"
	block2URL := uploadResp.SignedUploadUrl[idx:] + "&comp=block&blockid=block2"
	blockListURL := uploadResp.SignedUploadUrl[idx:] + "&comp=blocklist"

	// upload artifact chunks
	bodyb := strings.Repeat("B", 1024)
	req = NewRequestWithBody(t, "PUT", block2URL, strings.NewReader(bodyb))
	MakeRequest(t, req, http.StatusCreated)

	bodya := strings.Repeat("A", 1024)
	req = NewRequestWithBody(t, "PUT", block1URL, strings.NewReader(bodya))
	MakeRequest(t, req, http.StatusCreated)

	// upload artifact blockList
	blockList := &actions.BlockList{
		Latest: []string{
			"block1",
			"block2",
		},
	}
	rawBlockList, err := xml.Marshal(blockList)
	assert.NoError(t, err)
	req = NewRequestWithBody(t, "PUT", blockListURL, bytes.NewReader(rawBlockList))
	MakeRequest(t, req, http.StatusCreated)

	t.Logf("Create artifact confirm")

	sha := sha256.Sum256([]byte(bodya + bodyb))

	// confirm artifact upload
	req = NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/FinalizeArtifact", toProtoJSON(&actions.FinalizeArtifactRequest{
		Name:                    "artifactWithChunksOutOfOrder",
		Size:                    2048,
		Hash:                    wrapperspb.String("sha256:" + hex.EncodeToString(sha[:])),
		WorkflowRunBackendId:    "792",
		WorkflowJobRunBackendId: "193",
	})).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	var finalizeResp actions.FinalizeArtifactResponse
	protojson.Unmarshal(resp.Body.Bytes(), &finalizeResp)
	assert.True(t, finalizeResp.Ok)
}

func TestActionsArtifactV4DownloadSingle(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	token, err := actions_service.CreateAuthorizationToken(48, 792, 193)
	assert.NoError(t, err)

	// acquire artifact upload url
	req := NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/ListArtifacts", toProtoJSON(&actions.ListArtifactsRequest{
		NameFilter:              wrapperspb.String("artifact-v4-download"),
		WorkflowRunBackendId:    "792",
		WorkflowJobRunBackendId: "193",
	})).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var listResp actions.ListArtifactsResponse
	protojson.Unmarshal(resp.Body.Bytes(), &listResp)
	assert.Len(t, listResp.Artifacts, 1)

	// confirm artifact upload
	req = NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/GetSignedArtifactURL", toProtoJSON(&actions.GetSignedArtifactURLRequest{
		Name:                    "artifact-v4-download",
		WorkflowRunBackendId:    "792",
		WorkflowJobRunBackendId: "193",
	})).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	var finalizeResp actions.GetSignedArtifactURLResponse
	protojson.Unmarshal(resp.Body.Bytes(), &finalizeResp)
	assert.NotEmpty(t, finalizeResp.SignedUrl)

	req = NewRequest(t, "GET", finalizeResp.SignedUrl)
	resp = MakeRequest(t, req, http.StatusOK)
	body := strings.Repeat("D", 1024)
	assert.Equal(t, body, resp.Body.String())
}

func TestActionsArtifactV4RunDownloadSinglePublicApi(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	// confirm artifact can be listed and found by name
	req := NewRequestWithBody(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/792/artifacts?name=artifact-v4-download", repo.FullName()), nil).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var listResp api.ActionArtifactsResponse
	err := json.Unmarshal(resp.Body.Bytes(), &listResp)
	assert.NoError(t, err)
	assert.NotEmpty(t, listResp.Entries[0].ArchiveDownloadURL)
	assert.Equal(t, "artifact-v4-download", listResp.Entries[0].Name)

	// confirm artifact blob storage url can be retrieved
	req = NewRequestWithBody(t, "GET", listResp.Entries[0].ArchiveDownloadURL, nil).
		AddTokenAuth(token)

	resp = MakeRequest(t, req, http.StatusFound)

	// confirm artifact can be downloaded and has expected content
	req = NewRequestWithBody(t, "GET", resp.Header().Get("Location"), nil).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)

	body := strings.Repeat("D", 1024)
	assert.Equal(t, body, resp.Body.String())
}

func TestActionsArtifactV4DownloadSinglePublicApi(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	// confirm artifact can be listed and found by name
	req := NewRequestWithBody(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/artifacts?name=artifact-v4-download", repo.FullName()), nil).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var listResp api.ActionArtifactsResponse
	err := json.Unmarshal(resp.Body.Bytes(), &listResp)
	assert.NoError(t, err)
	assert.NotEmpty(t, listResp.Entries[0].ArchiveDownloadURL)
	assert.Equal(t, "artifact-v4-download", listResp.Entries[0].Name)

	// confirm artifact blob storage url can be retrieved
	req = NewRequestWithBody(t, "GET", listResp.Entries[0].ArchiveDownloadURL, nil).
		AddTokenAuth(token)

	resp = MakeRequest(t, req, http.StatusFound)

	blobLocation := resp.Header().Get("Location")

	// confirm artifact can be downloaded without token and has expected content
	req = NewRequestWithBody(t, "GET", blobLocation, nil)
	resp = MakeRequest(t, req, http.StatusOK)
	body := strings.Repeat("D", 1024)
	assert.Equal(t, body, resp.Body.String())

	// confirm artifact can not be downloaded without query
	req = NewRequestWithBody(t, "GET", blobLocation, nil)
	req.URL.RawQuery = ""
	_ = MakeRequest(t, req, http.StatusUnauthorized)
}

func TestActionsArtifactV4DownloadSinglePublicApiPrivateRepo(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	// confirm artifact can be listed and found by name
	req := NewRequestWithBody(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/artifacts?name=artifact-v4-download", repo.FullName()), nil).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var listResp api.ActionArtifactsResponse
	err := json.Unmarshal(resp.Body.Bytes(), &listResp)
	assert.NoError(t, err)
	assert.Equal(t, int64(23), listResp.Entries[0].ID)
	assert.NotEmpty(t, listResp.Entries[0].ArchiveDownloadURL)
	assert.Equal(t, "artifact-v4-download", listResp.Entries[0].Name)

	// confirm artifact blob storage url can be retrieved
	req = NewRequestWithBody(t, "GET", listResp.Entries[0].ArchiveDownloadURL, nil).
		AddTokenAuth(token)

	resp = MakeRequest(t, req, http.StatusFound)

	blobLocation := resp.Header().Get("Location")
	// confirm artifact can be downloaded without token and has expected content
	req = NewRequestWithBody(t, "GET", blobLocation, nil)
	resp = MakeRequest(t, req, http.StatusOK)
	body := strings.Repeat("D", 1024)
	assert.Equal(t, body, resp.Body.String())

	// confirm artifact can not be downloaded without query
	req = NewRequestWithBody(t, "GET", blobLocation, nil)
	req.URL.RawQuery = ""
	_ = MakeRequest(t, req, http.StatusUnauthorized)
}

func TestActionsArtifactV4ListAndGetPublicApi(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	// confirm artifact can be listed
	req := NewRequestWithBody(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/artifacts", repo.FullName()), nil).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var listResp api.ActionArtifactsResponse
	err := json.Unmarshal(resp.Body.Bytes(), &listResp)
	assert.NoError(t, err)

	for _, artifact := range listResp.Entries {
		assert.Contains(t, artifact.URL, fmt.Sprintf("/api/v1/repos/%s/actions/artifacts/%d", repo.FullName(), artifact.ID))
		assert.Contains(t, artifact.ArchiveDownloadURL, fmt.Sprintf("/api/v1/repos/%s/actions/artifacts/%d/zip", repo.FullName(), artifact.ID))
		req = NewRequestWithBody(t, "GET", listResp.Entries[0].URL, nil).
			AddTokenAuth(token)

		resp = MakeRequest(t, req, http.StatusOK)
		var artifactResp api.ActionArtifact
		err := json.Unmarshal(resp.Body.Bytes(), &artifactResp)
		assert.NoError(t, err)

		assert.Equal(t, artifact.ID, artifactResp.ID)
		assert.Equal(t, artifact.Name, artifactResp.Name)
		assert.Equal(t, artifact.SizeInBytes, artifactResp.SizeInBytes)
		assert.Equal(t, artifact.URL, artifactResp.URL)
		assert.Equal(t, artifact.ArchiveDownloadURL, artifactResp.ArchiveDownloadURL)
	}
}

func TestActionsArtifactV4GetArtifactMismatchedRepoNotFound(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	// confirm artifacts of wrong repo is not visible
	req := NewRequestWithBody(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/artifacts/%d", repo.FullName(), 22), nil).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestActionsArtifactV4DownloadArtifactMismatchedRepoNotFound(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	// confirm artifacts of wrong repo is not visible
	req := NewRequestWithBody(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/artifacts/%d/zip", repo.FullName(), 22), nil).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestActionsArtifactV4DownloadArtifactCorrectRepoFound(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	// confirm artifacts of correct repo is visible
	req := NewRequestWithBody(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/artifacts/%d/zip", repo.FullName(), 22), nil).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusFound)
}

func TestActionsArtifactV4DownloadRawArtifactCorrectRepoMissingSignatureUnauthorized(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	// confirm cannot use the raw artifact endpoint even with a correct access token
	req := NewRequestWithBody(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/artifacts/%d/zip/raw", repo.FullName(), 22), nil).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestActionsArtifactV4Delete(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	token, err := actions_service.CreateAuthorizationToken(48, 792, 193)
	assert.NoError(t, err)

	// delete artifact by name
	req := NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/DeleteArtifact", toProtoJSON(&actions.DeleteArtifactRequest{
		Name:                    "artifact-v4-download",
		WorkflowRunBackendId:    "792",
		WorkflowJobRunBackendId: "193",
	})).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var deleteResp actions.DeleteArtifactResponse
	protojson.Unmarshal(resp.Body.Bytes(), &deleteResp)
	assert.True(t, deleteResp.Ok)
}

func TestActionsArtifactV4DeletePublicApi(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	// confirm artifacts exists
	req := NewRequestWithBody(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/artifacts/%d", repo.FullName(), 22), nil).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	// delete artifact by id
	req = NewRequestWithBody(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/actions/artifacts/%d", repo.FullName(), 22), nil).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// confirm artifacts has been deleted
	req = NewRequestWithBody(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/artifacts/%d", repo.FullName(), 22), nil).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestActionsArtifactV4DeletePublicApiNotAllowedReadScope(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	// confirm artifacts exists
	req := NewRequestWithBody(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/artifacts/%d", repo.FullName(), 22), nil).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	// try delete artifact by id
	req = NewRequestWithBody(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/actions/artifacts/%d", repo.FullName(), 22), nil).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)

	// confirm artifacts has not been deleted
	req = NewRequestWithBody(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/artifacts/%d", repo.FullName(), 22), nil).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)
}
