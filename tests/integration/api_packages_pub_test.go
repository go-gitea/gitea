// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	pub_module "code.gitea.io/gitea/modules/packages/pub"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestPackagePub(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	token := "Bearer " + getUserToken(t, user.Name, auth_model.AccessTokenScopeWritePackage)

	packageName := "test_package"
	packageVersion := "1.0.1"
	packageDescription := "Test Description"

	filename := fmt.Sprintf("%s.tar.gz", packageVersion)

	pubspecContent := `name: ` + packageName + `
version: ` + packageVersion + `
description: ` + packageDescription

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	archive := tar.NewWriter(zw)
	archive.WriteHeader(&tar.Header{
		Name: "pubspec.yaml",
		Mode: 0o600,
		Size: int64(len(pubspecContent)),
	})
	archive.Write([]byte(pubspecContent))
	archive.Close()
	zw.Close()
	content := buf.Bytes()

	root := fmt.Sprintf("/api/packages/%s/pub", user.Name)

	t.Run("Upload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		uploadURL := root + "/api/packages/versions/new"

		req := NewRequest(t, "GET", uploadURL)
		MakeRequest(t, req, http.StatusUnauthorized)

		req = NewRequest(t, "GET", uploadURL)
		addTokenAuthHeader(req, token)
		resp := MakeRequest(t, req, http.StatusOK)

		type UploadRequest struct {
			URL    string            `json:"url"`
			Fields map[string]string `json:"fields"`
		}

		var result UploadRequest
		DecodeJSON(t, resp, &result)

		assert.Empty(t, result.Fields)

		uploadFile := func(t *testing.T, url string, content []byte, expectedStatus int) *httptest.ResponseRecorder {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, _ := writer.CreateFormFile("file", "dummy.tar.gz")
			_, _ = io.Copy(part, bytes.NewReader(content))

			_ = writer.Close()

			req := NewRequestWithBody(t, "POST", url, body)
			req.Header.Add("Content-Type", writer.FormDataContentType())
			addTokenAuthHeader(req, token)
			return MakeRequest(t, req, expectedStatus)
		}

		resp = uploadFile(t, result.URL, content, http.StatusNoContent)

		req = NewRequest(t, "GET", resp.Header().Get("Location"))
		addTokenAuthHeader(req, token)
		MakeRequest(t, req, http.StatusOK)

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypePub)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
		assert.NoError(t, err)
		assert.NotNil(t, pd.SemVer)
		assert.IsType(t, &pub_module.Metadata{}, pd.Metadata)
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

		_ = uploadFile(t, result.URL, content, http.StatusConflict)
	})

	t.Run("Download", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/api/packages/%s/%s", root, packageName, packageVersion))
		resp := MakeRequest(t, req, http.StatusOK)

		type VersionMetadata struct {
			Version    string    `json:"version"`
			ArchiveURL string    `json:"archive_url"`
			Published  time.Time `json:"published"`
			Pubspec    any       `json:"pubspec,omitempty"`
		}

		var result VersionMetadata
		DecodeJSON(t, resp, &result)

		assert.Equal(t, packageVersion, result.Version)
		assert.NotNil(t, result.Pubspec)

		req = NewRequest(t, "GET", result.ArchiveURL)
		resp = MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, content, resp.Body.Bytes())
	})

	t.Run("EnumeratePackageVersions", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/api/packages/%s", root, packageName))
		resp := MakeRequest(t, req, http.StatusOK)

		type VersionMetadata struct {
			Version    string    `json:"version"`
			ArchiveURL string    `json:"archive_url"`
			Published  time.Time `json:"published"`
			Pubspec    any       `json:"pubspec,omitempty"`
		}

		type PackageVersions struct {
			Name     string             `json:"name"`
			Latest   *VersionMetadata   `json:"latest"`
			Versions []*VersionMetadata `json:"versions"`
		}

		var result PackageVersions
		DecodeJSON(t, resp, &result)

		assert.Equal(t, packageName, result.Name)
		assert.NotNil(t, result.Latest)
		assert.Len(t, result.Versions, 1)
		assert.Equal(t, result.Latest.Version, result.Versions[0].Version)
		assert.Equal(t, packageVersion, result.Latest.Version)
		assert.NotNil(t, result.Latest.Pubspec)
	})
}
