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

func prepareTestEnvActionsArtifacts(t *testing.T) func() {
	t.Helper()
	f := tests.PrepareTestEnv(t, 1)
	tests.PrepareArtifactsStorage(t)
	return f
}

func TestActionsArtifactUploadSingleFile(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	// acquire artifact upload url
	req := NewRequestWithJSON(t, "POST", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts", getUploadArtifactRequest{
		Type: "actions_storage",
		Name: "artifact",
	}).AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp := MakeRequest(t, req, http.StatusOK)
	var uploadResp uploadArtifactResponse
	DecodeJSON(t, resp, &uploadResp)
	assert.Contains(t, uploadResp.FileContainerResourceURL, "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts")

	// get upload url
	idx := strings.Index(uploadResp.FileContainerResourceURL, "/api/actions_pipeline/_apis/pipelines/")
	url := uploadResp.FileContainerResourceURL[idx:] + "?itemPath=artifact/abc-2.txt"

	// upload artifact chunk
	body := strings.Repeat("C", 1024)
	req = NewRequestWithBody(t, "PUT", url, strings.NewReader(body)).
		AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a").
		SetHeader("Content-Range", "bytes 0-1023/1024").
		SetHeader("x-tfs-filelength", "1024").
		SetHeader("x-actions-results-md5", "XVlf820rMInUi64wmMi6EA==") // base64(md5(body))
	MakeRequest(t, req, http.StatusOK)

	t.Logf("Create artifact confirm")

	// confirm artifact upload
	req = NewRequest(t, "PATCH", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts?artifactName=artifact-single").
		AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	MakeRequest(t, req, http.StatusOK)
}

func TestActionsArtifactUploadInvalidHash(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	// artifact id 54321 not exist
	url := "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts/8e5b948a454515dbabfc7eb718ddddddd/upload?itemPath=artifact/abc.txt"
	body := strings.Repeat("A", 1024)
	req := NewRequestWithBody(t, "PUT", url, strings.NewReader(body)).
		AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a").
		SetHeader("Content-Range", "bytes 0-1023/1024").
		SetHeader("x-tfs-filelength", "1024").
		SetHeader("x-actions-results-md5", "1HsSe8LeLWh93ILaw1TEFQ==") // base64(md5(body))
	resp := MakeRequest(t, req, http.StatusBadRequest)
	assert.Contains(t, resp.Body.String(), "Invalid artifact hash")
}

