// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"mime/multipart"
	"net/http"
	"testing"

	"gitea.dev/models/packages"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/packages/rubygems"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

type tarFile struct {
	Name string
	Data []byte
}

func makeArchiveFileTar(files []*tarFile) []byte {
	buf := new(bytes.Buffer)
	tarWriter := tar.NewWriter(buf)
	for _, file := range files {
		_ = tarWriter.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     file.Name,
			Mode:     0o644,
			Size:     int64(len(file.Data)),
		})
		_, _ = tarWriter.Write(file.Data)
	}
	_ = tarWriter.Close()
	return buf.Bytes()
}

func makeArchiveFileGz(data []byte) []byte {
	buf := new(bytes.Buffer)
	gzWriter, _ := gzip.NewWriterLevel(buf, gzip.NoCompression)
	_, _ = gzWriter.Write(data)
	_ = gzWriter.Close()
	return buf.Bytes()
}

func makeRubyGem(name, version string) []byte {
	metadataContent := fmt.Sprintf(`--- !ruby/object:Gem::Specification
name: %s
version: !ruby/object:Gem::Version
  version: %s
platform: ruby
authors:
- Gitea
autorequire:
bindir: bin
cert_chain: []
date: 2021-08-23 00:00:00.000000000 Z
dependencies:
- !ruby/object:Gem::Dependency
  name: runtime-dep
  requirement: !ruby/object:Gem::Requirement
    requirements:
    - - ">="
      - !ruby/object:Gem::Version
        version: 1.2.0
    - - "<"
      - !ruby/object:Gem::Version
        version: '2.0'
  type: :runtime
  prerelease: false
  version_requirements: !ruby/object:Gem::Requirement
    requirements:
    - - ">="
      - !ruby/object:Gem::Version
        version: 1.2.0
    - - "<"
      - !ruby/object:Gem::Version
        version: '2.0'
- !ruby/object:Gem::Dependency
  name: dev-dep
  requirement: !ruby/object:Gem::Requirement
    requirements:
    - - "~>"
      - !ruby/object:Gem::Version
        version: '5.2'
  type: :development
  prerelease: false
  version_requirements: !ruby/object:Gem::Requirement
    requirements:
    - - "~>"
      - !ruby/object:Gem::Version
        version: '5.2'
description: RubyGems package test
email: rubygems@gitea.io
executables: []
extensions: []
extra_rdoc_files: []
files:
- lib/gitea.rb
homepage: https://gitea.io/
licenses:
- MIT
metadata: {}
post_install_message:
rdoc_options: []
require_paths:
- lib
required_ruby_version: !ruby/object:Gem::Requirement
  requirements:
  - - ">="
    - !ruby/object:Gem::Version
      version: 2.3.0
required_rubygems_version: !ruby/object:Gem::Requirement
  requirements:
  - - ">="
    - !ruby/object:Gem::Version
      version: '1.0'
requirements: []
rubyforge_project:
rubygems_version: 2.7.6.2
signing_key:
specification_version: 4
summary: Gitea package
test_files: []
`, name, version)

	metadataGz := makeArchiveFileGz([]byte(metadataContent))
	dataTarGz := makeArchiveFileGz(makeArchiveFileTar([]*tarFile{
		{
			Name: "lib/gitea.rb",
			Data: []byte("class Gitea\nend"),
		},
	}))

	checksumsYaml := fmt.Sprintf(`---
SHA256:
  metadata.gz: %x
  data.tar.gz: %x
SHA512:
  metadata.gz: %x
  data.tar.gz: %x
`, sha256.Sum256(metadataGz), sha256.Sum256(dataTarGz), sha512.Sum512(metadataGz), sha512.Sum512(dataTarGz))

	files := []*tarFile{
		{
			Name: "data.tar.gz",
			Data: dataTarGz,
		},
		{
			Name: "metadata.gz",
			Data: metadataGz,
		},
		{
			Name: "checksums.yaml.gz",
			Data: makeArchiveFileGz([]byte(checksumsYaml)),
		},
	}
	return makeArchiveFileTar(files)
}

