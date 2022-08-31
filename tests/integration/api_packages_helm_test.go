// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	helm_module "code.gitea.io/gitea/modules/packages/helm"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func TestPackageHelm(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	packageName := "test-chart"
	packageVersion := "1.0.3"
	packageAuthor := "KN4CK3R"
	packageDescription := "Gitea Test Package"

	filename := fmt.Sprintf("%s-%s.tgz", packageName, packageVersion)

	chartContent := `apiVersion: v2
description: ` + packageDescription + `
name: ` + packageName + `
type: application
version: ` + packageVersion + `
maintainers:
- name: ` + packageAuthor + `
dependencies:
- name: dep1
  repository: https://example.com/
  version: 1.0.0`

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	archive := tar.NewWriter(zw)
	archive.WriteHeader(&tar.Header{
		Name: fmt.Sprintf("%s/Chart.yaml", packageName),
		Mode: 0o600,
		Size: int64(len(chartContent)),
	})
	archive.Write([]byte(chartContent))
	archive.Close()
	zw.Close()
	content := buf.Bytes()

	url := fmt.Sprintf("/api/packages/%s/helm", user.Name)

	t.Run("Upload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		uploadURL := url + "/api/charts"

		req := NewRequestWithBody(t, "POST", uploadURL, bytes.NewReader(content))
		req = AddBasicAuthHeader(req, user.Name)
		MakeRequest(t, req, http.StatusCreated)

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeHelm)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
		assert.NoError(t, err)
		assert.NotNil(t, pd.SemVer)
		assert.IsType(t, &helm_module.Metadata{}, pd.Metadata)
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

		req = NewRequestWithBody(t, "POST", uploadURL, bytes.NewReader(content))
		req = AddBasicAuthHeader(req, user.Name)
		MakeRequest(t, req, http.StatusCreated)
	})

	t.Run("Download", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		checkDownloadCount := func(count int64) {
			pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeHelm)
			assert.NoError(t, err)
			assert.Len(t, pvs, 1)
			assert.Equal(t, count, pvs[0].DownloadCount)
		}

		checkDownloadCount(0)

		req := NewRequest(t, "GET", fmt.Sprintf("%s/%s", url, filename))
		req = AddBasicAuthHeader(req, user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, content, resp.Body.Bytes())

		checkDownloadCount(1)
	})

	t.Run("Index", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/index.yaml", url))
		req = AddBasicAuthHeader(req, user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		type ChartVersion struct {
			helm_module.Metadata `yaml:",inline"`
			URLs                 []string  `yaml:"urls"`
			Created              time.Time `yaml:"created,omitempty"`
			Removed              bool      `yaml:"removed,omitempty"`
			Digest               string    `yaml:"digest,omitempty"`
		}

		type ServerInfo struct {
			ContextPath string `yaml:"contextPath,omitempty"`
		}

		type Index struct {
			APIVersion string                     `yaml:"apiVersion"`
			Entries    map[string][]*ChartVersion `yaml:"entries"`
			Generated  time.Time                  `yaml:"generated,omitempty"`
			ServerInfo *ServerInfo                `yaml:"serverInfo,omitempty"`
		}

		var result Index
		assert.NoError(t, yaml.NewDecoder(resp.Body).Decode(&result))
		assert.NotEmpty(t, result.Entries)
		assert.Contains(t, result.Entries, packageName)

		cvs := result.Entries[packageName]
		assert.Len(t, cvs, 1)

		cv := cvs[0]
		assert.Equal(t, packageName, cv.Name)
		assert.Equal(t, packageVersion, cv.Version)
		assert.Equal(t, packageDescription, cv.Description)
		assert.Len(t, cv.Maintainers, 1)
		assert.Equal(t, packageAuthor, cv.Maintainers[0].Name)
		assert.Len(t, cv.Dependencies, 1)
		assert.ElementsMatch(t, []string{fmt.Sprintf("%s%s/%s", setting.AppURL, url[1:], filename)}, cv.URLs)

		assert.Equal(t, url, result.ServerInfo.ContextPath)
	})
}
