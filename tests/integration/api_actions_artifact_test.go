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

type uploadArtifactResponse struct {
	FileContainerResourceURL string `json:"fileContainerResourceUrl"`
}

type getUploadArtifactRequest struct {
	Type          string
	Name          string
	RetentionDays int64
}

func TestActionsArtifactUploadSingleFile(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

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
	req = NewRequest(t, "PATCH", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts?artifactName=artifact")
	req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	MakeRequest(t, req, http.StatusOK)
}

func TestActionsArtifactUploadInvalidHash(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// artifact id 54321 not exist
	url := "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts/8e5b948a454515dbabfc7eb718ddddddd/upload?itemPath=artifact/abc.txt"
	body := strings.Repeat("A", 1024)
	req := NewRequestWithBody(t, "PUT", url, strings.NewReader(body))
	req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	req.Header.Add("Content-Range", "bytes 0-1023/1024")
	req.Header.Add("x-tfs-filelength", "1024")
	req.Header.Add("x-actions-results-md5", "1HsSe8LeLWh93ILaw1TEFQ==") // base64(md5(body))
	resp := MakeRequest(t, req, http.StatusBadRequest)
	assert.Contains(t, resp.Body.String(), "Invalid artifact hash")
}

func TestActionsArtifactConfirmUploadWithoutName(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "PATCH", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts")
	req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp := MakeRequest(t, req, http.StatusBadRequest)
	assert.Contains(t, resp.Body.String(), "artifact name is empty")
}

func TestActionsArtifactUploadWithoutToken(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequestWithJSON(t, "POST", "/api/actions_pipeline/_apis/pipelines/workflows/1/artifacts", nil)
	MakeRequest(t, req, http.StatusUnauthorized)
}

type (
	listArtifactsResponseItem struct {
		Name                     string `json:"name"`
		FileContainerResourceURL string `json:"fileContainerResourceUrl"`
	}
	listArtifactsResponse struct {
		Count int64                       `json:"count"`
		Value []listArtifactsResponseItem `json:"value"`
	}
	downloadArtifactResponseItem struct {
		Path            string `json:"path"`
		ItemType        string `json:"itemType"`
		ContentLocation string `json:"contentLocation"`
	}
	downloadArtifactResponse struct {
		Value []downloadArtifactResponseItem `json:"value"`
	}
)

func TestActionsArtifactDownload(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts")
	req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp := MakeRequest(t, req, http.StatusOK)
	var listResp listArtifactsResponse
	DecodeJSON(t, resp, &listResp)
	assert.Equal(t, int64(1), listResp.Count)
	assert.Equal(t, "artifact", listResp.Value[0].Name)
	assert.Contains(t, listResp.Value[0].FileContainerResourceURL, "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts")

	idx := strings.Index(listResp.Value[0].FileContainerResourceURL, "/api/actions_pipeline/_apis/pipelines/")
	url := listResp.Value[0].FileContainerResourceURL[idx+1:] + "?itemPath=artifact"
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

func TestActionsArtifactUploadMultipleFile(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	const testArtifactName = "multi-files"

	// acquire artifact upload url
	req := NewRequestWithJSON(t, "POST", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts", getUploadArtifactRequest{
		Type: "actions_storage",
		Name: testArtifactName,
	})
	req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp := MakeRequest(t, req, http.StatusOK)
	var uploadResp uploadArtifactResponse
	DecodeJSON(t, resp, &uploadResp)
	assert.Contains(t, uploadResp.FileContainerResourceURL, "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts")

	type uploadingFile struct {
		Path    string
		Content string
		MD5     string
	}

	files := []uploadingFile{
		{
			Path:    "abc.txt",
			Content: strings.Repeat("A", 1024),
			MD5:     "1HsSe8LeLWh93ILaw1TEFQ==",
		},
		{
			Path:    "xyz/def.txt",
			Content: strings.Repeat("B", 1024),
			MD5:     "6fgADK/7zjadf+6cB9Q1CQ==",
		},
	}

	for _, f := range files {
		// get upload url
		idx := strings.Index(uploadResp.FileContainerResourceURL, "/api/actions_pipeline/_apis/pipelines/")
		url := uploadResp.FileContainerResourceURL[idx:] + "?itemPath=" + testArtifactName + "/" + f.Path

		// upload artifact chunk
		req = NewRequestWithBody(t, "PUT", url, strings.NewReader(f.Content))
		req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
		req.Header.Add("Content-Range", "bytes 0-1023/1024")
		req.Header.Add("x-tfs-filelength", "1024")
		req.Header.Add("x-actions-results-md5", f.MD5) // base64(md5(body))
		MakeRequest(t, req, http.StatusOK)
	}

	t.Logf("Create artifact confirm")

	// confirm artifact upload
	req = NewRequest(t, "PATCH", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts?artifactName="+testArtifactName)
	req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	MakeRequest(t, req, http.StatusOK)
}

func TestActionsArtifactDownloadMultiFiles(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	const testArtifactName = "multi-files"

	req := NewRequest(t, "GET", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts")
	req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp := MakeRequest(t, req, http.StatusOK)
	var listResp listArtifactsResponse
	DecodeJSON(t, resp, &listResp)
	assert.Equal(t, int64(2), listResp.Count)

	var fileContainerResourceURL string
	for _, v := range listResp.Value {
		if v.Name == testArtifactName {
			fileContainerResourceURL = v.FileContainerResourceURL
			break
		}
	}
	assert.Contains(t, fileContainerResourceURL, "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts")

	idx := strings.Index(fileContainerResourceURL, "/api/actions_pipeline/_apis/pipelines/")
	url := fileContainerResourceURL[idx+1:] + "?itemPath=" + testArtifactName
	req = NewRequest(t, "GET", url)
	req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp = MakeRequest(t, req, http.StatusOK)
	var downloadResp downloadArtifactResponse
	DecodeJSON(t, resp, &downloadResp)
	assert.Len(t, downloadResp.Value, 2)

	downloads := [][]string{{"multi-files/abc.txt", "A"}, {"multi-files/xyz/def.txt", "B"}}
	for _, v := range downloadResp.Value {
		var bodyChar string
		var path string
		for _, d := range downloads {
			if v.Path == d[0] {
				path = d[0]
				bodyChar = d[1]
				break
			}
		}
		value := v
		assert.Equal(t, path, value.Path)
		assert.Equal(t, "file", value.ItemType)
		assert.Contains(t, value.ContentLocation, "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts")

		idx = strings.Index(value.ContentLocation, "/api/actions_pipeline/_apis/pipelines/")
		url = value.ContentLocation[idx:]
		req = NewRequest(t, "GET", url)
		req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
		resp = MakeRequest(t, req, http.StatusOK)
		body := strings.Repeat(bodyChar, 1024)
		assert.Equal(t, resp.Body.String(), body)
	}
}

func TestActionsArtifactUploadWithRetentionDays(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// acquire artifact upload url
	req := NewRequestWithJSON(t, "POST", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts", getUploadArtifactRequest{
		Type:          "actions_storage",
		Name:          "artifact-retention-days",
		RetentionDays: 9,
	})
	req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp := MakeRequest(t, req, http.StatusOK)
	var uploadResp uploadArtifactResponse
	DecodeJSON(t, resp, &uploadResp)
	assert.Contains(t, uploadResp.FileContainerResourceURL, "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts")
	assert.Contains(t, uploadResp.FileContainerResourceURL, "?retentionDays=9")

	// get upload url
	idx := strings.Index(uploadResp.FileContainerResourceURL, "/api/actions_pipeline/_apis/pipelines/")
	url := uploadResp.FileContainerResourceURL[idx:] + "&itemPath=artifact-retention-days/abc.txt"

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
	req = NewRequest(t, "PATCH", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts?artifactName=artifact-retention-days")
	req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	MakeRequest(t, req, http.StatusOK)
}
