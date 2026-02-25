// Copyright 2021 The Gitea Authors. All rights reserved.
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
		// 1. Lock the state
		lockID := uuid.New().String()
		lockInfo := genLock(lockID)
		req := NewRequestWithBody(t, "POST", lockURL, strings.NewReader(lockInfo)).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusOK)

		// Verify lock property in DB
		p, err := packages.GetPackageByName(t.Context(), user.ID, packages.TypeTerraform, packageName)
		require.NoError(t, err)
		props, err := packages.GetPropertiesByName(t.Context(), packages.PropertyTypePackage, p.ID, "terraform.lock")
		require.NoError(t, err)
		require.Len(t, props, 1)
		assert.Contains(t, props[0].Value, lockID)

		// Upload state with correct Lock ID
		state1 := genState(1)
		req = NewRequestWithBody(t, "POST", url+"?ID="+lockID, strings.NewReader(state1)).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)

		// Verify version created
		pv, err := packages.GetVersionByNameAndVersion(t.Context(), user.ID, packages.TypeTerraform, packageName, "1")
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
		_, err = packages.GetPackageByName(t.Context(), user.ID, packages.TypeTerraform, packageName)
		assert.ErrorIs(t, err, packages.ErrPackageNotExist)
	})

	t.Run("StateHistory", func(t *testing.T) {
		// Upload 3 versions
		for i := range 3 {
			state := genState(i)
			req := NewRequestWithBody(t, "POST", url, strings.NewReader(state)).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusCreated)
		}

		// Verify latest is 2
		req := NewRequest(t, "GET", url).AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, genState(2), resp.Body.String())

		// Verify version 1 is accessible
		req = NewRequest(t, "GET", url+"/versions/1").AddBasicAuth(user.Name)
		resp = MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, genState(1), resp.Body.String())

		// Delete version 1
		req = NewRequest(t, "DELETE", url+"/versions/1").AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNoContent)

		// Verify version 1 is gone from DB
		_, err := packages.GetVersionByNameAndVersion(t.Context(), user.ID, packages.TypeTerraform, packageName, "1")
		assert.ErrorIs(t, err, packages.ErrPackageNotExist)

		// Verify version 1 is gone from API
		req = NewRequest(t, "GET", url+"/versions/1").AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNotFound)

		// Deleting latest version (2) should be forbidden
		req = NewRequest(t, "DELETE", url+"/versions/2").AddBasicAuth(user.Name)
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

			// Verify package STILL EXISTS (testing the bug I found, though it might fail if I didn't fix it yet)
			// User said "Don't modify the logic that exists currently", so I'll just add the assertion.
			// If it fails, it proves the bug.
			p, err := packages.GetPackageByName(t.Context(), user.ID, packages.TypeTerraform, packageName)
			assert.NoError(t, err, "Package should still exist because it is locked")
			assert.NotNil(t, p)

			// Cleanup
			req = NewRequestWithBody(t, "DELETE", lockURL, strings.NewReader(lockInfo)).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusOK)
			req = NewRequest(t, "DELETE", url).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusOK)
		})
	})
}
