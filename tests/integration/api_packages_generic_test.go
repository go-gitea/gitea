// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestPackageGeneric(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	packageName := "te-st_pac.kage"
	packageVersion := "1.0.3-te st"
	filename := "fi-le_na.me"
	content := []byte{1, 2, 3}

	url := fmt.Sprintf("/api/packages/%s/generic/%s/%s", user.Name, packageName, packageVersion)

	t.Run("Upload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequestWithBody(t, "PUT", url+"/"+filename, bytes.NewReader(content))
		AddBasicAuthHeader(req, user.Name)
		MakeRequest(t, req, http.StatusCreated)

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeGeneric)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
		assert.NoError(t, err)
		assert.Nil(t, pd.Metadata)
		assert.Equal(t, packageName, pd.Package.Name)
		assert.Equal(t, packageVersion, pd.Version.Version)

		pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
		assert.NoError(t, err)
		assert.Len(t, pfs, 1)
		assert.Equal(t, filename, pfs[0].Name)
		assert.True(t, pfs[0].IsLead)

		pb, err := packages.GetBlobByID(db.DefaultContext, pfs[0].BlobID)
		assert.NoError(t, err)
		assert.Equal(t, int64(len(content)), pb.Size)

		t.Run("Exists", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequestWithBody(t, "PUT", url+"/"+filename, bytes.NewReader(content))
			AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusConflict)
		})

		t.Run("Additional", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequestWithBody(t, "PUT", url+"/dummy.bin", bytes.NewReader(content))
			AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusCreated)

			// Check deduplication
			pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
			assert.NoError(t, err)
			assert.Len(t, pfs, 2)
			assert.Equal(t, pfs[0].BlobID, pfs[1].BlobID)
		})

		t.Run("InvalidParameter", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequestWithBody(t, "PUT", fmt.Sprintf("/api/packages/%s/generic/%s/%s/%s", user.Name, "invalid+package name", packageVersion, filename), bytes.NewReader(content))
			AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusBadRequest)

			req = NewRequestWithBody(t, "PUT", fmt.Sprintf("/api/packages/%s/generic/%s/%s/%s", user.Name, packageName, "%20test ", filename), bytes.NewReader(content))
			AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusBadRequest)

			req = NewRequestWithBody(t, "PUT", fmt.Sprintf("/api/packages/%s/generic/%s/%s/%s", user.Name, packageName, packageVersion, "inval+id.na me"), bytes.NewReader(content))
			AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusBadRequest)
		})
	})

	t.Run("Download", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		checkDownloadCount := func(count int64) {
			pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeGeneric)
			assert.NoError(t, err)
			assert.Len(t, pvs, 1)
			assert.Equal(t, count, pvs[0].DownloadCount)
		}

		checkDownloadCount(0)

		req := NewRequest(t, "GET", url+"/"+filename)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, content, resp.Body.Bytes())

		checkDownloadCount(1)

		req = NewRequest(t, "GET", url+"/dummy.bin")
		MakeRequest(t, req, http.StatusOK)

		checkDownloadCount(2)

		t.Run("NotExists", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", url+"/not.found")
			MakeRequest(t, req, http.StatusNotFound)
		})

		t.Run("RequireSignInView", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			setting.Service.RequireSignInView = true
			defer func() {
				setting.Service.RequireSignInView = false
			}()

			req = NewRequest(t, "GET", url+"/dummy.bin")
			MakeRequest(t, req, http.StatusUnauthorized)
		})
	})

	t.Run("Delete", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		t.Run("File", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "DELETE", url+"/"+filename)
			MakeRequest(t, req, http.StatusUnauthorized)

			req = NewRequest(t, "DELETE", url+"/"+filename)
			AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusNoContent)

			req = NewRequest(t, "GET", url+"/"+filename)
			MakeRequest(t, req, http.StatusNotFound)

			req = NewRequest(t, "DELETE", url+"/"+filename)
			AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusNotFound)

			pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeGeneric)
			assert.NoError(t, err)
			assert.Len(t, pvs, 1)

			t.Run("RemovesVersion", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req = NewRequest(t, "DELETE", url+"/dummy.bin")
				AddBasicAuthHeader(req, user.Name)
				MakeRequest(t, req, http.StatusNoContent)

				pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeGeneric)
				assert.NoError(t, err)
				assert.Empty(t, pvs)
			})
		})

		t.Run("Version", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequestWithBody(t, "PUT", url+"/"+filename, bytes.NewReader(content))
			AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusCreated)

			req = NewRequest(t, "DELETE", url)
			MakeRequest(t, req, http.StatusUnauthorized)

			req = NewRequest(t, "DELETE", url)
			AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusNoContent)

			pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeGeneric)
			assert.NoError(t, err)
			assert.Empty(t, pvs)

			req = NewRequest(t, "GET", url+"/"+filename)
			MakeRequest(t, req, http.StatusNotFound)

			req = NewRequest(t, "DELETE", url)
			AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusNotFound)
		})
	})
}
