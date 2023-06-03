// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestActionsArtifactUpload(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	type uploadArtifactResponse struct {
		FileContainerResourceURL string `json:"fileContainerResourceUrl"`
	}

	type getUploadArtifactRequest struct {
		Type string
		Name string
	}

	// acquire artifact upload url
	req := NewRequestWithJSON(t, "POST", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts", getUploadArtifactRequest{
		Type: "actions_storage",
		Name: "artifact",
	})
	req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp := MakeRequest(t, req, http.StatusOK)
	var uploadResp uploadArtifactResponse
	DecodeJSON(t, resp, &uploadResp)
	assert.Contains(t, uploadResp.FileContainerResourceURL, "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts")

	// get upload url
	idx := strings.Index(uploadResp.FileContainerResourceURL, "/api/actions_pipeline/_apis/pipelines/")
	url := uploadResp.FileContainerResourceURL[idx:] + "?itemPath=artifact/abc.txt"

	// upload artifact chunk
	body := strings.Repeat("A", 1024)
	req = NewRequestWithBody(t, "PUT", url, strings.NewReader(body))
	req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	req.Header.Add("Content-Range", "bytes 0-1023/1024")
	req.Header.Add("x-tfs-filelength", "1024")
	req.Header.Add("x-actions-results-md5", "1HsSe8LeLWh93ILaw1TEFQ==") // base64(md5(body))
	MakeRequest(t, req, http.StatusOK)

	t.Logf("Create artifact confirm")

	// confirm artifact upload
	req = NewRequest(t, "PATCH", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts")
	req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	MakeRequest(t, req, http.StatusOK)
}

func TestActionsArtifactUploadNotExist(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// artifact id 54321 not exist
	url := "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts/54321/upload?itemPath=artifact/abc.txt"
	body := strings.Repeat("A", 1024)
	req := NewRequestWithBody(t, "PUT", url, strings.NewReader(body))
	req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	req.Header.Add("Content-Range", "bytes 0-1023/1024")
	req.Header.Add("x-tfs-filelength", "1024")
	req.Header.Add("x-actions-results-md5", "1HsSe8LeLWh93ILaw1TEFQ==") // base64(md5(body))
	MakeRequest(t, req, http.StatusNotFound)
}

func TestActionsArtifactConfirmUpload(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "PATCH", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts")
	req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp := MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "success")
}

func TestActionsArtifactUploadWithoutToken(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequestWithJSON(t, "POST", "/api/actions_pipeline/_apis/pipelines/workflows/1/artifacts", nil)
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestActionsArtifactDownload(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	type (
		listArtifactsResponseItem struct {
			Name                     string `json:"name"`
			FileContainerResourceURL string `json:"fileContainerResourceUrl"`
		}
		listArtifactsResponse struct {
			Count int64                       `json:"count"`
			Value []listArtifactsResponseItem `json:"value"`
		}
	)

	req := NewRequest(t, "GET", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts")
	req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp := MakeRequest(t, req, http.StatusOK)
	var listResp listArtifactsResponse
	DecodeJSON(t, resp, &listResp)
	assert.Equal(t, int64(1), listResp.Count)
	assert.Equal(t, "artifact", listResp.Value[0].Name)
	assert.Contains(t, listResp.Value[0].FileContainerResourceURL, "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts")

	type (
		downloadArtifactResponseItem struct {
			Path            string `json:"path"`
			ItemType        string `json:"itemType"`
			ContentLocation string `json:"contentLocation"`
		}
		downloadArtifactResponse struct {
			Value []downloadArtifactResponseItem `json:"value"`
		}
	)

	idx := strings.Index(listResp.Value[0].FileContainerResourceURL, "/api/actions_pipeline/_apis/pipelines/")
	url := listResp.Value[0].FileContainerResourceURL[idx+1:]
	req = NewRequest(t, "GET", url)
	req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp = MakeRequest(t, req, http.StatusOK)
	var downloadResp downloadArtifactResponse
	DecodeJSON(t, resp, &downloadResp)
	assert.Len(t, downloadResp.Value, 1)
	assert.Equal(t, "artifact/abc.txt", downloadResp.Value[0].Path)
	assert.Equal(t, "file", downloadResp.Value[0].ItemType)
	assert.Contains(t, downloadResp.Value[0].ContentLocation, "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts")

	idx = strings.Index(downloadResp.Value[0].ContentLocation, "/api/actions_pipeline/_apis/pipelines/")
	url = downloadResp.Value[0].ContentLocation[idx:]
	req = NewRequest(t, "GET", url)
	req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp = MakeRequest(t, req, http.StatusOK)
	body := strings.Repeat("A", 1024)
	assert.Equal(t, resp.Body.String(), body)
}
