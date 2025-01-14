// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	terraform_module "code.gitea.io/gitea/modules/packages/terraform"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestPackageTerraform(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	token := "Bearer " + getUserToken(t, user.Name, auth_model.AccessTokenScopeWritePackage)

	packageName := "test_module"
	packageVersion := "1.0.1"
	packageDescription := "Test Terraform Module"

	filename := "terraform_module.tar.gz"

	infoContent, _ := json.Marshal(map[string]string{
		"description": packageDescription,
	})

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	archive := tar.NewWriter(zw)
	archive.WriteHeader(&tar.Header{
		Name: "info.json",
		Mode: 0o600,
		Size: int64(len(infoContent)),
	})
	archive.Write(infoContent)
	archive.Close()
	zw.Close()
	content := buf.Bytes()

	root := fmt.Sprintf("/api/packages/%s/terraform", user.Name)

	t.Run("Authenticate", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		authenticateURL := fmt.Sprintf("%s/authenticate", root)

		req := NewRequest(t, "GET", authenticateURL)
		MakeRequest(t, req, http.StatusUnauthorized)

		req = NewRequest(t, "GET", authenticateURL).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusOK)
	})

	moduleURL := fmt.Sprintf("%s/%s", root, packageName)

	t.Run("Upload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "HEAD", moduleURL)
		MakeRequest(t, req, http.StatusNotFound)

		uploadURL := fmt.Sprintf("%s/%s/%s", moduleURL, packageVersion, filename)

		req = NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader(content))
		MakeRequest(t, req, http.StatusUnauthorized)

		req = NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader(content)).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequest(t, "HEAD", moduleURL)
		resp := MakeRequest(t, req, http.StatusOK)
		assert.True(t, strings.HasPrefix(resp.Header().Get("Content-Type"), "application/json"))

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeTerraform)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
		assert.NoError(t, err)
		assert.NotNil(t, pd.SemVer)
		assert.IsType(t, &terraform_module.Metadata{}, pd.Metadata)
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

		req = NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader(content)).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusConflict)
	})

	t.Run("Download", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/%s", moduleURL, packageVersion, filename))
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, content, resp.Body.Bytes())
	})

	t.Run("EnumeratePackageVersions", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", moduleURL)
		resp := MakeRequest(t, req, http.StatusOK)

		type versionMetadata struct {
			Version string `json:"version"`
			Status  string `json:"status"`
		}

		type packageMetadata struct {
			Name        string             `json:"name"`
			Description string             `json:"description,omitempty"`
			Versions    []*versionMetadata `json:"versions"`
		}

		var result packageMetadata
		DecodeJSON(t, resp, &result)

		assert.Equal(t, packageName, result.Name)
		assert.Equal(t, packageDescription, result.Description)
		assert.Len(t, result.Versions, 1)
		version := result.Versions[0]
		assert.Equal(t, packageVersion, version.Version)
		assert.Equal(t, "active", version.Status)
	})
}
