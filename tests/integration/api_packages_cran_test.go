// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitea.dev/models/packages"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	cran_module "gitea.dev/modules/packages/cran"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			)).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusBadRequest)

			req = NewRequestWithBody(t, "PUT", uploadURL, createArchive(
				"package/DESCRIPTION",
				createDescription(packageName, packageVersion),
			)).AddBasicAuth(user.Name)
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

			req = NewRequestWithBody(t, "PUT", uploadURL, createArchive(
				"package/DESCRIPTION",
				createDescription(packageName, packageVersion),
			)).AddBasicAuth(user.Name)
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
}

func TestPackageCranLatestVersion(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	baseURL := fmt.Sprintf("/api/packages/%s/cran", user.Name)
	name := "ordered.package"
	upload := func(version, platform, rversion string) {
		t.Helper()
		description := fmt.Sprintf("Package: %s\nVersion: %s\nLicense: MIT\n", name, version)
		var archive io.Reader
		uploadURL := baseURL + "/src"
		if platform == "" {
			var buf bytes.Buffer
			gw := gzip.NewWriter(&buf)
			tw := tar.NewWriter(gw)
			require.NoError(t, tw.WriteHeader(&tar.Header{Name: "package/DESCRIPTION", Mode: 0o600, Size: int64(len(description))}))
			_, err := tw.Write([]byte(description))
			require.NoError(t, err)
			require.NoError(t, tw.Close())
			require.NoError(t, gw.Close())
			archive = &buf
		} else {
			archive = test.WriteZipArchive(map[string]string{"package/DESCRIPTION": description})
			uploadURL = fmt.Sprintf("%s/bin?platform=%s&rversion=%s", baseURL, platform, rversion)
		}
		MakeRequest(t, NewRequestWithBody(t, "PUT", uploadURL, archive).AddBasicAuth(user.Name), http.StatusCreated)
	}
	checkIndex := func(path, version string) {
		t.Helper()
		for _, suffix := range []string{"", ".gz"} {
			resp := MakeRequest(t, NewRequest(t, "GET", baseURL+path+"/PACKAGES"+suffix), http.StatusOK)
			var reader io.Reader = resp.Body
			if suffix != "" {
				gz, err := gzip.NewReader(resp.Body)
				require.NoError(t, err)
				defer gz.Close()
				reader = gz
			}
			body, err := io.ReadAll(reader)
			require.NoError(t, err)
			assert.Contains(t, string(body), "Package: "+name+"\nVersion: "+version+"\n")
			assert.Equal(t, 1, strings.Count(string(body), "Package: "+name+"\n"))
		}
	}

	for _, tc := range []struct{ upload, want string }{
		{"3.0.2", "3.0.2"},
		{"2.5.2", "3.0.2"},
		{"3.0.10", "3.0.10"},
		{"3.0.9", "3.0.10"},
		{"3.0-11", "3.0-11"},
		{"3.0.10.1", "3.0-11"},
	} {
		upload(tc.upload, "", "")
		checkIndex("/src/contrib", tc.want)
	}

	upload("2.5.2", "windows", "4.2")
	upload("3.0.2", "windows", "4.3")
	upload("4.0", "osx", "4.3")
	checkIndex("/src/contrib", "3.0-11")
	checkIndex("/bin/windows/contrib/4.2", "2.5.2")
	checkIndex("/bin/windows/contrib/4.3", "3.0.2")
	checkIndex("/bin/osx/contrib/4.3", "4.0")
	MakeRequest(t, NewRequest(t, "GET", baseURL+"/bin/windows/contrib/4.1/PACKAGES"), http.StatusNotFound)
	MakeRequest(t, NewRequest(t, "GET", baseURL+"/src/contrib/Archive/"+name+"/"+name+"_2.5.2.tar.gz"), http.StatusOK)
}