func TestActionsArtifactConfirmUploadWithoutName(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	req := NewRequest(t, "PATCH", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts").
		AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp := MakeRequest(t, req, http.StatusBadRequest)
	assert.Contains(t, resp.Body.String(), "artifact name is empty")
}

func TestActionsArtifactUploadWithoutToken(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

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
	defer prepareTestEnvActionsArtifacts(t)()

	req := NewRequest(t, "GET", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts").
		AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp := MakeRequest(t, req, http.StatusOK)
	var listResp listArtifactsResponse
	DecodeJSON(t, resp, &listResp)
	assert.Equal(t, int64(2), listResp.Count)

	// Return list might be in any order. Get one file.
	var artifactIdx int
	for i, artifact := range listResp.Value {
		if artifact.Name == "artifact-download" {
			artifactIdx = i
			break
		}
	}
	assert.NotNil(t, artifactIdx)
	assert.Equal(t, "artifact-download", listResp.Value[artifactIdx].Name)
	assert.Contains(t, listResp.Value[artifactIdx].FileContainerResourceURL, "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts")

	idx := strings.Index(listResp.Value[artifactIdx].FileContainerResourceURL, "/api/actions_pipeline/_apis/pipelines/")
	url := listResp.Value[artifactIdx].FileContainerResourceURL[idx+1:] + "?itemPath=artifact-download"
	req = NewRequest(t, "GET", url).
		AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp = MakeRequest(t, req, http.StatusOK)
	var downloadResp downloadArtifactResponse
	DecodeJSON(t, resp, &downloadResp)
	assert.Len(t, downloadResp.Value, 1)
	assert.Equal(t, "artifact-download/abc.txt", downloadResp.Value[0].Path)
	assert.Equal(t, "file", downloadResp.Value[0].ItemType)
	assert.Contains(t, downloadResp.Value[0].ContentLocation, "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts")

	idx = strings.Index(downloadResp.Value[0].ContentLocation, "/api/actions_pipeline/_apis/pipelines/")
	url = downloadResp.Value[0].ContentLocation[idx:]
	req = NewRequest(t, "GET", url).
		AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp = MakeRequest(t, req, http.StatusOK)

	body := strings.Repeat("A", 1024)
	assert.Equal(t, body, resp.Body.String())
}

func TestActionsArtifactUploadMultipleFile(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	const testArtifactName = "multi-files"

	// acquire artifact upload url
	req := NewRequestWithJSON(t, "POST", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts", getUploadArtifactRequest{
		Type: "actions_storage",
		Name: testArtifactName,
	}).AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
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
			Path:    "abc-3.txt",
			Content: strings.Repeat("D", 1024),
			MD5:     "9nqj7E8HZmfQtPifCJ5Zww==",
		},
		{
			Path:    "xyz/def-2.txt",
			Content: strings.Repeat("E", 1024),
			MD5:     "/s1kKvxeHlUX85vaTaVxuA==",
		},
	}

	for _, f := range files {
		// get upload url
		idx := strings.Index(uploadResp.FileContainerResourceURL, "/api/actions_pipeline/_apis/pipelines/")
		url := uploadResp.FileContainerResourceURL[idx:] + "?itemPath=" + testArtifactName + "/" + f.Path

		// upload artifact chunk
		req = NewRequestWithBody(t, "PUT", url, strings.NewReader(f.Content)).
			AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a").
			SetHeader("Content-Range", "bytes 0-1023/1024").
			SetHeader("x-tfs-filelength", "1024").
			SetHeader("x-actions-results-md5", f.MD5) // base64(md5(body))
		MakeRequest(t, req, http.StatusOK)
	}

	t.Logf("Create artifact confirm")

	// confirm artifact upload
	req = NewRequest(t, "PATCH", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts?artifactName="+testArtifactName).
		AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	MakeRequest(t, req, http.StatusOK)
}

func TestActionsArtifactDownloadMultiFiles(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	const testArtifactName = "multi-file-download"

	req := NewRequest(t, "GET", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts").
		AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
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
	req = NewRequest(t, "GET", url).
		AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp = MakeRequest(t, req, http.StatusOK)
	var downloadResp downloadArtifactResponse
	DecodeJSON(t, resp, &downloadResp)
	assert.Len(t, downloadResp.Value, 2)

	downloads := [][]string{{"multi-file-download/abc.txt", "B"}, {"multi-file-download/xyz/def.txt", "C"}}
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
		req = NewRequest(t, "GET", url).
			AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
		resp = MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, strings.Repeat(bodyChar, 1024), resp.Body.String())
	}
}

func TestActionsArtifactUploadWithRetentionDays(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	// acquire artifact upload url
	req := NewRequestWithJSON(t, "POST", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts", getUploadArtifactRequest{
		Type:          "actions_storage",
		Name:          "artifact-retention-days",
		RetentionDays: 9,
	}).AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
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
	req = NewRequestWithBody(t, "PUT", url, strings.NewReader(body)).
		AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a").
		SetHeader("Content-Range", "bytes 0-1023/1024").
		SetHeader("x-tfs-filelength", "1024").
		SetHeader("x-actions-results-md5", "1HsSe8LeLWh93ILaw1TEFQ==") // base64(md5(body))
	MakeRequest(t, req, http.StatusOK)

	t.Logf("Create artifact confirm")

	// confirm artifact upload
	req = NewRequest(t, "PATCH", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts?artifactName=artifact-retention-days").
		AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	MakeRequest(t, req, http.StatusOK)
}

