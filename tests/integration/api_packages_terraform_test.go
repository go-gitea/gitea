// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageTerraform(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	packageName := "te-st_pac.kage"
	lineage := "bca3c5f6-01dc-cdad-5310-d1b12e02e430"
	terraformVersion := "1.10.4"
	resourceName := "hello"
	resourceType := "null_resource"

	// Build the state JSON
	state := `{
			"version": 4,
			"terraform_version": "` + terraformVersion + `",
			"serial": 1,
			"lineage": "` + lineage + `",
			"outPOSTs": {},
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
	content := []byte(state)

	url := fmt.Sprintf("/api/packages/%s/terraform/state/%s", user.Name, packageName)

	t.Run("Upload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequestWithBody(t, "POST", url, bytes.NewReader(content)).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)

		pvs, err := packages.GetVersionsByPackageType(t.Context(), user.ID, packages.TypeTerraform)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(t.Context(), pvs[0])
		assert.NoError(t, err)
		assert.Nil(t, pd.Metadata)
		assert.Equal(t, packageName, pd.Package.Name)
		// assert.Equal(t, filename, pd.Version.Version)

		pfs, err := packages.GetFilesByVersionID(t.Context(), pvs[0].ID)
		assert.NoError(t, err)
		assert.Len(t, pfs, 1)
		assert.Equal(t, "tfstate", pfs[0].Name)
		assert.True(t, pfs[0].IsLead)

		pb, err := packages.GetBlobByID(t.Context(), pfs[0].BlobID)
		assert.NoError(t, err)
		assert.Equal(t, int64(len(content)), pb.Size)

		t.Run("Exists", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequestWithBody(t, "POST", url, bytes.NewReader(content)).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusCreated)
		})
		// TODO: Do we want multiple states in one package?
		//t.Run("Additional", func(t *testing.T) {
		//	defer tests.PrintCurrentTest(t)()
		//
		//	req := NewRequestWithBody(t, "POST", url+"/dummy.bin", bytes.NewReader(content)).
		//		AddBasicAuth(user.Name)
		//	MakeRequest(t, req, http.StatusCreated)
		//
		//	// Check deduplication
		//	pfs, err := packages.GetFilesByVersionID(t.Context(), pvs[0].ID)
		//	assert.NoError(t, err)
		//	assert.Len(t, pfs, 1)
		//	//			assert.Equal(t, pfs[0].BlobID, pfs[1].BlobID)
		//})
	})

	t.Run("Download", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		checkDownloadCount := func(count int64) {
			pvs, err := packages.GetVersionsByPackageType(t.Context(), user.ID, packages.TypeTerraform)
			require.NoError(t, err)
			assert.Len(t, pvs, 1)
			assert.Equal(t, count, pvs[0].DownloadCount)
		}

		checkDownloadCount(0)

		req := NewRequest(t, "GET", url)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, content, resp.Body.Bytes())

		checkDownloadCount(1)

		t.Run("RequireSignInView", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			defer test.MockVariableValue(&setting.Service.RequireSignInViewStrict, true)()

			req = NewRequest(t, "GET", url)
			MakeRequest(t, req, http.StatusUnauthorized)
		})
	})

	t.Run("Delete", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		t.Run("File", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "DELETE", url)
			MakeRequest(t, req, http.StatusUnauthorized)

			req = NewRequest(t, "DELETE", url).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusNoContent)

			req = NewRequest(t, "GET", url)
			MakeRequest(t, req, http.StatusNotFound)

			req = NewRequest(t, "DELETE", url).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusNotFound)

			pvs, err := packages.GetVersionsByPackageType(t.Context(), user.ID, packages.TypeTerraform)
			assert.NoError(t, err)
			assert.Empty(t, pvs)
		})

		t.Run("Version", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequestWithBody(t, "POST", url, bytes.NewReader(content)).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusCreated)

			req = NewRequest(t, "DELETE", url)
			MakeRequest(t, req, http.StatusUnauthorized)

			req = NewRequest(t, "DELETE", url).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusNoContent)

			pvs, err := packages.GetVersionsByPackageType(t.Context(), user.ID, packages.TypeTerraform)
			assert.NoError(t, err)
			assert.Empty(t, pvs)

			req = NewRequest(t, "GET", url)
			MakeRequest(t, req, http.StatusNotFound)

			req = NewRequest(t, "DELETE", url).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusNotFound)
		})
	})
}
