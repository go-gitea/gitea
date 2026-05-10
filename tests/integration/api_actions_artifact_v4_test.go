// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/actions"
	actions_service "code.gitea.io/gitea/services/actions"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	table := []struct {
		name        string
		version     int32
		contentType string
		blockID     bool
		noLength    bool
		append      int
		path        string
	}{
		{
			name:    "artifact",
			version: 4,
			path:    "artifact.zip",
		},
		{
			name:    "artifact2",
			version: 4,
			blockID: true,
		},
		{
			name:     "artifact3",
			version:  4,
			noLength: true,
		},
		{
			name:     "artifact4",
			version:  4,
			blockID:  true,
			noLength: true,
		},
		{
			name:    "artifact5",
			version: 7,
			blockID: true,
		},
		{
			name:     "artifact6",
			version:  7,
			append:   2,
			noLength: true,
		},
		{
			name:     "artifact7",
			version:  7,
			append:   3,
			blockID:  true,
			noLength: true,
		},
		{
			name:    "artifact8",
			version: 7,
			append:  4,
			blockID: true,
		},
		{
			name:        "artifact9.json",
			version:     7,
			contentType: "application/json",
		},
		{
			name:        "artifact10",
			version:     7,
			contentType: "application/zip",
			path:        "artifact10.zip",
		},
		{
			name:        "artifact11.zip",
			version:     7,
			contentType: "application/zip",
			path:        "artifact11.zip",
		},
	}

	for _, entry := range table {
		t.Run(entry.name, func(t *testing.T) {
			// acquire artifact upload url
			req := NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/CreateArtifact", toProtoJSON(&actions.CreateArtifactRequest{
				Version:                 entry.version,
				Name:                    entry.name,
				WorkflowRunBackendId:    "792",
				WorkflowJobRunBackendId: "193",
				MimeType:                util.Iif(entry.contentType != "", wrapperspb.String(entry.contentType), nil),
			})).AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			var uploadResp actions.CreateArtifactResponse
			protojson.Unmarshal(resp.Body.Bytes(), &uploadResp)
			assert.True(t, uploadResp.Ok)
			assert.Contains(t, uploadResp.SignedUploadUrl, "/twirp/github.actions.results.api.v1.ArtifactService/UploadArtifact")

			h := sha256.New()

			blocks := make([]string, 0, util.Iif(entry.blockID, entry.append+1, 0))

			// get upload url
			for i := range entry.append + 1 {
				url := uploadResp.SignedUploadUrl
				// See https://learn.microsoft.com/en-us/rest/api/storageservices/append-block
				// See https://learn.microsoft.com/en-us/rest/api/storageservices/put-block
				if entry.blockID {
					blockID := base64.RawURLEncoding.EncodeToString(fmt.Append([]byte("SOME_BIG_BLOCK_ID_"), i))
					blocks = append(blocks, blockID)
					url += "&comp=block&blockid=" + blockID
				} else {
					url += "&comp=appendBlock"
				}

				// upload artifact chunk
				body := strings.Repeat("A", 1024)
				_, _ = h.Write([]byte(body))
				var bodyReader io.Reader = strings.NewReader(body)
				if entry.noLength {
					bodyReader = io.MultiReader(bodyReader)
				}
				req = NewRequestWithBody(t, "PUT", url, bodyReader)
				MakeRequest(t, req, http.StatusCreated)
			}

			if entry.blockID && entry.append > 0 {
				// https://learn.microsoft.com/en-us/rest/api/storageservices/put-block-list
				blockListURL := uploadResp.SignedUploadUrl + "&comp=blocklist"
				// upload artifact blockList
				blockList := &actions.BlockList{
					Latest: blocks,
				}
				rawBlockList, err := xml.Marshal(blockList)
				assert.NoError(t, err)
				req = NewRequestWithBody(t, "PUT", blockListURL, bytes.NewReader(rawBlockList))
				MakeRequest(t, req, http.StatusCreated)
			}

			sha := h.Sum(nil)

			t.Logf("Create artifact confirm")

			// confirm artifact upload
			req = NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/FinalizeArtifact", toProtoJSON(&actions.FinalizeArtifactRequest{
				Name:                    entry.name,
				Size:                    int64(entry.append+1) * 1024,
				Hash:                    wrapperspb.String("sha256:" + hex.EncodeToString(sha)),
				WorkflowRunBackendId:    "792",
				WorkflowJobRunBackendId: "193",
			})).
				AddTokenAuth(token)
			resp = MakeRequest(t, req, http.StatusOK)
			var finalizeResp actions.FinalizeArtifactResponse
			protojson.Unmarshal(resp.Body.Bytes(), &finalizeResp)
			assert.True(t, finalizeResp.Ok)

			artifact := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionArtifact{ID: finalizeResp.ArtifactId})
			if entry.contentType != "" {
				assert.Equal(t, entry.contentType, artifact.ContentEncodingOrType)
			} else {
				assert.Equal(t, "application/zip", artifact.ContentEncodingOrType)
			}
			if entry.path != "" {
				assert.Equal(t, entry.path, artifact.ArtifactPath)
			}
			assert.Equal(t, actions_model.ArtifactStatusUploadConfirmed, artifact.Status)
			assert.Equal(t, int64(entry.append+1)*1024, artifact.FileSize)
			assert.Equal(t, int64(entry.append+1)*1024, artifact.FileCompressedSize)
		})
	}
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
	url := uploadResp.SignedUploadUrl + "&comp=block"

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
	url := uploadResp.SignedUploadUrl + "&comp=block"

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
	url := uploadResp.SignedUploadUrl + "&comp=block&blockid=%2f..%2fmyfile"
	blockListURL := uploadResp.SignedUploadUrl + "&comp=blocklist"

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

	table := []struct {
		name         string
		artifactName string
		serveDirect  bool
		contentType  string
	}{
		{name: "Upload-Zip", artifactName: "artifact-v4-upload", contentType: ""},
		{name: "Upload-Pdf", artifactName: "report-upload.pdf", contentType: "application/pdf"},
		{name: "Upload-Html", artifactName: "report-upload.html", contentType: "application/html"},
		{name: "ServeDirect-Zip", artifactName: "artifact-v4-upload-serve-direct", contentType: "", serveDirect: true},
		{name: "ServeDirect-Pdf", artifactName: "report-upload-serve-direct.pdf", contentType: "application/pdf", serveDirect: true},
		{name: "ServeDirect-Html", artifactName: "report-upload-serve-direct.html", contentType: "application/html", serveDirect: true},
	}

	for _, entry := range table {
		t.Run(entry.name, func(t *testing.T) {
			// Only AzureBlobStorageType supports ServeDirect Uploads
			switch setting.Actions.ArtifactStorage.Type {
			case setting.AzureBlobStorageType:
				defer test.MockVariableValue(&setting.Actions.ArtifactStorage.AzureBlobConfig.ServeDirect, entry.serveDirect)()
			default:
				if entry.serveDirect {
					t.Skip()
				}
			}
			// acquire artifact upload url
			req := NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/CreateArtifact", toProtoJSON(&actions.CreateArtifactRequest{
				Version:                 util.Iif[int32](entry.contentType != "", 7, 4),
				Name:                    entry.artifactName,
				WorkflowRunBackendId:    "792",
				WorkflowJobRunBackendId: "193",
				MimeType:                util.Iif(entry.contentType != "", wrapperspb.String(entry.contentType), nil),
			})).AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			var uploadResp actions.CreateArtifactResponse
			protojson.Unmarshal(resp.Body.Bytes(), &uploadResp)
			assert.True(t, uploadResp.Ok)
			if !entry.serveDirect {
				assert.Contains(t, uploadResp.SignedUploadUrl, "/twirp/github.actions.results.api.v1.ArtifactService/UploadArtifact")
			}

			// get upload urls
			block1URL := uploadResp.SignedUploadUrl + "&comp=block&blockid=" + base64.RawURLEncoding.EncodeToString([]byte("block1"))
			block2URL := uploadResp.SignedUploadUrl + "&comp=block&blockid=" + base64.RawURLEncoding.EncodeToString([]byte("block2"))
			blockListURL := uploadResp.SignedUploadUrl + "&comp=blocklist"

			// upload artifact chunks
			bodyb := strings.Repeat("B", 1024)
			req = NewRequestWithBody(t, "PUT", block2URL, strings.NewReader(bodyb))
			if entry.serveDirect {
				req.Request.RequestURI = ""
				nresp, err := http.DefaultClient.Do(req.Request)
				require.NoError(t, err)
				nresp.Body.Close()
				require.Equal(t, http.StatusCreated, nresp.StatusCode)
			} else {
				MakeRequest(t, req, http.StatusCreated)
			}

			bodya := strings.Repeat("A", 1024)
			req = NewRequestWithBody(t, "PUT", block1URL, strings.NewReader(bodya))
			if entry.serveDirect {
				req.Request.RequestURI = ""
				nresp, err := http.DefaultClient.Do(req.Request)
				require.NoError(t, err)
				nresp.Body.Close()
				require.Equal(t, http.StatusCreated, nresp.StatusCode)
			} else {
				MakeRequest(t, req, http.StatusCreated)
			}

			// upload artifact blockList
			blockList := &actions.BlockList{
				Latest: []string{
					base64.RawURLEncoding.EncodeToString([]byte("block1")),
					base64.RawURLEncoding.EncodeToString([]byte("block2")),
				},
			}
			rawBlockList, err := xml.Marshal(blockList)
			assert.NoError(t, err)
			req = NewRequestWithBody(t, "PUT", blockListURL, bytes.NewReader(rawBlockList))
			if entry.serveDirect {
				req.Request.RequestURI = ""
				nresp, err := http.DefaultClient.Do(req.Request)
				require.NoError(t, err)
				nresp.Body.Close()
				require.Equal(t, http.StatusCreated, nresp.StatusCode)
			} else {
				MakeRequest(t, req, http.StatusCreated)
			}

			t.Logf("Create artifact confirm")

			sha := sha256.Sum256([]byte(bodya + bodyb))

			// confirm artifact upload
			req = NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/FinalizeArtifact", toProtoJSON(&actions.FinalizeArtifactRequest{
				Name:                    entry.artifactName,
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

			artifact := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionArtifact{ID: finalizeResp.ArtifactId})
			if entry.contentType != "" {
				assert.Equal(t, entry.contentType, artifact.ContentEncodingOrType)
			} else {
				assert.Equal(t, "application/zip", artifact.ContentEncodingOrType)
			}
			assert.Equal(t, actions_model.ArtifactStatusUploadConfirmed, artifact.Status)
			assert.Equal(t, int64(2048), artifact.FileSize)
			assert.Equal(t, int64(2048), artifact.FileCompressedSize)
		})
	}
}

