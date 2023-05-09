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

func TestArtifactsUpload(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	type uploadArtifactResponse struct {
		FileContainerResourceUrl string `json:"fileContainerResourceUrl"`
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
	assert.Contains(t, uploadResp.FileContainerResourceUrl, "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts")

	// get upload url
	idx := strings.Index(uploadResp.FileContainerResourceUrl, "/api/actions_pipeline/_apis/pipelines/")
	url := uploadResp.FileContainerResourceUrl[idx:] + "?itemPath=artifact/abc.txt"

	// upload artifact chunk
	body := strings.Repeat("A", 1024)
	req = NewRequestWithBody(t, "PUT", url, strings.NewReader(body))
	req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	req.Header.Add("Content-Range", "bytes 0-1023/1024")
	req.Header.Add("x-tfs-filelength", "1024")
	req.Header.Add("x-actions-results-md5", "1HsSe8LeLWh93ILaw1TEFQ==") // base64(md5(body))
	MakeRequest(t, req, http.StatusOK)

	// confirm artifact upload
	req = NewRequest(t, "PATCH", "/api/actions_pipeline/_apis/pipelines/workflows/791/artifacts")
	req = addTokenAuthHeader(req, "Bearer 8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	MakeRequest(t, req, http.StatusOK)
}
