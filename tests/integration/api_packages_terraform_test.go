// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/tests"

	gouuid "github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageTerraform(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// Get token for the user
	token := "Bearer " + getUserToken(t, user.Name, auth.AccessTokenScopeWritePackage)

	// Define important values
	lineage := "bca3c5f6-01dc-cdad-5310-d1b12e02e430"
	terraformVersion := "1.10.4"
	serial := float64(1)
	resourceName := "hello"
	resourceType := "null_resource"
	id := gouuid.New().String() // Generate a unique ID

	// Build the state JSON
	buildState := func() string {
		return `{
			"version": 4,
			"terraform_version": "` + terraformVersion + `",
			"serial": ` + fmt.Sprintf("%.0f", serial) + `,
			"lineage": "` + lineage + `",
			"outputs": {},
			"resources": [{
				"mode": "managed",
				"type": "` + resourceType + `",
				"name": "` + resourceName + `",
				"provider": "provider[\"registry.terraform.io/hashicorp/null\"]",
				"instances": [{
					"schema_version": 0,
					"attributes": {
						"id": "3832416504545530133",
						"triggers": null
					},
					"sensitive_attributes": []
				}]
			}],
			"check_results": null
		}`
	}
	state := buildState()
	content := []byte(state)
	root := fmt.Sprintf("/api/packages/%s/terraform/state", user.Name)
	stateURL := fmt.Sprintf("%s/providers-gitea.tfstate", root)

	// Upload test
	t.Run("Upload", func(t *testing.T) {
		uploadURL := fmt.Sprintf("%s?ID=%s", stateURL, id)
		req := NewRequestWithBody(t, "POST", uploadURL, bytes.NewReader(content)).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK) // Expecting 200 OK
		assert.Equal(t, http.StatusOK, resp.Code)
		assert.Contains(t, resp.Header().Get("Content-Type"), "application/json")
		bodyBytes, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NotEmpty(t, bodyBytes)
	})

	// Download test
	t.Run("Download", func(t *testing.T) {
		downloadURL := fmt.Sprintf("%s?ID=%s", stateURL, id)
		req := NewRequest(t, "GET", downloadURL)
		resp := MakeRequest(t, req, http.StatusOK)
		assert.True(t, strings.HasPrefix(resp.Header().Get("Content-Type"), "application/json"))

		bodyBytes, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NotEmpty(t, bodyBytes)

		var jsonResponse map[string]any
		err = json.Unmarshal(bodyBytes, &jsonResponse)
		require.NoError(t, err)

		// Validate the response
		assert.Equal(t, lineage, jsonResponse["lineage"])
		assert.Equal(t, terraformVersion, jsonResponse["terraform_version"])
		assert.InEpsilon(t, serial, jsonResponse["serial"].(float64), 0.0001)
		resource := jsonResponse["resources"].([]any)[0].(map[string]any)
		assert.Equal(t, resourceName, resource["name"])
		assert.Equal(t, resourceType, resource["type"])
		assert.NotContains(t, resource, "sensitive_attributes")
	})

	// Lock state test
	t.Run("LockState", func(t *testing.T) {
		lockURL := fmt.Sprintf("%s/lock?ID=%s", stateURL, id)
		req := NewRequestWithBody(t, "POST", lockURL, bytes.NewReader(content)).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK) // Expecting 200 OK
		assert.Equal(t, http.StatusOK, resp.Code)
	})

	// Unlock state test
	t.Run("UnlockState", func(t *testing.T) {
		unlockURL := fmt.Sprintf("%s/lock?ID=%s", stateURL, id)
		req := NewRequestWithBody(t, "DELETE", unlockURL, bytes.NewReader(content)).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK) // Expecting 200 OK
		assert.Equal(t, http.StatusOK, resp.Code)
	})

	// Download not found test
	t.Run("DownloadNotFound", func(t *testing.T) {
		invalidStateURL := fmt.Sprintf("%s/invalid-state.tfstate?ID=%s", root, id)
		req := NewRequest(t, "GET", invalidStateURL)
		resp := MakeRequest(t, req, http.StatusNoContent) // Expecting 204 No Content
		assert.Equal(t, http.StatusNoContent, resp.Code)
	})
}