func TestActionsArtifactV4DownloadSingle(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	token, err := actions_service.CreateAuthorizationToken(48, 792, 193)
	assert.NoError(t, err)

	table := []struct {
		Name               string
		ArtifactName       string
		FileName           string
		ServeDirect        bool
		ContentType        string
		ContentDisposition string
	}{
		{Name: "Download-Zip", ArtifactName: "artifact-v4-download", FileName: "artifact-v4-download.zip", ContentType: "application/zip"},
		{Name: "Download-Pdf", ArtifactName: "report.pdf", FileName: "report.pdf", ContentType: "application/pdf"},
		{Name: "Download-Html", ArtifactName: "report.html", FileName: "report.html", ContentType: "application/html"},
		{Name: "ServeDirect-Zip", ArtifactName: "artifact-v4-download", FileName: "artifact-v4-download.zip", ContentType: "application/zip", ServeDirect: true},
		{Name: "ServeDirect-Pdf", ArtifactName: "report.pdf", FileName: "report.pdf", ContentType: "application/pdf", ServeDirect: true},
		{Name: "ServeDirect-Html", ArtifactName: "report.html", FileName: "report.html", ContentType: "application/html", ServeDirect: true},
	}

	for _, entry := range table {
		t.Run(entry.Name, func(t *testing.T) {
			switch setting.Actions.ArtifactStorage.Type {
			case setting.AzureBlobStorageType:
				defer test.MockVariableValue(&setting.Actions.ArtifactStorage.AzureBlobConfig.ServeDirect, entry.ServeDirect)()
			case setting.MinioStorageType:
				defer test.MockVariableValue(&setting.Actions.ArtifactStorage.MinioConfig.ServeDirect, entry.ServeDirect)()
			default:
				if entry.ServeDirect {
					t.Skip()
				}
			}

			// list artifacts by name
			req := NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/ListArtifacts", toProtoJSON(&actions.ListArtifactsRequest{
				NameFilter:              wrapperspb.String(entry.ArtifactName),
				WorkflowRunBackendId:    "792",
				WorkflowJobRunBackendId: "193",
			})).AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			var listResp actions.ListArtifactsResponse
			require.NoError(t, protojson.Unmarshal(resp.Body.Bytes(), &listResp))
			require.Len(t, listResp.Artifacts, 1)

			// list artifacts by id
			req = NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/ListArtifacts", toProtoJSON(&actions.ListArtifactsRequest{
				IdFilter:                wrapperspb.Int64(listResp.Artifacts[0].DatabaseId),
				WorkflowRunBackendId:    "792",
				WorkflowJobRunBackendId: "193",
			})).AddTokenAuth(token)
			resp = MakeRequest(t, req, http.StatusOK)
			require.NoError(t, protojson.Unmarshal(resp.Body.Bytes(), &listResp))
			assert.Len(t, listResp.Artifacts, 1)

			// acquire artifact download url
			req = NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/GetSignedArtifactURL", toProtoJSON(&actions.GetSignedArtifactURLRequest{
				Name:                    entry.ArtifactName,
				WorkflowRunBackendId:    "792",
				WorkflowJobRunBackendId: "193",
			})).
				AddTokenAuth(token)
			resp = MakeRequest(t, req, http.StatusOK)
			var finalizeResp actions.GetSignedArtifactURLResponse
			require.NoError(t, protojson.Unmarshal(resp.Body.Bytes(), &finalizeResp))
			assert.NotEmpty(t, finalizeResp.SignedUrl)

			body := strings.Repeat("D", 1024)
			var contentDisposition string
			if entry.ServeDirect {
				externalReq, err := http.NewRequestWithContext(t.Context(), http.MethodGet, finalizeResp.SignedUrl, nil)
				require.NoError(t, err)
				externalResp, err := http.DefaultClient.Do(externalReq)
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, externalResp.StatusCode)
				assert.Equal(t, entry.ContentType, externalResp.Header.Get("Content-Type"))
				contentDisposition = externalResp.Header.Get("Content-Disposition")
				buf := make([]byte, 1024)
				n, err := io.ReadAtLeast(externalResp.Body, buf, len(buf))
				externalResp.Body.Close()
				require.NoError(t, err)
				assert.Equal(t, len(buf), n)
				assert.Equal(t, body, string(buf))
			} else {
				req = NewRequest(t, "GET", finalizeResp.SignedUrl)
				resp = MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, entry.ContentType, resp.Header().Get("Content-Type"))
				contentDisposition = resp.Header().Get("Content-Disposition")
				assert.Equal(t, body, resp.Body.String())
			}
			disposition, param, err := mime.ParseMediaType(contentDisposition)
			require.NoError(t, err)
			assert.Equal(t, "inline", disposition)
			assert.Equal(t, entry.FileName, param["filename"])
		})
	}
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
		req = NewRequestWithBody(t, "GET", artifact.URL, nil).
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

	// confirm artifact is no longer accessible by GetSignedArtifactURL
	req = NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/GetSignedArtifactURL", toProtoJSON(&actions.GetSignedArtifactURLRequest{
		Name:                    "artifact-v4-download",
		WorkflowRunBackendId:    "792",
		WorkflowJobRunBackendId: "193",
	})).
		AddTokenAuth(token)
	_ = MakeRequest(t, req, http.StatusNotFound)

	// confirm artifact is no longer enumerateable by ListArtifacts and returns length == 0 without error
	req = NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/ListArtifacts", toProtoJSON(&actions.ListArtifactsRequest{
		NameFilter:              wrapperspb.String("artifact-v4-download"),
		WorkflowRunBackendId:    "792",
		WorkflowJobRunBackendId: "193",
	})).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	var listResp actions.ListArtifactsResponse
	protojson.Unmarshal(resp.Body.Bytes(), &listResp)
	assert.Empty(t, listResp.Artifacts)
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