func TestPackageRubyGems(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	testGemName := "gitea"
	testGemVersion := "1.0.5"
	testGemContent := makeRubyGem(testGemName, testGemVersion)
	testGemContentChecksum := fmt.Sprintf("%x", sha256.Sum256(testGemContent))

	testAnotherGemName := "gitea-another"
	testAnotherGemVersion := "0.99"

	root := fmt.Sprintf("/api/packages/%s/rubygems", user.Name)

	uploadFile := func(t *testing.T, content []byte, expectedStatus int) {
		req := NewRequestWithBody(t, "POST", root+"/api/v1/gems", bytes.NewReader(content)).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, expectedStatus)
	}

	t.Run("Upload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		uploadFile(t, testGemContent, http.StatusCreated)

		pvs, err := packages.GetVersionsByPackageType(t.Context(), user.ID, packages.TypeRubyGems)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(t.Context(), pvs[0])
		assert.NoError(t, err)
		assert.NotNil(t, pd.SemVer)
		assert.IsType(t, &rubygems.Metadata{}, pd.Metadata)
		assert.Equal(t, testGemName, pd.Package.Name)
		assert.Equal(t, testGemVersion, pd.Version.Version)

		pfs, err := packages.GetFilesByVersionID(t.Context(), pvs[0].ID)
		assert.NoError(t, err)
		assert.Len(t, pfs, 1)
		assert.Equal(t, fmt.Sprintf("%s-%s.gem", testGemName, testGemVersion), pfs[0].Name)
		assert.True(t, pfs[0].IsLead)

		pb, err := packages.GetBlobByID(t.Context(), pfs[0].BlobID)
		assert.NoError(t, err)
		assert.EqualValues(t, len(testGemContent), pb.Size)
	})

	t.Run("UploadExists", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		uploadFile(t, testGemContent, http.StatusConflict)
	})

	t.Run("Download", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/gems/%s-%s.gem", root, testGemName, testGemVersion)).
			AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, testGemContent, resp.Body.Bytes())

		pvs, err := packages.GetVersionsByPackageType(t.Context(), user.ID, packages.TypeRubyGems)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)
		assert.Equal(t, int64(1), pvs[0].DownloadCount)
	})

	t.Run("DownloadGemspec", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/quick/Marshal.4.8/%s-%s.gemspec.rz", root, testGemName, testGemVersion)).
			AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		b, _ := base64.StdEncoding.DecodeString(`eJxi4Si1EndPzbWyCi5ITc5My0xOLMnMz2M8zMIRLeGpxGWsZ6RnzGbF5hqSyempxJWeWZKayGbN
EBJqJQjWFZZaVJyZnxfN5qnEZahnoGcKkjTwVBJyB6lUKEhMzk5MTwULGngqcRaVJlWCONEMBp5K
DGAWSKc7zFhPJamg0qRK99TcYphehZLU4hKQSk8lsYySkoJiK319sDP0MvP1QeIhSOYagBzC4esZ
wmbNEAIYADEtOhs=`)
		assert.Equal(t, b, resp.Body.Bytes())

		pvs, err := packages.GetVersionsByPackageType(t.Context(), user.ID, packages.TypeRubyGems)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)
		assert.Equal(t, int64(1), pvs[0].DownloadCount)
	})

	t.Run("EnumeratePackages", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		enumeratePackages := func(t *testing.T, endpoint string, expectedContent []byte) {
			req := NewRequest(t, "GET", fmt.Sprintf("%s/%s", root, endpoint)).
				AddBasicAuth(user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, expectedContent, resp.Body.Bytes())
		}

		b, _ := base64.StdEncoding.DecodeString(`H4sICAAAAAAA/3NwZWNzLjQuOAAAOwDE/wQIWwZbCEkiCmdpdGVhBjoGRVRVOhFHZW06OlZlcnNpb25bBkkiCjEuMC41BjsAVEkiCXJ1YnkGOwBUAwDxOEYKOwAAAA==`)
		enumeratePackages(t, "specs.4.8.gz", b)
		b, _ = base64.StdEncoding.DecodeString(`H4sICAAAAAAA/2xhdGVzdF9zcGVjcy40LjgAADsAxP8ECFsGWwhJIgpnaXRlYQY6BkVUVToRR2VtOjpWZXJzaW9uWwZJIgoxLjAuNQY7AFRJIglydWJ5BjsAVAMA8ThGCjsAAAA=`)
		enumeratePackages(t, "latest_specs.4.8.gz", b)
		b, _ = base64.StdEncoding.DecodeString(`H4sICAAAAAAA/3ByZXJlbGVhc2Vfc3BlY3MuNC44AAAEAPv/BAhbAAMAbJ16+QQAAAA=`)
		enumeratePackages(t, "prerelease_specs.4.8.gz", b)
	})

	t.Run("UploadAnother", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		uploadFile(t, makeRubyGem(testAnotherGemName, testAnotherGemVersion), http.StatusCreated)
	})

	t.Run("PackageInfo", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/info/%s", root, testGemName)).AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)
		expected := fmt.Sprintf(`---
1.0.5 runtime-dep:>= 1.2.0&< 2.0|checksum:%s,ruby:>= 2.3.0,rubygems:>= 1.0
`, testGemContentChecksum)
		assert.Equal(t, expected, resp.Body.String())
	})

	t.Run("Versions", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", root+"/versions").AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, `---
gitea 1.0.5 6dc029f6875c637b6e2236c5056f431d
gitea-another 0.99 3f0251d551c97733836b9c983366c36d
`, resp.Body.String())
	})

	deleteGemPackage := func(t *testing.T, packageName, packageVersion string) {
		body := bytes.Buffer{}
		writer := multipart.NewWriter(&body)
		_ = writer.WriteField("gem_name", packageName)
		_ = writer.WriteField("version", packageVersion)
		_ = writer.Close()
		req := NewRequestWithBody(t, "DELETE", root+"/api/v1/gems/yank", &body).
			SetHeader("Content-Type", writer.FormDataContentType()).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusOK)
	}

	t.Run("DeleteAll", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		deleteGemPackage(t, testGemName, testGemVersion)
		deleteGemPackage(t, testAnotherGemName, testAnotherGemVersion)
		pvs, err := packages.GetVersionsByPackageType(t.Context(), user.ID, packages.TypeRubyGems)
		assert.NoError(t, err)
		assert.Empty(t, pvs)
	})

	t.Run("PackageInfoAfterDelete", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", fmt.Sprintf("%s/info/%s", root, testGemName)).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("VersionsAfterDelete", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", root+"/versions").AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, "---\n", resp.Body.String())
	})
}
