// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestPackageGo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	packageName := "gitea.com/go-gitea/gitea"
	packageVersion := "v0.0.1"
	packageVersion2 := "v0.0.2"
	goModContent := `module "gitea.com/go-gitea/gitea"`

	createArchive := func(files map[string][]byte) []byte {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		for name, content := range files {
			w, _ := zw.Create(name)
			w.Write(content)
		}
		zw.Close()
		return buf.Bytes()
	}

	url := fmt.Sprintf("/api/packages/%s/go", user.Name)

	t.Run("Upload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		content := createArchive(nil)

		req := NewRequestWithBody(t, "PUT", url+"/upload", bytes.NewReader(content))
		MakeRequest(t, req, http.StatusUnauthorized)

		req = NewRequestWithBody(t, "PUT", url+"/upload", bytes.NewReader(content))
		AddBasicAuthHeader(req, user.Name)
		MakeRequest(t, req, http.StatusBadRequest)

		content = createArchive(map[string][]byte{
			packageName + "@" + packageVersion + "/go.mod": []byte(goModContent),
		})

		req = NewRequestWithBody(t, "PUT", url+"/upload", bytes.NewReader(content))
		AddBasicAuthHeader(req, user.Name)
		MakeRequest(t, req, http.StatusCreated)

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeGo)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
		assert.NoError(t, err)
		assert.Nil(t, pd.Metadata)
		assert.Equal(t, packageName, pd.Package.Name)
		assert.Equal(t, packageVersion, pd.Version.Version)

		pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
		assert.NoError(t, err)
		assert.Len(t, pfs, 1)
		assert.Equal(t, packageVersion+".zip", pfs[0].Name)
		assert.True(t, pfs[0].IsLead)

		pb, err := packages.GetBlobByID(db.DefaultContext, pfs[0].BlobID)
		assert.NoError(t, err)
		assert.Equal(t, int64(len(content)), pb.Size)

		req = NewRequestWithBody(t, "PUT", url+"/upload", bytes.NewReader(content))
		AddBasicAuthHeader(req, user.Name)
		MakeRequest(t, req, http.StatusConflict)

		time.Sleep(time.Second)

		content = createArchive(map[string][]byte{
			packageName + "@" + packageVersion2 + "/go.mod": []byte(goModContent),
		})

		req = NewRequestWithBody(t, "PUT", url+"/upload", bytes.NewReader(content))
		AddBasicAuthHeader(req, user.Name)
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

		req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/@v/%s.info", url, packageName, packageVersion))
		resp := MakeRequest(t, req, http.StatusOK)

		type Info struct {
			Version string    `json:"Version"`
			Time    time.Time `json:"Time"`
		}

		info := &Info{}
		DecodeJSON(t, resp, &info)

		assert.Equal(t, packageVersion, info.Version)

		req = NewRequest(t, "GET", fmt.Sprintf("%s/%s/@v/latest.info", url, packageName))
		resp = MakeRequest(t, req, http.StatusOK)

		info = &Info{}
		DecodeJSON(t, resp, &info)

		assert.Equal(t, packageVersion2, info.Version)

		req = NewRequest(t, "GET", fmt.Sprintf("%s/%s/@latest", url, packageName))
		resp = MakeRequest(t, req, http.StatusOK)

		info = &Info{}
		DecodeJSON(t, resp, &info)

		assert.Equal(t, packageVersion2, info.Version)
	})

	t.Run("GoMod", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/@v/%s.mod", url, packageName, packageVersion))
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, goModContent, resp.Body.String())

		req = NewRequest(t, "GET", fmt.Sprintf("%s/%s/@v/latest.mod", url, packageName))
		resp = MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, goModContent, resp.Body.String())
	})

	t.Run("Download", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/@v/%s.zip", url, packageName, packageVersion))
		MakeRequest(t, req, http.StatusOK)

		req = NewRequest(t, "GET", fmt.Sprintf("%s/%s/@v/latest.zip", url, packageName))
		MakeRequest(t, req, http.StatusOK)
	})
}
