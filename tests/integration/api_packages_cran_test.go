// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"testing"

	"gitea.dev/models/packages"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	cran_module "gitea.dev/modules/packages/cran"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

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
		t.Run("Upload", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			uploadURL := url + "/src"

			req := NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader([]byte{}))
			MakeRequest(t, req, http.StatusUnauthorized)

			req = NewRequestWithBody(t, "PUT", uploadURL, test.WriteTarCompression(gzip.NewWriter, map[string]string{
				"dummy.txt": "",
			})).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusBadRequest)

			req = NewRequestWithBody(t, "PUT", uploadURL, test.WriteTarCompression(gzip.NewWriter, map[string]string{
				"package/DESCRIPTION": string(createDescription(packageName, packageVersion)),
			})).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusCreated)

			pvs, err := packages.GetVersionsByPackageType(t.Context(), user.ID, packages.TypeCran)
			assert.NoError(t, err)
			assert.Len(t, pvs, 1)

			pd, err := packages.GetPackageDescriptor(t.Context(), pvs[0])
			assert.NoError(t, err)
			assert.Nil(t, pd.SemVer)
			assert.IsType(t, &cran_module.Metadata{}, pd.Metadata)
			assert.Equal(t, packageName, pd.Package.Name)
			assert.Equal(t, packageVersion, pd.Version.Version)

			pfs, err := packages.GetFilesByVersionID(t.Context(), pvs[0].ID)
			assert.NoError(t, err)
			assert.Len(t, pfs, 1)
			assert.Equal(t, fmt.Sprintf("%s_%s.tar.gz", packageName, packageVersion), pfs[0].Name)
			assert.True(t, pfs[0].IsLead)

			req = NewRequestWithBody(t, "PUT", uploadURL, test.WriteTarCompression(gzip.NewWriter, map[string]string{
				"package/DESCRIPTION": string(createDescription(packageName, packageVersion)),
			})).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusConflict)
		})

		t.Run("Download", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/src/contrib/%s_%s.tar.gz", url, packageName, packageVersion)).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusOK)
		})

		t.Run("DownloadArchived", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/src/contrib/Archive/%s/%s_%s.tar.gz", url, packageName, packageName, packageVersion)).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusOK)
		})

		t.Run("Enumerate", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", url+"/src/contrib/PACKAGES").
				AddBasicAuth(user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			assert.Contains(t, resp.Header().Get("Content-Type"), "text/plain")

			body := resp.Body.String()
			assert.Contains(t, body, "Package: "+packageName)
			assert.Contains(t, body, "Version: "+packageVersion)

			req = NewRequest(t, "GET", url+"/src/contrib/PACKAGES.gz").
				AddBasicAuth(user.Name)
			resp = MakeRequest(t, req, http.StatusOK)

			assert.Contains(t, resp.Header().Get("Content-Type"), "application/x-gzip")
		})
	})

	t.Run("Binary", func(t *testing.T) {
		t.Run("Upload", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			uploadURL := url + "/bin"

			req := NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader([]byte{}))
			MakeRequest(t, req, http.StatusUnauthorized)

			req = NewRequestWithBody(t, "PUT", uploadURL, test.WriteZipArchive(map[string]string{
				"dummy.txt": "",
			})).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusBadRequest)

			req = NewRequestWithBody(t, "PUT", uploadURL+"?platform=&rversion=", test.WriteZipArchive(map[string]string{
				"package/DESCRIPTION": string(createDescription(packageName, packageVersion)),
			})).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusBadRequest)

			uploadURL += "?platform=windows&rversion=4.2"

			req = NewRequestWithBody(t, "PUT", uploadURL, test.WriteZipArchive(map[string]string{
				"package/DESCRIPTION": string(createDescription(packageName, packageVersion)),
			})).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusCreated)

			pvs, err := packages.GetVersionsByPackageType(t.Context(), user.ID, packages.TypeCran)
			assert.NoError(t, err)
			assert.Len(t, pvs, 1)

			pfs, err := packages.GetFilesByVersionID(t.Context(), pvs[0].ID)
			assert.NoError(t, err)
			assert.Len(t, pfs, 2)

			req = NewRequestWithBody(t, "PUT", uploadURL, test.WriteZipArchive(map[string]string{
				"package/DESCRIPTION": string(createDescription(packageName, packageVersion)),
			})).AddBasicAuth(user.Name)
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
				req := NewRequest(t, "GET", fmt.Sprintf("%s/bin/%s/contrib/%s/%s_%s.zip", url, c.Platform, c.RVersion, packageName, packageVersion)).
					AddBasicAuth(user.Name)
				MakeRequest(t, req, c.ExpectedStatus)
			}
		})

		t.Run("Enumerate", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", url+"/bin/windows/contrib/4.1/PACKAGES")
			MakeRequest(t, req, http.StatusNotFound)

			req = NewRequest(t, "GET", url+"/bin/windows/contrib/4.2/PACKAGES").
				AddBasicAuth(user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			assert.Contains(t, resp.Header().Get("Content-Type"), "text/plain")

			body := resp.Body.String()
			assert.Contains(t, body, "Package: "+packageName)
			assert.Contains(t, body, "Version: "+packageVersion)

			req = NewRequest(t, "GET", url+"/bin/windows/contrib/4.2/PACKAGES.gz").
				AddBasicAuth(user.Name)
			resp = MakeRequest(t, req, http.StatusOK)

			assert.Contains(t, resp.Header().Get("Content-Type"), "application/x-gzip")
		})
	})

	t.Run("EnumerateAfterOlderUpload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequestWithBody(t, "PUT", url+"/src", test.WriteTarCompression(gzip.NewWriter, map[string]string{
			"package/DESCRIPTION": string(createDescription(packageName, "1.0.2")),
		})).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)

		for _, indexURL := range []string{url + "/src/contrib/PACKAGES", url + "/bin/windows/contrib/4.2/PACKAGES"} {
			req = NewRequest(t, "GET", indexURL).AddBasicAuth(user.Name)
			resp := MakeRequest(t, req, http.StatusOK)
			assert.Contains(t, resp.Body.String(), "Version: "+packageVersion)
		}
	})
}
