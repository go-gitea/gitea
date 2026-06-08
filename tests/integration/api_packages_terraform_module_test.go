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

// canonicalModuleArchive returns the .tar.gz used across most subtests.
// Kept as a function so each subtest gets its own buffer (the helpers
// below read it).
func canonicalModuleArchive(t *testing.T) []byte {
	t.Helper()
	src := `
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
	return buildTFModuleArchive(t, map[string]string{
		"main.tf":   src,
		"README.md": "# vpc\n",
	})
}

func TestPackageTerraformModule(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	const (
		name     = "vpc"
		provider = "aws"
		version  = "1.0.0"
	)
	base := fmt.Sprintf("/api/packages/-/terraform/modules/%s/%s/%s", user.Name, name, provider)

	// uploadFixture publishes the canonical module so the subtest can
	// exercise the read path. Registered cleanup deletes it, which keeps
	// every subtest self-contained — running `go test -run .../Download_*`
	// or any individual subtest works without the others having run.
	uploadFixture := func(t *testing.T) {
		t.Helper()
		archive := canonicalModuleArchive(t)
		req := NewRequestWithBody(t, "PUT", base+"/"+version, bytes.NewReader(archive)).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)
		t.Cleanup(func() {
			req := NewRequest(t, "DELETE", base+"/"+version).AddBasicAuth(user.Name)
			// Accept 204 (deleted) or 404 (subtest already deleted it).
			resp := MakeRequest(t, req, http.StatusNoContent)
			_ = resp
		})
	}

	t.Run("ServiceDiscovery", func(t *testing.T) {
		req := NewRequest(t, "GET", "/.well-known/terraform.json")
		resp := MakeRequest(t, req, http.StatusOK)
		var doc map[string]string
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &doc))
		assert.Equal(t, "/api/packages/-/terraform/modules/", doc["modules.v1"])
	})

	t.Run("ListVersions_UnknownPackage", func(t *testing.T) {
		req := NewRequest(t, "GET",
			fmt.Sprintf("/api/packages/-/terraform/modules/%s/does-not-exist/%s/versions", user.Name, provider),
		).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("Upload", func(t *testing.T) {
		uploadFixture(t)
	})

	t.Run("Upload_DuplicateVersion", func(t *testing.T) {
		uploadFixture(t)
		archive := canonicalModuleArchive(t)
		req := NewRequestWithBody(t, "PUT", base+"/"+version, bytes.NewReader(archive)).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusConflict)
	})

	t.Run("Upload_InvalidSemver", func(t *testing.T) {
		archive := canonicalModuleArchive(t)
		req := NewRequestWithBody(t, "PUT", base+"/not-semver", bytes.NewReader(archive)).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusBadRequest)
	})

	t.Run("Upload_InvalidProvider", func(t *testing.T) {
		archive := canonicalModuleArchive(t)
		// Uppercase provider violates the naming rule.
		req := NewRequestWithBody(t, "PUT",
			fmt.Sprintf("/api/packages/-/terraform/modules/%s/%s/AWS/%s", user.Name, name, version),
			bytes.NewReader(archive)).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusBadRequest)
	})

	t.Run("Upload_PathTraversal", func(t *testing.T) {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: "../evil.tf", Typeflag: tar.TypeReg, Mode: 0o644}))
		require.NoError(t, tw.Close())
		require.NoError(t, gz.Close())

		req := NewRequestWithBody(t, "PUT", base+"/2.0.0", bytes.NewReader(buf.Bytes())).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusBadRequest)
	})

	t.Run("ListVersions", func(t *testing.T) {
		uploadFixture(t)
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
		assert.Equal(t, version, body.Modules[0].Versions[0].Version)
	})

	t.Run("Download_XTerraformGet", func(t *testing.T) {
		uploadFixture(t)
		req := NewRequest(t, "GET", base+"/"+version+"/download").AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusNoContent)
		got := resp.Header().Get("X-Terraform-Get")
		require.NotEmpty(t, got)
		assert.True(t, strings.HasSuffix(got,
			fmt.Sprintf("/api/packages/-/terraform/modules/%s/%s/%s/%s/archive", user.Name, name, provider, version)),
			"X-Terraform-Get should point at the archive endpoint, got %q", got)
	})

	t.Run("Download_Archive", func(t *testing.T) {
		uploadFixture(t)
		expected := canonicalModuleArchive(t)
		req := NewRequest(t, "GET", base+"/"+version+"/archive").AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, expected, resp.Body.Bytes())
	})

	t.Run("Download_UnknownVersion", func(t *testing.T) {
		uploadFixture(t)
		req := NewRequest(t, "GET", base+"/9.9.9/download").AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("Delete", func(t *testing.T) {
		// Upload directly (no Cleanup) since this subtest owns the
		// deletion. Skipping uploadFixture also avoids a double-DELETE
		// returning 404 in cleanup.
		archive := canonicalModuleArchive(t)
		req := NewRequestWithBody(t, "PUT", base+"/"+version, bytes.NewReader(archive)).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequest(t, "DELETE", base+"/"+version).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "GET", base+"/"+version+"/download").AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("Anonymous_PublicOwner_ReadAllowed_WriteDenied", func(t *testing.T) {
		// user2 has the default (public) visibility, so an anonymous
		// caller may read but never write.
		uploadFixture(t)

		req := NewRequest(t, "GET", base+"/versions")
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Contains(t, resp.Body.String(), version)

		archive := canonicalModuleArchive(t)
		req = NewRequestWithBody(t, "PUT", base+"/3.0.0", bytes.NewReader(archive))
		MakeRequest(t, req, http.StatusUnauthorized)

		req = NewRequest(t, "DELETE", base+"/"+version)
		MakeRequest(t, req, http.StatusUnauthorized)
	})
}