func TestActionsArtifactOverwrite(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	{
		// download old artifact uploaded by tests above, it should 1024 A
		req := NewRequest(t, "GET", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts").
			AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
		resp := MakeRequest(t, req, http.StatusOK)
		var listResp listArtifactsResponse
		DecodeJSON(t, resp, &listResp)

		idx := strings.Index(listResp.Value[0].FileContainerResourceURL, "/api/actions_pipeline/_apis/pipelines/")
		url := listResp.Value[0].FileContainerResourceURL[idx+1:] + "?itemPath=artifact-download"
		req = NewRequest(t, "GET", url).
			AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
		resp = MakeRequest(t, req, http.StatusOK)
		var downloadResp downloadArtifactResponse
		DecodeJSON(t, resp, &downloadResp)

		idx = strings.Index(downloadResp.Value[0].ContentLocation, "/api/actions_pipeline/_apis/pipelines/")
		url = downloadResp.Value[0].ContentLocation[idx:]
		req = NewRequest(t, "GET", url).
			AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
		resp = MakeRequest(t, req, http.StatusOK)
		body := strings.Repeat("A", 1024)
		assert.Equal(t, resp.Body.String(), body)
	}

	{
		// upload same artifact, it uses 4096 B
		req := NewRequestWithJSON(t, "POST", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts", getUploadArtifactRequest{
			Type: "actions_storage",
			Name: "artifact-download",
		}).AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
		resp := MakeRequest(t, req, http.StatusOK)
		var uploadResp uploadArtifactResponse
		DecodeJSON(t, resp, &uploadResp)

		idx := strings.Index(uploadResp.FileContainerResourceURL, "/api/actions_pipeline/_apis/pipelines/")
		url := uploadResp.FileContainerResourceURL[idx:] + "?itemPath=artifact-download/abc.txt"
		body := strings.Repeat("B", 4096)
		req = NewRequestWithBody(t, "PUT", url, strings.NewReader(body)).
			AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a").
			SetHeader("Content-Range", "bytes 0-4095/4096").
			SetHeader("x-tfs-filelength", "4096").
			SetHeader("x-actions-results-md5", "wUypcJFeZCK5T6r4lfqzqg==") // base64(md5(body))
		MakeRequest(t, req, http.StatusOK)

		// confirm artifact upload
		req = NewRequest(t, "PATCH", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts?artifactName=artifact-download").
			AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
		MakeRequest(t, req, http.StatusOK)
	}

	{
		// download artifact again, it should 4096 B
		req := NewRequest(t, "GET", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts").
			AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
		resp := MakeRequest(t, req, http.StatusOK)
		var listResp listArtifactsResponse
		DecodeJSON(t, resp, &listResp)

		var uploadedItem listArtifactsResponseItem
		for _, item := range listResp.Value {
			if item.Name == "artifact-download" {
				uploadedItem = item
				break
			}
		}
		assert.Equal(t, "artifact-download", uploadedItem.Name)

		idx := strings.Index(uploadedItem.FileContainerResourceURL, "/api/actions_pipeline/_apis/pipelines/")
		url := uploadedItem.FileContainerResourceURL[idx+1:] + "?itemPath=artifact-download"
		req = NewRequest(t, "GET", url).
			AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
		resp = MakeRequest(t, req, http.StatusOK)
		var downloadResp downloadArtifactResponse
		DecodeJSON(t, resp, &downloadResp)

		idx = strings.Index(downloadResp.Value[0].ContentLocation, "/api/actions_pipeline/_apis/pipelines/")
		url = downloadResp.Value[0].ContentLocation[idx:]
		req = NewRequest(t, "GET", url).
			AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
		resp = MakeRequest(t, req, http.StatusOK)
		body := strings.Repeat("B", 4096)
		assert.Equal(t, resp.Body.String(), body)
	}
}
