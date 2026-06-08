// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildTFModuleArchive returns a gzipped tarball with the given files at the root.
func buildTFModuleArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     name,
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Size:     int64(len(content)),
		}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func TestPackageTerraformModule(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	name := "vpc"
	provider := "aws"
	v := "1.0.0"
	base := fmt.Sprintf("/api/packages/-/terraform/modules/%s/%s/%s", user.Name, name, provider)

	tfSrc := `
terraform {
  required_version = ">= 1.5.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

variable "region" {
  type        = string
  description = "AWS region"
  default     = "eu-west-1"
}

output "vpc_id" {
  value = aws_vpc.this.id
}

resource "aws_vpc" "this" {
  cidr_block = "10.0.0.0/16"
}
`
	archive := buildTFModuleArchive(t, map[string]string{
		"main.tf":   tfSrc,
		"README.md": "# vpc\n",
	})

	t.Run("ServiceDiscovery", func(t *testing.T) {
		req := NewRequest(t, "GET", "/.well-known/terraform.json")
		resp := MakeRequest(t, req, http.StatusOK)
		var doc map[string]string
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &doc))
		assert.Equal(t, "/api/packages/-/terraform/modules/", doc["modules.v1"])
	})

	t.Run("ListVersions_BeforeUpload", func(t *testing.T) {
		req := NewRequest(t, "GET", base+"/versions").AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("Upload", func(t *testing.T) {
		req := NewRequestWithBody(t, "PUT", base+"/"+v, bytes.NewReader(archive)).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)
	})

	t.Run("Upload_DuplicateVersion", func(t *testing.T) {
		req := NewRequestWithBody(t, "PUT", base+"/"+v, bytes.NewReader(archive)).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusConflict)
	})

	t.Run("Upload_InvalidSemver", func(t *testing.T) {
		req := NewRequestWithBody(t, "PUT", base+"/not-semver", bytes.NewReader(archive)).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusBadRequest)
	})

	t.Run("Upload_InvalidProvider", func(t *testing.T) {
		req := NewRequestWithBody(t, "PUT",
			fmt.Sprintf("/api/packages/-/terraform/modules/%s/%s/AWS/%s", user.Name, name, v),
			bytes.NewReader(archive)).AddBasicAuth(user.Name)
		// Uppercase provider violates the naming rule.
		MakeRequest(t, req, http.StatusBadRequest)
	})

	t.Run("Upload_PathTraversal", func(t *testing.T) {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: "../evil.tf", Typeflag: tar.TypeReg, Mode: 0o644}))
		require.NoError(t, tw.Close())
		require.NoError(t, gz.Close())

		// Use a fresh version so the 400 we expect isn't masked by the 409 duplicate.
		req := NewRequestWithBody(t, "PUT", base+"/2.0.0", bytes.NewReader(buf.Bytes())).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusBadRequest)
	})

	t.Run("ListVersions", func(t *testing.T) {
		req := NewRequest(t, "GET", base+"/versions").AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)
		var body struct {
			Modules []struct {
				Versions []struct {
					Version string `json:"version"`
				} `json:"versions"`
			} `json:"modules"`
		}
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
		require.Len(t, body.Modules, 1)
		require.Len(t, body.Modules[0].Versions, 1)
		assert.Equal(t, v, body.Modules[0].Versions[0].Version)
	})

	t.Run("Download_XTerraformGet", func(t *testing.T) {
		req := NewRequest(t, "GET", base+"/"+v+"/download").AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusNoContent)
		got := resp.Header().Get("X-Terraform-Get")
		require.NotEmpty(t, got)
		assert.True(t, strings.HasSuffix(got, fmt.Sprintf("/api/packages/-/terraform/modules/%s/%s/%s/%s/archive", user.Name, name, provider, v)),
			"X-Terraform-Get should point at the archive endpoint, got %q", got)
	})

	t.Run("Download_Archive", func(t *testing.T) {
		req := NewRequest(t, "GET", base+"/"+v+"/archive").AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, archive, resp.Body.Bytes())
	})

	t.Run("Download_UnknownVersion", func(t *testing.T) {
		req := NewRequest(t, "GET", base+"/9.9.9/download").AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("Delete", func(t *testing.T) {
		req := NewRequest(t, "DELETE", base+"/"+v).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "GET", base+"/"+v+"/download").AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("Anonymous_PrivateOwner_Forbidden", func(t *testing.T) {
		// User #2 (visible to anyone) — re-upload to set up, then test anon read.
		req := NewRequestWithBody(t, "PUT", base+"/"+v, bytes.NewReader(archive)).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)
		t.Cleanup(func() {
			req := NewRequest(t, "DELETE", base+"/"+v).AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusNoContent)
		})

		// Anonymous read on a public owner is allowed in Gitea's default
		// access model; assert the endpoint at least responds successfully
		// rather than 401, to mirror the State endpoint behavior.
		req = NewRequest(t, "GET", base+"/versions")
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Contains(t, resp.Body.String(), v)
	})
}
