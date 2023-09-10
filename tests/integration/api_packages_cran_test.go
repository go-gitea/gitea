// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	cran_module "code.gitea.io/gitea/modules/packages/cran"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestPackageCran(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	packageName := "test.package"
	packageVersion := "1.0.3"
	packageAuthor := "KN4CK3R"
	packageDescription := "Gitea Test Package"

	createDescription := func(name, version string) []byte {
		var buf bytes.Buffer
		fmt.Fprintln(&buf, "Package:", name)
		fmt.Fprintln(&buf, "Version:", version)
		fmt.Fprintln(&buf, "Description:", packageDescription)
		fmt.Fprintln(&buf, "Imports: abc,\n123")
		fmt.Fprintln(&buf, "NeedsCompilation: yes")
		fmt.Fprintln(&buf, "License: MIT")
		fmt.Fprintln(&buf, "Author:", packageAuthor)
		return buf.Bytes()
	}

	url := fmt.Sprintf("/api/packages/%s/cran", user.Name)

	t.Run("Source", func(t *testing.T) {
		createArchive := func(filename string, content []byte) *bytes.Buffer {
			var buf bytes.Buffer
			gw := gzip.NewWriter(&buf)
			tw := tar.NewWriter(gw)
			hdr := &tar.Header{
				Name: filename,
				Mode: 0o600,
				Size: int64(len(content)),
			}
			tw.WriteHeader(hdr)
			tw.Write(content)
			tw.Close()
			gw.Close()
			return &buf
		}

		t.Run("Upload", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			uploadURL := url + "/src"

			req := NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader([]byte{}))
			MakeRequest(t, req, http.StatusUnauthorized)

			req = NewRequestWithBody(t, "PUT", uploadURL, createArchive(
				"dummy.txt",
				[]byte{},
			))
			req = AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusBadRequest)

			req = NewRequestWithBody(t, "PUT", uploadURL, createArchive(
				"package/DESCRIPTION",
				createDescription(packageName, packageVersion),
			))
			req = AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusCreated)

			pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeCran)
			assert.NoError(t, err)
			assert.Len(t, pvs, 1)

			pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
			assert.NoError(t, err)
			assert.Nil(t, pd.SemVer)
			assert.IsType(t, &cran_module.Metadata{}, pd.Metadata)
			assert.Equal(t, packageName, pd.Package.Name)
			assert.Equal(t, packageVersion, pd.Version.Version)

			pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
			assert.NoError(t, err)
			assert.Len(t, pfs, 1)
			assert.Equal(t, fmt.Sprintf("%s_%s.tar.gz", packageName, packageVersion), pfs[0].Name)
			assert.True(t, pfs[0].IsLead)

			req = NewRequestWithBody(t, "PUT", uploadURL, createArchive(
				"package/DESCRIPTION",
				createDescription(packageName, packageVersion),
			))
			req = AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusConflict)
		})

		t.Run("Download", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/src/contrib/%s_%s.tar.gz", url, packageName, packageVersion))
			req = AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusOK)
		})

		t.Run("Enumerate", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", url+"/src/contrib/PACKAGES")
			req = AddBasicAuthHeader(req, user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			assert.Contains(t, resp.Header().Get("Content-Type"), "text/plain")

			body := resp.Body.String()
			assert.Contains(t, body, fmt.Sprintf("Package: %s", packageName))
			assert.Contains(t, body, fmt.Sprintf("Version: %s", packageVersion))

			req = NewRequest(t, "GET", url+"/src/contrib/PACKAGES.gz")
			req = AddBasicAuthHeader(req, user.Name)
			resp = MakeRequest(t, req, http.StatusOK)

			assert.Contains(t, resp.Header().Get("Content-Type"), "application/x-gzip")
		})
	})

	t.Run("Binary", func(t *testing.T) {
		createArchive := func(filename string, content []byte) *bytes.Buffer {
			var buf bytes.Buffer
			archive := zip.NewWriter(&buf)
			w, _ := archive.Create(filename)
			w.Write(content)
			archive.Close()
			return &buf
		}

		t.Run("Upload", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			uploadURL := url + "/bin"

			req := NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader([]byte{}))
			MakeRequest(t, req, http.StatusUnauthorized)

			req = NewRequestWithBody(t, "PUT", uploadURL, createArchive(
				"dummy.txt",
				[]byte{},
			))
			req = AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusBadRequest)

			req = NewRequestWithBody(t, "PUT", uploadURL+"?platform=&rversion=", createArchive(
				"package/DESCRIPTION",
				createDescription(packageName, packageVersion),
			))
			req = AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusBadRequest)

			uploadURL += "?platform=windows&rversion=4.2"

			req = NewRequestWithBody(t, "PUT", uploadURL, createArchive(
				"package/DESCRIPTION",
				createDescription(packageName, packageVersion),
			))
			req = AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusCreated)

			pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeCran)
			assert.NoError(t, err)
			assert.Len(t, pvs, 1)

			pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
			assert.NoError(t, err)
			assert.Len(t, pfs, 2)

			req = NewRequestWithBody(t, "PUT", uploadURL, createArchive(
				"package/DESCRIPTION",
				createDescription(packageName, packageVersion),
			))
			req = AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusConflict)
		})

		t.Run("Download", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			cases := []struct {
				Platform       string
				RVersion       string
				ExpectedStatus int
			}{
				{"osx", "4.2", http.StatusNotFound},
				{"windows", "4.1", http.StatusNotFound},
				{"windows", "4.2", http.StatusOK},
			}

			for _, c := range cases {
				req := NewRequest(t, "GET", fmt.Sprintf("%s/bin/%s/contrib/%s/%s_%s.zip", url, c.Platform, c.RVersion, packageName, packageVersion))
				req = AddBasicAuthHeader(req, user.Name)
				MakeRequest(t, req, c.ExpectedStatus)
			}
		})

		t.Run("Enumerate", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", url+"/bin/windows/contrib/4.1/PACKAGES")
			MakeRequest(t, req, http.StatusNotFound)

			req = NewRequest(t, "GET", url+"/bin/windows/contrib/4.2/PACKAGES")
			req = AddBasicAuthHeader(req, user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			assert.Contains(t, resp.Header().Get("Content-Type"), "text/plain")

			body := resp.Body.String()
			assert.Contains(t, body, fmt.Sprintf("Package: %s", packageName))
			assert.Contains(t, body, fmt.Sprintf("Version: %s", packageVersion))

			req = NewRequest(t, "GET", url+"/bin/windows/contrib/4.2/PACKAGES.gz")
			req = AddBasicAuthHeader(req, user.Name)
			resp = MakeRequest(t, req, http.StatusOK)

			assert.Contains(t, resp.Header().Get("Content-Type"), "application/x-gzip")
		})
	})
}
