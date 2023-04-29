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
	vagrant_module "code.gitea.io/gitea/modules/packages/vagrant"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestPackageVagrant(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	token := "Bearer " + getUserToken(t, user.Name, auth_model.AccessTokenScopePackage)

	packageName := "test_package"
	packageVersion := "1.0.1"
	packageDescription := "Test Description"
	packageProvider := "virtualbox"

	filename := fmt.Sprintf("%s.box", packageProvider)

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

	root := fmt.Sprintf("/api/packages/%s/vagrant", user.Name)

	t.Run("Authenticate", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		authenticateURL := fmt.Sprintf("%s/authenticate", root)

		req := NewRequest(t, "GET", authenticateURL)
		MakeRequest(t, req, http.StatusUnauthorized)

		req = NewRequest(t, "GET", authenticateURL)
		addTokenAuthHeader(req, token)
		MakeRequest(t, req, http.StatusOK)
	})

	boxURL := fmt.Sprintf("%s/%s", root, packageName)

	t.Run("Upload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "HEAD", boxURL)
		MakeRequest(t, req, http.StatusNotFound)

		uploadURL := fmt.Sprintf("%s/%s/%s", boxURL, packageVersion, filename)

		req = NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader(content))
		MakeRequest(t, req, http.StatusUnauthorized)

		req = NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader(content))
		addTokenAuthHeader(req, token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequest(t, "HEAD", boxURL)
		resp := MakeRequest(t, req, http.StatusOK)
		assert.True(t, strings.HasPrefix(resp.Header().Get("Content-Type"), "application/json"))

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeVagrant)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
		assert.NoError(t, err)
		assert.NotNil(t, pd.SemVer)
		assert.IsType(t, &vagrant_module.Metadata{}, pd.Metadata)
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

		req = NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader(content))
		addTokenAuthHeader(req, token)
		MakeRequest(t, req, http.StatusConflict)
	})

	t.Run("Download", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/%s", boxURL, packageVersion, filename))
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, content, resp.Body.Bytes())
	})

	t.Run("EnumeratePackageVersions", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", boxURL)
		resp := MakeRequest(t, req, http.StatusOK)

		type providerData struct {
			Name         string `json:"name"`
			URL          string `json:"url"`
			Checksum     string `json:"checksum"`
			ChecksumType string `json:"checksum_type"`
		}

		type versionMetadata struct {
			Version             string          `json:"version"`
			Status              string          `json:"status"`
			DescriptionHTML     string          `json:"description_html,omitempty"`
			DescriptionMarkdown string          `json:"description_markdown,omitempty"`
			Providers           []*providerData `json:"providers"`
		}

		type packageMetadata struct {
			Name             string             `json:"name"`
			Description      string             `json:"description,omitempty"`
			ShortDescription string             `json:"short_description,omitempty"`
			Versions         []*versionMetadata `json:"versions"`
		}

		var result packageMetadata
		DecodeJSON(t, resp, &result)

		assert.Equal(t, packageName, result.Name)
		assert.Equal(t, packageDescription, result.Description)
		assert.Len(t, result.Versions, 1)
		version := result.Versions[0]
		assert.Equal(t, packageVersion, version.Version)
		assert.Equal(t, "active", version.Status)
		assert.Len(t, version.Providers, 1)
		provider := version.Providers[0]
		assert.Equal(t, packageProvider, provider.Name)
		assert.Equal(t, "sha512", provider.ChecksumType)
		assert.Equal(t, "259bebd6160acad695016d22a45812e26f187aaf78e71a4c23ee3201528346293f991af3468a8c6c5d2a21d7d9e1bdc1bf79b87110b2fddfcc5a0d45963c7c30", provider.Checksum)
	})
}
