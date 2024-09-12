// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	conda_module "code.gitea.io/gitea/modules/packages/conda"
	"code.gitea.io/gitea/modules/zstd"
	"code.gitea.io/gitea/tests"

	"github.com/dsnet/compress/bzip2"
	"github.com/stretchr/testify/assert"
)

func TestPackageConda(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	packageName := "test_package"
	packageVersion := "1.0.1"

	channel := "test-channel"
	root := fmt.Sprintf("/api/packages/%s/conda", user.Name)

	t.Run("Upload", func(t *testing.T) {
		tarContent := func() []byte {
			var buf bytes.Buffer
			tw := tar.NewWriter(&buf)

			content := []byte(`{"name":"` + packageName + `","version":"` + packageVersion + `","subdir":"noarch","build":"xxx"}`)

			hdr := &tar.Header{
				Name: "info/index.json",
				Mode: 0o600,
				Size: int64(len(content)),
			}
			tw.WriteHeader(hdr)
			tw.Write(content)
			tw.Close()
			return buf.Bytes()
		}()

		t.Run(".tar.bz2", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			var buf bytes.Buffer
			bw, _ := bzip2.NewWriter(&buf, nil)
			io.Copy(bw, bytes.NewReader(tarContent))
			bw.Close()

			filename := fmt.Sprintf("%s-%s.tar.bz2", packageName, packageVersion)

			req := NewRequestWithBody(t, "PUT", root+"/"+filename, bytes.NewReader(buf.Bytes()))
			MakeRequest(t, req, http.StatusUnauthorized)

			req = NewRequestWithBody(t, "PUT", root+"/"+filename, bytes.NewReader(buf.Bytes())).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusCreated)

			req = NewRequestWithBody(t, "PUT", root+"/"+filename, bytes.NewReader(buf.Bytes())).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusConflict)

			pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeConda)
			assert.NoError(t, err)
			assert.Len(t, pvs, 1)

			pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
			assert.NoError(t, err)
			assert.Nil(t, pd.SemVer)
			assert.IsType(t, &conda_module.VersionMetadata{}, pd.Metadata)
			assert.Equal(t, packageName, pd.Package.Name)
			assert.Equal(t, packageVersion, pd.Version.Version)
			assert.Empty(t, pd.PackageProperties.GetByName(conda_module.PropertyChannel))
		})

		t.Run(".conda", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			var infoBuf bytes.Buffer
			zsw, _ := zstd.NewWriter(&infoBuf)
			io.Copy(zsw, bytes.NewReader(tarContent))
			zsw.Close()

			var buf bytes.Buffer
			zpw := zip.NewWriter(&buf)
			w, _ := zpw.Create("info-x.tar.zst")
			w.Write(infoBuf.Bytes())
			zpw.Close()

			fullName := channel + "/" + packageName
			filename := fmt.Sprintf("%s-%s.conda", packageName, packageVersion)

			req := NewRequestWithBody(t, "PUT", root+"/"+channel+"/"+filename, bytes.NewReader(buf.Bytes()))
			MakeRequest(t, req, http.StatusUnauthorized)

			req = NewRequestWithBody(t, "PUT", root+"/"+channel+"/"+filename, bytes.NewReader(buf.Bytes())).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusCreated)

			req = NewRequestWithBody(t, "PUT", root+"/"+channel+"/"+filename, bytes.NewReader(buf.Bytes())).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusConflict)

			pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeConda)
			assert.NoError(t, err)
			assert.Len(t, pvs, 2)

			pds, err := packages.GetPackageDescriptors(db.DefaultContext, pvs)
			assert.NoError(t, err)

			assert.Condition(t, func() bool {
				for _, pd := range pds {
					if pd.Package.Name == fullName {
						return true
					}
				}
				return false
			})

			for _, pd := range pds {
				if pd.Package.Name == fullName {
					assert.Nil(t, pd.SemVer)
					assert.IsType(t, &conda_module.VersionMetadata{}, pd.Metadata)
					assert.Equal(t, fullName, pd.Package.Name)
					assert.Equal(t, packageVersion, pd.Version.Version)
					assert.Equal(t, channel, pd.PackageProperties.GetByName(conda_module.PropertyChannel))
				}
			}
		})
	})

	t.Run("Download", func(t *testing.T) {
		t.Run(".tar.bz2", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/noarch/%s-%s-xxx.tar.bz2", root, packageName, packageVersion))
			MakeRequest(t, req, http.StatusOK)

			req = NewRequest(t, "GET", fmt.Sprintf("%s/%s/noarch/%s-%s-xxx.tar.bz2", root, channel, packageName, packageVersion))
			MakeRequest(t, req, http.StatusNotFound)
		})

		t.Run(".conda", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/noarch/%s-%s-xxx.conda", root, packageName, packageVersion))
			MakeRequest(t, req, http.StatusNotFound)

			req = NewRequest(t, "GET", fmt.Sprintf("%s/%s/noarch/%s-%s-xxx.conda", root, channel, packageName, packageVersion))
			MakeRequest(t, req, http.StatusOK)
		})
	})

	t.Run("EnumeratePackages", func(t *testing.T) {
		type Info struct {
			Subdir string `json:"subdir"`
		}

		type PackageInfo struct {
			Name          string   `json:"name"`
			Version       string   `json:"version"`
			NoArch        string   `json:"noarch"`
			Subdir        string   `json:"subdir"`
			Timestamp     int64    `json:"timestamp"`
			Build         string   `json:"build"`
			BuildNumber   int64    `json:"build_number"`
			Dependencies  []string `json:"depends"`
			License       string   `json:"license"`
			LicenseFamily string   `json:"license_family"`
			HashMD5       string   `json:"md5"`
			HashSHA256    string   `json:"sha256"`
			Size          int64    `json:"size"`
		}

		type RepoData struct {
			Info          Info                    `json:"info"`
			Packages      map[string]*PackageInfo `json:"packages"`
			PackagesConda map[string]*PackageInfo `json:"packages.conda"`
			Removed       map[string]*PackageInfo `json:"removed"`
		}

		req := NewRequest(t, "GET", fmt.Sprintf("%s/noarch/repodata.json", root))
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, "application/json", resp.Header().Get("Content-Type"))

		req = NewRequest(t, "GET", fmt.Sprintf("%s/noarch/repodata.json.bz2", root))
		resp = MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, "application/x-bzip2", resp.Header().Get("Content-Type"))

		req = NewRequest(t, "GET", fmt.Sprintf("%s/noarch/current_repodata.json", root))
		resp = MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, "application/json", resp.Header().Get("Content-Type"))

		req = NewRequest(t, "GET", fmt.Sprintf("%s/noarch/current_repodata.json.bz2", root))
		resp = MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, "application/x-bzip2", resp.Header().Get("Content-Type"))

		t.Run(".tar.bz2", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			pv, err := packages.GetVersionByNameAndVersion(db.DefaultContext, user.ID, packages.TypeConda, packageName, packageVersion)
			assert.NoError(t, err)

			pd, err := packages.GetPackageDescriptor(db.DefaultContext, pv)
			assert.NoError(t, err)

			req := NewRequest(t, "GET", fmt.Sprintf("%s/noarch/repodata.json", root))
			resp := MakeRequest(t, req, http.StatusOK)

			var result RepoData
			DecodeJSON(t, resp, &result)

			assert.Equal(t, "noarch", result.Info.Subdir)
			assert.Empty(t, result.PackagesConda)
			assert.Empty(t, result.Removed)

			filename := fmt.Sprintf("%s-%s-xxx.tar.bz2", packageName, packageVersion)
			assert.Contains(t, result.Packages, filename)
			packageInfo := result.Packages[filename]
			assert.Equal(t, packageName, packageInfo.Name)
			assert.Equal(t, packageVersion, packageInfo.Version)
			assert.Equal(t, "noarch", packageInfo.Subdir)
			assert.Equal(t, "xxx", packageInfo.Build)
			assert.Equal(t, pd.Files[0].Blob.HashMD5, packageInfo.HashMD5)
			assert.Equal(t, pd.Files[0].Blob.HashSHA256, packageInfo.HashSHA256)
			assert.Equal(t, pd.Files[0].Blob.Size, packageInfo.Size)
		})

		t.Run(".conda", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			pv, err := packages.GetVersionByNameAndVersion(db.DefaultContext, user.ID, packages.TypeConda, channel+"/"+packageName, packageVersion)
			assert.NoError(t, err)

			pd, err := packages.GetPackageDescriptor(db.DefaultContext, pv)
			assert.NoError(t, err)

			req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/noarch/repodata.json", root, channel))
			resp := MakeRequest(t, req, http.StatusOK)

			var result RepoData
			DecodeJSON(t, resp, &result)

			assert.Equal(t, "noarch", result.Info.Subdir)
			assert.Empty(t, result.Packages)
			assert.Empty(t, result.Removed)

			filename := fmt.Sprintf("%s-%s-xxx.conda", packageName, packageVersion)
			assert.Contains(t, result.PackagesConda, filename)
			packageInfo := result.PackagesConda[filename]
			assert.Equal(t, packageName, packageInfo.Name)
			assert.Equal(t, packageVersion, packageInfo.Version)
			assert.Equal(t, "noarch", packageInfo.Subdir)
			assert.Equal(t, "xxx", packageInfo.Build)
			assert.Equal(t, pd.Files[0].Blob.HashMD5, packageInfo.HashMD5)
			assert.Equal(t, pd.Files[0].Blob.HashSHA256, packageInfo.HashSHA256)
			assert.Equal(t, pd.Files[0].Blob.Size, packageInfo.Size)
		})
	})
}