func testActionRunAttemptArtifactV4(t *testing.T, repo *repo_model.Repository, session *TestSession, runner *mockRunner) {
	req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/actions/run?workflow=%s", repo.OwnerName, repo.Name, "run-attempt-artifact.yml"), map[string]string{
		"ref": "refs/heads/main",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	// first run
	task1 := runner.fetchTask(t)
	_, job1, run := getTaskAndJobAndRunByTaskID(t, task1.Id)
	require.NotZero(t, job1.RunAttemptID)
	taskToken1 := task1.Context.GetFields()["gitea_runtime_token"].GetStringValue()
	require.NotEmpty(t, taskToken1)
	uploadTestArtifactFileV4(t, run.ID, job1.ID, taskToken1, "artifact-attempt-1", strings.Repeat("A", 32))
	uploadTestArtifactFileV4(t, run.ID, job1.ID, taskToken1, "artifact-shared", strings.Repeat("C", 32))
	attempt1Names := listArtifactNamesForRunV4(t, run.ID, job1.ID, taskToken1)
	assert.ElementsMatch(t, []string{"artifact-attempt-1", "artifact-shared"}, attempt1Names)

	runner.execTask(t, task1, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})

	// rerun
	req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/rerun", repo.OwnerName, repo.Name, run.ID))
	session.MakeRequest(t, req, http.StatusOK)
	task2 := runner.fetchTask(t)
	_, job2, _ := getTaskAndJobAndRunByTaskID(t, task2.Id)
	require.NotZero(t, job2.RunAttemptID)
	assert.NotEqual(t, job1.RunAttemptID, job2.RunAttemptID)
	taskToken2 := task2.Context.GetFields()["gitea_runtime_token"].GetStringValue()
	require.NotEmpty(t, taskToken2)
	uploadTestArtifactFileV4(t, run.ID, job2.ID, taskToken2, "artifact-attempt-2", strings.Repeat("B", 32))
	uploadTestArtifactFileV4(t, run.ID, job2.ID, taskToken2, "artifact-shared", strings.Repeat("D", 32))
	attempt2Names := listArtifactNamesForRunV4(t, run.ID, job2.ID, taskToken2)
	assert.ElementsMatch(t, []string{"artifact-attempt-2", "artifact-shared"}, attempt2Names)
	assert.NotContains(t, attempt2Names, "artifact-attempt-1")

	// "artifact-attempt-1" belongs to the first attempt, so the rerun token cannot access it
	req = NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/GetSignedArtifactURL", toProtoJSON(&actions.GetSignedArtifactURLRequest{
		Name:                    "artifact-attempt-1",
		WorkflowRunBackendId:    strconv.FormatInt(run.ID, 10),
		WorkflowJobRunBackendId: strconv.FormatInt(job2.ID, 10),
	})).AddTokenAuth(taskToken2)
	MakeRequest(t, req, http.StatusNotFound)

	// the run-scoped repo API should list finalized v4 artifacts from all attempts
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/actions/runs/%d/artifacts", repo.OwnerName, repo.Name, run.ID))
	resp := session.MakeRequest(t, req, http.StatusOK)
	var runArtifactsResp api.ActionArtifactsResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &runArtifactsResp))
	require.Len(t, runArtifactsResp.Entries, 4)
	runArtifactNames := make([]string, 0, len(runArtifactsResp.Entries))
	for _, artifact := range runArtifactsResp.Entries {
		runArtifactNames = append(runArtifactNames, artifact.Name)
	}
	assert.ElementsMatch(t, []string{"artifact-attempt-1", "artifact-shared", "artifact-attempt-2", "artifact-shared"}, runArtifactNames)

	// the result should contain 2 artifacts when query by name=artifact-shared
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/actions/runs/%d/artifacts?name=artifact-shared", repo.OwnerName, repo.Name, run.ID))
	resp = session.MakeRequest(t, req, http.StatusOK)
	var sharedArtifactsResp api.ActionArtifactsResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &sharedArtifactsResp))
	require.Len(t, sharedArtifactsResp.Entries, 2)
	assert.Equal(t, strings.Repeat("C", 32), downloadRepoArtifactV4Content(t, session, sharedArtifactsResp.Entries[0].ArchiveDownloadURL))
	assert.Equal(t, strings.Repeat("D", 32), downloadRepoArtifactV4Content(t, session, sharedArtifactsResp.Entries[1].ArchiveDownloadURL))
}

