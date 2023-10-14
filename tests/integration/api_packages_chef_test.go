// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"hash"
	"math/big"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	chef_module "code.gitea.io/gitea/modules/packages/chef"
	"code.gitea.io/gitea/modules/setting"
	chef_router "code.gitea.io/gitea/routers/api/packages/chef"
	"code.gitea.io/gitea/tests"

	"github.com/minio/sha256-simd"
	"github.com/stretchr/testify/assert"
)

func TestPackageChef(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	privPem := `-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEAtWp2PZz4TSU5A6ixw41HdbfBuGJwPuTtrsdoUf0DQ0/DJBNP
qOCBAgEu6ZdUqIbWJ5Da+nevjtncy5hENdi6XrXjyzlUxghMuXjE5SeLGpgfQvkq
bTkYaFpMe8PTzNeze3fei8+Eu6mzeb6g1GrqXznuPIc7bNss0w5iX9RiBM9dWPuX
onx9xSEy0LYqJm7yXmshNe1aRwkjG/y5C26BzBFnMKp9YRTua0DO1WqLNhcaRnda
lIFYouDNVTbwxSlYL16bZVoebqzZvLGrPvZJkPuCu6vH9brvOuYo0q8hLVNkBeXc
imRpsDjLhQYzEJjoMTbaiVGnjBky+PWNiofJnwIDAQABAoIBAQCotF1KxLt/ejr/
9ROCh9JJXV3v6tL5GgkSPOv9Oq2bHgSZer/cixJNW+5VWd5nbiSe3K1WuJBw5pbW
Wj4sWORPiRRR+3mjQzqeS/nGJDTOwWJo9K8IrUzOVhLEEYLX/ksxaXJyT8PehFyb
vbNwdhCIB6ZNcXDItTWE+95twWJ5lxAIj2dNwZZni3UkwwjYnCnqFtvHCKOg0NH2
RjQcFYmu3fncNeqLezUSdVyRyXxSCHsUdlYeX/e44StCnXdrmLUHlb2P27ZVdPGh
SW7qTUPpmJKekYiRPOpTLj+ZKXIsANkyWO+7dVtZLBm5bIyAsmp0W/DmK+wRsejj
alFbIsh5AoGBANJr7HSG695wkfn+kvu/V8qHbt+KDv4WjWHjGRsUqvxoHOUNkQmW
vZWdk4gjHYn1l+QHWmoOE3AgyqtCZ4bFILkZPLN/F8Mh3+r4B0Ac4biJJt7XGMNQ
Nv4wsk7TR7CCARsjO7GP1PT60hpjMvYmc1E36gNM7QIZE9jBE+L8eWYtAoGBANy2
JOAWf+QeBlur6o9feH76cEmpQzUUq4Lj9mmnXgIirSsFoBnDb8VA6Ws+ltL9U9H2
vaCoaTyi9twW9zWj+Ywg2mVR5nlSAPfdlTWS1GLUbDotlj5apc/lvnGuNlWzN+I4
Tu64hhgBXqGvRZ0o7HzFodqRAkpVXp6CQCqBM7p7AoGAIgO0K3oL8t87ma/fTra1
mFWgRJ5qogQ/Qo2VZ11F7ptd4GD7CxPE/cSFLsKOadi7fu75XJ994OhMGrcXSR/g
lEtSFqn6y15UdgU2FtUUX+I72FXo+Nmkqh5xFHDu68d4Kkzdv2xCvn81K3LRsByz
E3P4biQnQ+mN3cIIVu79KNkCgYEAm6uctrEn4y2KLn5DInyj8GuTZ2ELFhVOIzPG
SR7TH451tTJyiblezDHMcOfkWUx0IlN1zCr8jtgiZXmNQzg0erFxWKU7ebZtGGYh
J3g4dLx+2Unt/mzRJqFUgbnueOO/Nr+gbJ+ZdLUCmeeVohOLOTXrws0kYGl2Izab
K1+VrKECgYEAxQohoOegA0f4mofisXItbwwqTIX3bLpxBc4woa1sB4kjNrLo4slc
qtWZGVlRxwBvQUg0cYj+xtr5nyBdHLy0qwX/kMq4GqQnvW6NqsbrP3MjCZ8NX/Sj
A2W0jx50Hs/XNw6IZFLYgWVoOzCaD+jYFpHhzUZyQD6/rYhwhHrNQmU=
-----END RSA PRIVATE KEY-----`

	tmp, _ := pem.Decode([]byte(privPem))
	privKey, _ := x509.ParsePKCS1PrivateKey(tmp.Bytes)

	pubPem := `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAtWp2PZz4TSU5A6ixw41H
dbfBuGJwPuTtrsdoUf0DQ0/DJBNPqOCBAgEu6ZdUqIbWJ5Da+nevjtncy5hENdi6
XrXjyzlUxghMuXjE5SeLGpgfQvkqbTkYaFpMe8PTzNeze3fei8+Eu6mzeb6g1Grq
XznuPIc7bNss0w5iX9RiBM9dWPuXonx9xSEy0LYqJm7yXmshNe1aRwkjG/y5C26B
zBFnMKp9YRTua0DO1WqLNhcaRndalIFYouDNVTbwxSlYL16bZVoebqzZvLGrPvZJ
kPuCu6vH9brvOuYo0q8hLVNkBeXcimRpsDjLhQYzEJjoMTbaiVGnjBky+PWNiofJ
nwIDAQAB
-----END PUBLIC KEY-----`

	err := user_model.SetUserSetting(db.DefaultContext, user.ID, chef_module.SettingPublicPem, pubPem)
	assert.NoError(t, err)

	t.Run("Authenticate", func(t *testing.T) {
		auth := &chef_router.Auth{}

		t.Run("MissingUser", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "POST", "/dummy")
			u, err := auth.Verify(req, nil, nil, nil)
			assert.Nil(t, u)
			assert.NoError(t, err)
		})

		t.Run("NotExistingUser", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "POST", "/dummy")
			req.Header.Set("X-Ops-Userid", "not-existing-user")
			u, err := auth.Verify(req, nil, nil, nil)
			assert.Nil(t, u)
			assert.Error(t, err)
		})

		t.Run("Timestamp", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "POST", "/dummy")
			req.Header.Set("X-Ops-Userid", user.Name)
			u, err := auth.Verify(req, nil, nil, nil)
			assert.Nil(t, u)
			assert.Error(t, err)

			req.Header.Set("X-Ops-Timestamp", "2023-01-01T00:00:00Z")
			u, err = auth.Verify(req, nil, nil, nil)
			assert.Nil(t, u)
			assert.Error(t, err)
		})

		t.Run("SigningVersion", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "POST", "/dummy")
			req.Header.Set("X-Ops-Userid", user.Name)
			req.Header.Set("X-Ops-Timestamp", time.Now().UTC().Format(time.RFC3339))
			u, err := auth.Verify(req, nil, nil, nil)
			assert.Nil(t, u)
			assert.Error(t, err)

			req.Header.Set("X-Ops-Sign", "version=none")
			u, err = auth.Verify(req, nil, nil, nil)
			assert.Nil(t, u)
			assert.Error(t, err)

			req.Header.Set("X-Ops-Sign", "version=1.4")
			u, err = auth.Verify(req, nil, nil, nil)
			assert.Nil(t, u)
			assert.Error(t, err)

			req.Header.Set("X-Ops-Sign", "version=1.0;algorithm=sha2")
			u, err = auth.Verify(req, nil, nil, nil)
			assert.Nil(t, u)
			assert.Error(t, err)

			req.Header.Set("X-Ops-Sign", "version=1.0;algorithm=sha256")
			u, err = auth.Verify(req, nil, nil, nil)
			assert.Nil(t, u)
			assert.Error(t, err)
		})

		t.Run("SignedHeaders", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			ts := time.Now().UTC().Format(time.RFC3339)

			req := NewRequest(t, "POST", "/dummy")
			req.Header.Set("X-Ops-Userid", user.Name)
			req.Header.Set("X-Ops-Timestamp", ts)
			req.Header.Set("X-Ops-Sign", "version=1.0;algorithm=sha1")
			req.Header.Set("X-Ops-Content-Hash", "unused")
			req.Header.Set("X-Ops-Authorization-4", "dummy")
			u, err := auth.Verify(req, nil, nil, nil)
			assert.Nil(t, u)
			assert.Error(t, err)

			signRequest := func(t *testing.T, req *http.Request, version string) {
				username := req.Header.Get("X-Ops-Userid")
				if version != "1.0" && version != "1.3" {
					sum := sha1.Sum([]byte(username))
					username = base64.StdEncoding.EncodeToString(sum[:])
				}

				req.Header.Set("X-Ops-Sign", "version="+version)

				var data []byte
				if version == "1.3" {
					data = []byte(fmt.Sprintf(
						"Method:%s\nPath:%s\nX-Ops-Content-Hash:%s\nX-Ops-Sign:version=%s\nX-Ops-Timestamp:%s\nX-Ops-UserId:%s\nX-Ops-Server-API-Version:%s",
						req.Method,
						path.Clean(req.URL.Path),
						req.Header.Get("X-Ops-Content-Hash"),
						version,
						req.Header.Get("X-Ops-Timestamp"),
						username,
						req.Header.Get("X-Ops-Server-Api-Version"),
					))
				} else {
					sum := sha1.Sum([]byte(path.Clean(req.URL.Path)))
					data = []byte(fmt.Sprintf(
						"Method:%s\nHashed Path:%s\nX-Ops-Content-Hash:%s\nX-Ops-Timestamp:%s\nX-Ops-UserId:%s",
						req.Method,
						base64.StdEncoding.EncodeToString(sum[:]),
						req.Header.Get("X-Ops-Content-Hash"),
						req.Header.Get("X-Ops-Timestamp"),
						username,
					))
				}

				for k := range req.Header {
					if strings.HasPrefix(k, "X-Ops-Authorization-") {
						req.Header.Del(k)
					}
				}

				var signature []byte
				if version == "1.3" || version == "1.2" {
					var h hash.Hash
					var ch crypto.Hash
					if version == "1.3" {
						h = sha256.New()
						ch = crypto.SHA256
					} else {
						h = sha1.New()
						ch = crypto.SHA1
					}
					h.Write(data)

					signature, _ = rsa.SignPKCS1v15(rand.Reader, privKey, ch, h.Sum(nil))
				} else {
					c := new(big.Int).SetBytes(data)
					m := new(big.Int).Exp(c, privKey.D, privKey.N)

					signature = m.Bytes()
				}

				enc := base64.StdEncoding.EncodeToString(signature)

				const chunkSize = 60
				chunks := make([]string, 0, (len(enc)-1)/chunkSize+1)
				currentLen := 0
				currentStart := 0
				for i := range enc {
					if currentLen == chunkSize {
						chunks = append(chunks, enc[currentStart:i])
						currentLen = 0
						currentStart = i
					}
					currentLen++
				}
				chunks = append(chunks, enc[currentStart:])

				for i, chunk := range chunks {
					req.Header.Set(fmt.Sprintf("X-Ops-Authorization-%d", i+1), chunk)
				}
			}

			for _, v := range []string{"1.0", "1.1", "1.2", "1.3"} {
				t.Run(v, func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					signRequest(t, req, v)
					u, err = auth.Verify(req, nil, nil, nil)
					assert.NotNil(t, u)
					assert.NoError(t, err)
				})
			}
		})
	})

	packageName := "test"
	packageVersion := "1.0.1"
	packageDescription := "Test Description"
	packageAuthor := "KN4CK3R"

	root := fmt.Sprintf("/api/packages/%s/chef/api/v1", user.Name)

	uploadPackage := func(t *testing.T, version string, expectedStatus int) {
		var body bytes.Buffer
		mpw := multipart.NewWriter(&body)
		part, _ := mpw.CreateFormFile("tarball", fmt.Sprintf("%s.tar.gz", version))
		zw := gzip.NewWriter(part)
		tw := tar.NewWriter(zw)

		content := `{"name":"` + packageName + `","version":"` + version + `","description":"` + packageDescription + `","maintainer":"` + packageAuthor + `"}`

		hdr := &tar.Header{
			Name: packageName + "/metadata.json",
			Mode: 0o600,
			Size: int64(len(content)),
		}
		tw.WriteHeader(hdr)
		tw.Write([]byte(content))

		tw.Close()
		zw.Close()
		mpw.Close()

		req := NewRequestWithBody(t, "POST", root+"/cookbooks", &body)
		req.Header.Add("Content-Type", mpw.FormDataContentType())
		AddBasicAuthHeader(req, user.Name)
		MakeRequest(t, req, expectedStatus)
	}

	t.Run("Upload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequestWithBody(t, "POST", root+"/cookbooks", bytes.NewReader([]byte{}))
		MakeRequest(t, req, http.StatusUnauthorized)

		uploadPackage(t, packageVersion, http.StatusCreated)

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeChef)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
		assert.NoError(t, err)
		assert.NotNil(t, pd.SemVer)
		assert.IsType(t, &chef_module.Metadata{}, pd.Metadata)
		assert.Equal(t, packageName, pd.Package.Name)
		assert.Equal(t, packageVersion, pd.Version.Version)

		pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
		assert.NoError(t, err)
		assert.Len(t, pfs, 1)
		assert.Equal(t, fmt.Sprintf("%s.tar.gz", packageVersion), pfs[0].Name)
		assert.True(t, pfs[0].IsLead)

		uploadPackage(t, packageVersion, http.StatusConflict)
	})

	t.Run("Download", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/cookbooks/%s/versions/%s/download", root, packageName, packageVersion))
		MakeRequest(t, req, http.StatusOK)
	})

	t.Run("Universe", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", root+"/universe")
		resp := MakeRequest(t, req, http.StatusOK)

		type VersionInfo struct {
			LocationType string            `json:"location_type"`
			LocationPath string            `json:"location_path"`
			DownloadURL  string            `json:"download_url"`
			Dependencies map[string]string `json:"dependencies"`
		}

		var result map[string]map[string]*VersionInfo
		DecodeJSON(t, resp, &result)

		assert.Len(t, result, 1)
		assert.Contains(t, result, packageName)

		versions := result[packageName]

		assert.Len(t, versions, 1)
		assert.Contains(t, versions, packageVersion)

		info := versions[packageVersion]

		assert.Equal(t, "opscode", info.LocationType)
		assert.Equal(t, setting.AppURL+root[1:], info.LocationPath)
		assert.Equal(t, fmt.Sprintf("%s%s/cookbooks/%s/versions/%s/download", setting.AppURL, root[1:], packageName, packageVersion), info.DownloadURL)
	})

	t.Run("Search", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		cases := []struct {
			Query           string
			Start           int
			Items           int
			ExpectedTotal   int
			ExpectedResults int
		}{
			{"", 0, 0, 1, 1},
			{"", 0, 10, 1, 1},
			{"gitea", 0, 10, 0, 0},
			{"test", 0, 10, 1, 1},
			{"test", 1, 10, 1, 0},
		}

		type Item struct {
			CookbookName        string `json:"cookbook_name"`
			CookbookMaintainer  string `json:"cookbook_maintainer"`
			CookbookDescription string `json:"cookbook_description"`
			Cookbook            string `json:"cookbook"`
		}

		type Result struct {
			Start int     `json:"start"`
			Total int     `json:"total"`
			Items []*Item `json:"items"`
		}

		for i, c := range cases {
			req := NewRequest(t, "GET", fmt.Sprintf("%s/search?q=%s&start=%d&items=%d", root, c.Query, c.Start, c.Items))
			req = AddBasicAuthHeader(req, user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			var result Result
			DecodeJSON(t, resp, &result)

			assert.Equal(t, c.ExpectedTotal, result.Total, "case %d: unexpected total hits", i)
			assert.Len(t, result.Items, c.ExpectedResults, "case %d: unexpected result count", i)

			if len(result.Items) == 1 {
				item := result.Items[0]
				assert.Equal(t, packageName, item.CookbookName)
				assert.Equal(t, packageAuthor, item.CookbookMaintainer)
				assert.Equal(t, packageDescription, item.CookbookDescription)
				assert.Equal(t, fmt.Sprintf("%s%s/cookbooks/%s", setting.AppURL, root[1:], packageName), item.Cookbook)
			}
		}
	})

	t.Run("EnumeratePackages", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		cases := []struct {
			Sort            string
			Start           int
			Items           int
			ExpectedTotal   int
			ExpectedResults int
		}{
			{"", 0, 0, 1, 1},
			{"", 0, 10, 1, 1},
			{"RECENTLY_ADDED", 0, 10, 1, 1},
			{"RECENTLY_UPDATED", 0, 10, 1, 1},
			{"", 1, 10, 1, 0},
		}

		type Item struct {
			CookbookName        string `json:"cookbook_name"`
			CookbookMaintainer  string `json:"cookbook_maintainer"`
			CookbookDescription string `json:"cookbook_description"`
			Cookbook            string `json:"cookbook"`
		}

		type Result struct {
			Start int     `json:"start"`
			Total int     `json:"total"`
			Items []*Item `json:"items"`
		}

		for i, c := range cases {
			req := NewRequest(t, "GET", fmt.Sprintf("%s/cookbooks?start=%d&items=%d&sort=%s", root, c.Start, c.Items, c.Sort))
			req = AddBasicAuthHeader(req, user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			var result Result
			DecodeJSON(t, resp, &result)

			assert.Equal(t, c.ExpectedTotal, result.Total, "case %d: unexpected total hits", i)
			assert.Len(t, result.Items, c.ExpectedResults, "case %d: unexpected result count", i)

			if len(result.Items) == 1 {
				item := result.Items[0]
				assert.Equal(t, packageName, item.CookbookName)
				assert.Equal(t, packageAuthor, item.CookbookMaintainer)
				assert.Equal(t, packageDescription, item.CookbookDescription)
				assert.Equal(t, fmt.Sprintf("%s%s/cookbooks/%s", setting.AppURL, root[1:], packageName), item.Cookbook)
			}
		}
	})

	t.Run("PackageMetadata", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/cookbooks/%s", root, packageName))
		resp := MakeRequest(t, req, http.StatusOK)

		type Result struct {
			Name          string    `json:"name"`
			Maintainer    string    `json:"maintainer"`
			Description   string    `json:"description"`
			Category      string    `json:"category"`
			LatestVersion string    `json:"latest_version"`
			SourceURL     string    `json:"source_url"`
			CreatedAt     time.Time `json:"created_at"`
			UpdatedAt     time.Time `json:"updated_at"`
			Deprecated    bool      `json:"deprecated"`
			Versions      []string  `json:"versions"`
		}

		var result Result
		DecodeJSON(t, resp, &result)

		versionURL := fmt.Sprintf("%s%s/cookbooks/%s/versions/%s", setting.AppURL, root[1:], packageName, packageVersion)

		assert.Equal(t, packageName, result.Name)
		assert.Equal(t, packageAuthor, result.Maintainer)
		assert.Equal(t, packageDescription, result.Description)
		assert.Equal(t, versionURL, result.LatestVersion)
		assert.False(t, result.Deprecated)
		assert.ElementsMatch(t, []string{versionURL}, result.Versions)
	})

	t.Run("PackageVersionMetadata", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/cookbooks/%s/versions/%s", root, packageName, packageVersion))
		resp := MakeRequest(t, req, http.StatusOK)

		type Result struct {
			Version         string            `json:"version"`
			TarballFileSize int64             `json:"tarball_file_size"`
			PublishedAt     time.Time         `json:"published_at"`
			Cookbook        string            `json:"cookbook"`
			File            string            `json:"file"`
			License         string            `json:"license"`
			Dependencies    map[string]string `json:"dependencies"`
		}

		var result Result
		DecodeJSON(t, resp, &result)

		packageURL := fmt.Sprintf("%s%s/cookbooks/%s", setting.AppURL, root[1:], packageName)

		assert.Equal(t, packageVersion, result.Version)
		assert.Equal(t, packageURL, result.Cookbook)
		assert.Equal(t, fmt.Sprintf("%s/versions/%s/download", packageURL, packageVersion), result.File)
	})

	t.Run("Delete", func(t *testing.T) {
		uploadPackage(t, "1.0.2", http.StatusCreated)
		uploadPackage(t, "1.0.3", http.StatusCreated)

		t.Run("Version", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "DELETE", fmt.Sprintf("%s/cookbooks/%s/versions/%s", root, packageName, "1.0.2"))
			MakeRequest(t, req, http.StatusUnauthorized)

			req = NewRequest(t, "DELETE", fmt.Sprintf("%s/cookbooks/%s/versions/%s", root, packageName, "1.0.2"))
			AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusOK)

			pv, err := packages.GetVersionByNameAndVersion(db.DefaultContext, user.ID, packages.TypeChef, packageName, "1.0.2")
			assert.Nil(t, pv)
			assert.Error(t, err)
		})

		t.Run("Package", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "DELETE", fmt.Sprintf("%s/cookbooks/%s", root, packageName))
			MakeRequest(t, req, http.StatusUnauthorized)

			req = NewRequest(t, "DELETE", fmt.Sprintf("%s/cookbooks/%s", root, packageName))
			AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusOK)

			pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeChef)
			assert.NoError(t, err)
			assert.Empty(t, pvs)
		})
	})
}
