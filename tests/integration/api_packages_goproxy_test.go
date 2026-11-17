// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"
	"time"

	"gitea.dev/models/packages"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestPackageGo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	packageName := "gitea.com/go-gitea/gitea"
	packageVersion := "v0.0.1"
	packageVersion2 := "v0.0.2"
	goModContent := `module "gitea.com/go-gitea/gitea"`

	url := fmt.Sprintf("/api/packages/%s/go", user.Name)

	t.Run("Upload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		content := test.WriteZipArchive(nil).Bytes()

		req := NewRequestWithBody(t, "PUT", url+"/upload", bytes.NewReader(content))
		MakeRequest(t, req, http.StatusUnauthorized)

		req = NewRequestWithBody(t, "PUT", url+"/upload", bytes.NewReader(content)).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusBadRequest)

		content = test.WriteZipArchive(map[string]string{
			packageName + "@" + packageVersion + "/go.mod": goModContent,
		}).Bytes()

		req = NewRequestWithBody(t, "PUT", url+"/upload", bytes.NewReader(content)).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)

		pvs, err := packages.GetVersionsByPackageType(t.Context(), user.ID, packages.TypeGo)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(t.Context(), pvs[0])
		assert.NoError(t, err)
		assert.Nil(t, pd.Metadata)
		assert.Equal(t, packageName, pd.Package.Name)
		assert.Equal(t, packageVersion, pd.Version.Version)

		pfs, err := packages.GetFilesByVersionID(t.Context(), pvs[0].ID)
		assert.NoError(t, err)
		assert.Len(t, pfs, 1)
		assert.Equal(t, packageVersion+".zip", pfs[0].Name)
		assert.True(t, pfs[0].IsLead)

		pb, err := packages.GetBlobByID(t.Context(), pfs[0].BlobID)
		assert.NoError(t, err)
		assert.Equal(t, int64(len(content)), pb.Size)

		req = NewRequestWithBody(t, "PUT", url+"/upload", bytes.NewReader(content)).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusConflict)

		time.Sleep(time.Second) // Ensure the timestamp is different, then the "list" below can have stable order

		content = test.WriteZipArchive(map[string]string{
			packageName + "@" + packageVersion2 + "/go.mod": goModContent,
		}).Bytes()

		req = NewRequestWithBody(t, "PUT", url+"/upload", bytes.NewReader(content)).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)
	})

	t.Run("List", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/@v/list", url, packageName))
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, packageVersion+"\n"+packageVersion2+"\n", resp.Body.String())
	})

	t.Run("Info", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		type Info struct {
			Version string    `json:"Version"`
			Time    time.Time `json:"Time"`
		}

		for _, infoRequest := range []struct {
			path, version string
		}{
			{"@v/" + packageVersion + ".info", packageVersion},
			{"@v/latest.info", packageVersion2},
			{"@latest", packageVersion2},
		} {
			req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/%s", url, packageName, infoRequest.path))
			resp := MakeRequest(t, req, http.StatusOK)

			info := DecodeJSON(t, resp, &Info{})
			assert.Equal(t, infoRequest.version, info.Version)
		}
	})

	t.Run("GoMod", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		for _, version := range []string{packageVersion, "latest"} {
			req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/@v/%s.mod", url, packageName, version))
			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, goModContent, resp.Body.String())
		}
	})

	t.Run("Download", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		for _, version := range []string{packageVersion, "latest"} {
			req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/@v/%s.zip", url, packageName, version))
			MakeRequest(t, req, http.StatusOK)
		}
	})
}