func uploadTestArtifactFileV4(t *testing.T, runID, jobID int64, authToken, artifactName, content string) {
	t.Helper()

	req := NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/CreateArtifact", toProtoJSON(&actions.CreateArtifactRequest{
		Version:                 4,
		Name:                    artifactName,
		WorkflowRunBackendId:    strconv.FormatInt(runID, 10),
		WorkflowJobRunBackendId: strconv.FormatInt(jobID, 10),
		MimeType:                wrapperspb.String("application/zip"),
	})).AddTokenAuth(authToken)
	resp := MakeRequest(t, req, http.StatusOK)
	var uploadResp actions.CreateArtifactResponse
	require.NoError(t, protojson.Unmarshal(resp.Body.Bytes(), &uploadResp))
	require.True(t, uploadResp.Ok)

	req = NewRequestWithBody(t, "PUT", uploadResp.SignedUploadUrl+"&comp=appendBlock", strings.NewReader(content))
	MakeRequest(t, req, http.StatusCreated)

	sum := sha256.Sum256([]byte(content))
	req = NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/FinalizeArtifact", toProtoJSON(&actions.FinalizeArtifactRequest{
		Name:                    artifactName,
		Size:                    int64(len(content)),
		Hash:                    wrapperspb.String("sha256:" + hex.EncodeToString(sum[:])),
		WorkflowRunBackendId:    strconv.FormatInt(runID, 10),
		WorkflowJobRunBackendId: strconv.FormatInt(jobID, 10),
	})).AddTokenAuth(authToken)
	resp = MakeRequest(t, req, http.StatusOK)
	var finalizeResp actions.FinalizeArtifactResponse
	require.NoError(t, protojson.Unmarshal(resp.Body.Bytes(), &finalizeResp))
	require.True(t, finalizeResp.Ok)
}

