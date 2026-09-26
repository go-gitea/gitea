// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageTerraformModule(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	privateOrg := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "privated_org"})
	root := "/api/packages/-/terraform/modules/" + privateOrg.Name
	base := root + "/vpc/aws"
	archive := test.WriteTarCompression(gzip.NewWriter, map[string]string{"main.tf": `variable "region" {}`, "README.md": "# VPC module"}).Bytes()

	upload := func(t *testing.T, uploadURL string, body []byte, status int) {
		MakeRequest(t, NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader(body)).AddBasicAuth(admin.Name), status)
	}

	t.Run("ServiceDiscovery", func(t *testing.T) {
		resp := MakeRequest(t, NewRequest(t, "GET", "/.well-known/terraform.json"), http.StatusOK)
		assert.JSONEq(t, `{"modules.v1":"/api/packages/-/terraform/modules/"}`, resp.Body.String())

		defer test.MockVariableValue(&setting.Packages.Enabled, false)()
		MakeRequest(t, NewRequest(t, "GET", "/.well-known/terraform.json"), http.StatusForbidden)
	})

	t.Run("Upload", func(t *testing.T) {
		upload(t, base+"/v1.0.0", archive, http.StatusCreated)
		upload(t, base+"/1.0.0", archive, http.StatusConflict)
		upload(t, base+"/not-semver", archive, http.StatusBadRequest)
		upload(t, root+"/vpc/my-cloud/1.0.0", archive, http.StatusBadRequest)
		upload(t, base+"/2.0.0", test.WriteTarCompression(gzip.NewWriter, map[string]string{"vpc-2.0.0/main.tf": `variable "region" {}`}).Bytes(), http.StatusBadRequest)
		MakeRequest(t, NewRequestWithBody(t, "PUT", base+"/2.0.0", bytes.NewReader(archive)), http.StatusUnauthorized)
	})

	t.Run("View", func(t *testing.T) {
		resp := loginUser(t, admin.Name).MakeRequest(t, NewRequest(t, "GET", "/"+privateOrg.Name+"/-/packages/terraform-module/vpc%2Faws/1.0.0"), http.StatusOK)
		content := NewHTMLParser(t, resp.Body).Find(".packages-content-left").Text()
		appURL, _ := url.Parse(setting.AppURL)
		assert.Contains(t, content, `source  = "`+appURL.Host+"/"+privateOrg.Name+`/vpc/aws"`)
		assert.Contains(t, content, "VPC module")
		assert.Contains(t, content, "region")
	})

	t.Run("ListVersions", func(t *testing.T) {
		resp := MakeRequest(t, NewRequest(t, "GET", base+"/versions").AddBasicAuth(admin.Name), http.StatusOK)
		assert.JSONEq(t, `{"modules":[{"versions":[{"version":"1.0.0"}]}]}`, resp.Body.String())
		MakeRequest(t, NewRequest(t, "GET", base+"/versions"), http.StatusUnauthorized)
		MakeRequest(t, NewRequest(t, "GET", root+"/unknown/aws/versions").AddBasicAuth(admin.Name), http.StatusNotFound)
	})

	t.Run("Download", func(t *testing.T) {
		downloadURL := base + "/1.0.0/download"
		resp := MakeRequest(t, NewRequest(t, "GET", downloadURL).AddBasicAuth(admin.Name), http.StatusNoContent)
		location, err := url.Parse(resp.Header().Get("X-Terraform-Get"))
		require.NoError(t, err)
		assert.Equal(t, "tar.gz", location.Query().Get("archive"))
		archiveURL := (&url.URL{Path: downloadURL}).ResolveReference(location).String()

		resp = MakeRequest(t, NewRequest(t, "GET", archiveURL), http.StatusOK)
		assert.Equal(t, archive, resp.Body.Bytes())
		MakeRequest(t, NewRequest(t, "GET", strings.Replace(archiveURL, "sig=", "sig=x", 1)), http.StatusUnauthorized)
		MakeRequest(t, NewRequest(t, "GET", base+"/1.0.0/archive").AddBasicAuth(admin.Name), http.StatusOK)
		MakeRequest(t, NewRequest(t, "GET", base+"/9.9.9/download").AddBasicAuth(admin.Name), http.StatusNotFound)
	})

	t.Run("Delete", func(t *testing.T) {
		MakeRequest(t, NewRequest(t, "DELETE", base+"/1.0.0"), http.StatusUnauthorized)
		MakeRequest(t, NewRequest(t, "DELETE", base+"/v1.0.0").AddBasicAuth(admin.Name), http.StatusNoContent)
		MakeRequest(t, NewRequest(t, "GET", base+"/versions").AddBasicAuth(admin.Name), http.StatusNotFound)
	})
}
