// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/tests"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageTerraform(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	packageName := "te-st_pac.kage"
	// generate the state json
	genState := func(serial int) string {
		return fmt.Sprintf(`{
			"version": 4,
			"terraform_version": "1.10.4",
			"serial": %d,
			"lineage": "bca3c5f6-01dc-cdad-5310-d1b12e02e430",
			"outputs": {},
			"resources": [{
				"mode": "managed",
				"type": "hello",
				"name": "null_resource",
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
		}`, serial)
	}
	genEncryptedState := func(serial int) string {
		// json taken from wireshark inspection
		return fmt.Sprintf(`{
    "serial": %d,
    "lineage": "4dad5e35-cbd5-ca2f-3c20-97c5e13b7033",
    "meta": {
        "key_provider.pbkdf2.foo": "eyJzYWx0IjoiWE55NnRDZTlSQnFNWGRqWm5xc202TU1DREZNbW5FbXRHczc2UXI0NEpXST0iLCJpdGVyYXRpb25zIjo2MDAwMDAsImhhc2hfZnVuY3Rpb24iOiJzaGE1MTIiLCJrZXlfbGVuZ3RoIjozMn0="
    },
    "encrypted_data": "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",
    "encryption_version": "v0"
}`, serial)
	}
	genLock := func(uuid string) string {
		return fmt.Sprintf(`{
			"ID": "%s",
			"Operation": "OperationTypePlan",
			"Info": "",
			"Who": "test-user@localhost",
			"Version": "1.0",
			"Created": "2023-01-01T00:00:00Z",
			"Path": "test.tfstate"
		}`, uuid)
	}

	url := fmt.Sprintf("/api/packages/%s/terraform/state/%s", user.Name, packageName)
	lockURL := fmt.Sprintf("/api/packages/%s/terraform/state/%s/lock", user.Name, packageName)

	// Covers non-existing package retrieval and deletion
	t.Run("GetOrDeleteNonExisting", func(t *testing.T) {
		// Package does not exist yet
		req := NewRequest(t, "GET", url).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNotFound)

		// So deleting it also should not work
		req = NewRequest(t, "DELETE", url).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNotFound)
	})
	t.Run("RegularOperations", func(t *testing.T) {
		cases := []struct {
			name      string
			statefunc func(int) string
		}{
			{
				name:      "Plain",
				statefunc: genState,
			},
			{
				name:      "Encrypted",
				statefunc: genEncryptedState,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				// 1. Lock the state
				lockID := uuid.New().String()
				lockInfo := genLock(lockID)
				req := NewRequestWithBody(t, "POST", lockURL, strings.NewReader(lockInfo)).AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusOK)

				// Verify lock property in DB
				p, err := packages.GetPackageByName(t.Context(), user.ID, packages.TypeTerraformState, packageName)
				require.NoError(t, err)
				props, err := packages.GetPropertiesByName(t.Context(), packages.PropertyTypePackage, p.ID, "terraform.lock")
				require.NoError(t, err)
				require.Len(t, props, 1)
				assert.Contains(t, props[0].Value, lockID)

				// Upload state with correct Lock ID
				state1 := tc.statefunc(1)
				req = NewRequestWithBody(t, "POST", url+"?ID="+lockID, strings.NewReader(state1)).AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusCreated)

				// Verify version created
				pv, err := packages.GetVersionByNameAndVersion(t.Context(), user.ID, packages.TypeTerraformState, packageName, "1")
				assert.NoError(t, err)
				assert.NotNil(t, pv)

				// 3. Unlock the state
				req = NewRequestWithBody(t, "DELETE", lockURL, strings.NewReader(lockInfo)).AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusOK)

				// Verify lock property is cleared
				props, err = packages.GetPropertiesByName(t.Context(), packages.PropertyTypePackage, p.ID, "terraform.lock")
				require.NoError(t, err)
				require.Len(t, props, 1)
				assert.Empty(t, props[0].Value)

				// Get latest state
				req = NewRequest(t, "GET", url).AddBasicAuth(user.Name)
				resp := MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, state1, resp.Body.String())

				// Upload new version without lock
				state2 := genState(2)
				req = NewRequestWithBody(t, "POST", url, strings.NewReader(state2)).AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusCreated)

				// 6. Delete the entire package
				req = NewRequest(t, "DELETE", url).AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusOK)

				// Verify package is deleted from DB
				_, err = packages.GetPackageByName(t.Context(), user.ID, packages.TypeTerraformState, packageName)
				assert.ErrorIs(t, err, packages.ErrPackageNotExist)
			})
		}
	})

	t.Run("StateHistory", func(t *testing.T) {
		// Upload 3 versions
		for i := range 3 {
			state := genState(i + 1) // 1-based
			req := NewRequestWithBody(t, "POST", url, strings.NewReader(state)).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusCreated)
		}

		// Verify latest is 3
		req := NewRequest(t, "GET", url).AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, genState(3), resp.Body.String())

		// Verify version 2 is accessible
		req = NewRequest(t, "GET", url+"/versions/2").AddBasicAuth(user.Name)
		resp = MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, genState(2), resp.Body.String())

		// Delete version 2
		req = NewRequest(t, "DELETE", url+"/versions/2").AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNoContent)

		// Verify version 2 is gone from DB
		_, err := packages.GetVersionByNameAndVersion(t.Context(), user.ID, packages.TypeTerraformState, packageName, "2")
		assert.ErrorIs(t, err, packages.ErrPackageNotExist)

		// Verify version 2 is gone from API
		req = NewRequest(t, "GET", url+"/versions/2").AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNotFound)

		// Deleting latest version (3) should be forbidden
		req = NewRequest(t, "DELETE", url+"/versions/3").AddBasicAuth(user.Name)
		resp = MakeRequest(t, req, http.StatusForbidden)
		assert.Contains(t, resp.Body.String(), "cannot delete the latest version")

		// Cleanup
		req = NewRequest(t, "DELETE", url).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusOK)
	})

	t.Run("BadOperations", func(t *testing.T) {
		t.Run("LockingIssues", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			lockID1 := uuid.New().String()
			lockID2 := uuid.New().String()
			lockInfo1 := genLock(lockID1)
			lockInfo2 := genLock(lockID2)

			// Pre-create package - it's required for unlock on the non-locked package to work
			req := NewRequestWithBody(t, "POST", url, strings.NewReader(genState(1))).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusCreated)

			// Unlock non-locked state (should return 200)
			req = NewRequestWithBody(t, "DELETE", lockURL, strings.NewReader(lockInfo1)).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusOK)

			// Lock the state
			req = NewRequestWithBody(t, "POST", lockURL, strings.NewReader(lockInfo1)).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusOK)

			// Another lock attempt should fail
			req = NewRequestWithBody(t, "POST", lockURL, strings.NewReader(lockInfo2)).AddBasicAuth(user.Name)
			resp := MakeRequest(t, req, http.StatusLocked)
			assert.JSONEq(t, lockInfo1, resp.Body.String())

			// Unlock with wrong ID should fail
			req = NewRequestWithBody(t, "DELETE", lockURL, strings.NewReader(lockInfo2)).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusLocked)

			// Same user locking again should fail (already locked)
			req = NewRequestWithBody(t, "POST", lockURL, strings.NewReader(lockInfo1)).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusLocked)

			// Unlock with correct ID
			req = NewRequestWithBody(t, "DELETE", lockURL, strings.NewReader(lockInfo1)).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusOK)

			// Clean up
			req = NewRequest(t, "DELETE", url).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusOK)
		})

		t.Run("UploadWithoutValidLock", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			lockID := uuid.New().String()
			lockInfo := genLock(lockID)

			// Lock the state
			req := NewRequestWithBody(t, "POST", lockURL, strings.NewReader(lockInfo)).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusOK)

			// Upload without ID should fail
			req = NewRequestWithBody(t, "POST", url, strings.NewReader(genState(1))).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusLocked)

			// Upload with wrong ID should fail
			req = NewRequestWithBody(t, "POST", url+"?ID="+uuid.New().String(), strings.NewReader(genState(1))).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusLocked)

			// Cleanup lock
			req = NewRequestWithBody(t, "DELETE", lockURL, strings.NewReader(lockInfo)).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusOK)
		})

		t.Run("DeleteWithLock", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// Create package and lock it
			req := NewRequestWithBody(t, "POST", url, strings.NewReader(genState(1))).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusCreated)

			lockID := uuid.New().String()
			lockInfo := genLock(lockID)
			req = NewRequestWithBody(t, "POST", lockURL, strings.NewReader(lockInfo)).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusOK)

			// Delete package should fail
			req = NewRequest(t, "DELETE", url).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusLocked)

			// Verify package exists
			p, err := packages.GetPackageByName(t.Context(), user.ID, packages.TypeTerraformState, packageName)
			require.NoError(t, err)
			assert.NotNil(t, p)

			// Cleanup
			req = NewRequestWithBody(t, "DELETE", lockURL, strings.NewReader(lockInfo)).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusOK)
			req = NewRequest(t, "DELETE", url).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusOK)
		})
		t.Run("PutEmpty", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// safeguard against null payload
			req := NewRequestWithBody(t, "POST", url, strings.NewReader("null")).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusBadRequest)
		})
	})
}