func listArtifactNamesForRunV4(t *testing.T, runID, jobID int64, taskToken string) []string {
	t.Helper()

	req := NewRequestWithBody(t, "POST", "/twirp/github.actions.results.api.v1.ArtifactService/ListArtifacts", toProtoJSON(&actions.ListArtifactsRequest{
		WorkflowRunBackendId:    strconv.FormatInt(runID, 10),
		WorkflowJobRunBackendId: strconv.FormatInt(jobID, 10),
	})).AddTokenAuth(taskToken)
	resp := MakeRequest(t, req, http.StatusOK)
	var listResp actions.ListArtifactsResponse
	require.NoError(t, protojson.Unmarshal(resp.Body.Bytes(), &listResp))

	names := make([]string, 0, len(listResp.Artifacts))
	for _, item := range listResp.Artifacts {
		names = append(names, item.Name)
	}
	return names
}

func downloadRepoArtifactV4Content(t *testing.T, session *TestSession, archiveDownloadURL string) string {
	t.Helper()

	req := NewRequest(t, "GET", archiveDownloadURL)
	resp := session.MakeRequest(t, req, http.StatusFound)
	req = NewRequest(t, "GET", resp.Header().Get("Location"))
	resp = MakeRequest(t, req, http.StatusOK)
	return resp.Body.String()
}
