// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package npm

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/assert"
)

func TestParsePackage(t *testing.T) {
	packageScope := "@scope"
	packageName := "test-package"
	packageFullName := packageScope + "/" + packageName
	packageVersion := "1.0.1-pre"
	packageTag := "latest"
	packageAuthor := "KN4CK3R"
	packageBin := "gitea"
	packageDescription := "Test Description"
	data := "H4sIAAAAAAAA/ytITM5OTE/VL4DQelnF+XkMVAYGBgZmJiYK2MRBwNDcSIHB2NTMwNDQzMwAqA7IMDUxA9LUdgg2UFpcklgEdAql5kD8ogCnhwio5lJQUMpLzE1VslJQcihOzi9I1S9JLS7RhSYIJR2QgrLUouLM/DyQGkM9Az1D3YIiqExKanFyUWZBCVQ2BKhVwQVJDKwosbQkI78IJO/tZ+LsbRykxFXLNdA+HwWjYBSMgpENACgAbtAACAAA"
	integrity := "sha512-yA4FJsVhetynGfOC1jFf79BuS+jrHbm0fhh+aHzCQkOaOBXKf9oBnC4a6DnLLnEsHQDRLYd00cwj8sCXpC+wIg=="
	repository := Repository{
		Type: "gitea",
		URL:  "http://localhost:3000/gitea/test.git",
	}

	t.Run("InvalidUpload", func(t *testing.T) {
		p, err := ParsePackage(bytes.NewReader([]byte{0}))
		assert.Nil(t, p)
		assert.Error(t, err)
	})

	t.Run("InvalidUploadNoData", func(t *testing.T) {
		b, _ := json.Marshal(packageUpload{})
		p, err := ParsePackage(bytes.NewReader(b))
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrInvalidPackage)
	})

	t.Run("InvalidPackageName", func(t *testing.T) {
		test := func(t *testing.T, name string) {
			b, _ := json.Marshal(packageUpload{
				PackageMetadata: PackageMetadata{
					ID:   name,
					Name: name,
					Versions: map[string]*PackageMetadataVersion{
						packageVersion: {
							Name: name,
						},
					},
				},
			})

			p, err := ParsePackage(bytes.NewReader(b))
			assert.Nil(t, p)
			assert.ErrorIs(t, err, ErrInvalidPackageName)
		}

		test(t, " test ")
		test(t, " test")
		test(t, "test ")
		test(t, "te st")
		test(t, "Test")
		test(t, "_test")
		test(t, ".test")
		test(t, "^test")
		test(t, "te^st")
		test(t, "te|st")
		test(t, "te)(st")
		test(t, "te'st")
		test(t, "te!st")
		test(t, "te*st")
		test(t, "te~st")
		test(t, "invalid/scope")
		test(t, "@invalid/_name")
		test(t, "@invalid/.name")
	})

	t.Run("ValidPackageName", func(t *testing.T) {
		test := func(t *testing.T, name string) {
			b, _ := json.Marshal(packageUpload{
				PackageMetadata: PackageMetadata{
					ID:   name,
					Name: name,
					Versions: map[string]*PackageMetadataVersion{
						packageVersion: {
							Name: name,
						},
					},
				},
			})

			p, err := ParsePackage(bytes.NewReader(b))
			assert.Nil(t, p)
			assert.ErrorIs(t, err, ErrInvalidPackageVersion)
		}

		test(t, "test")
		test(t, "@scope/name")
		test(t, "@scope/q")
		test(t, "q")
		test(t, "@scope/package-name")
		test(t, "@scope/package.name")
		test(t, "@scope/package_name")
		test(t, "123name")
		test(t, "----")
		test(t, packageFullName)
	})

	t.Run("InvalidPackageVersion", func(t *testing.T) {
		version := "first-version"
		b, _ := json.Marshal(packageUpload{
			PackageMetadata: PackageMetadata{
				ID:   packageFullName,
				Name: packageFullName,
				Versions: map[string]*PackageMetadataVersion{
					version: {
						Name:    packageFullName,
						Version: version,
					},
				},
			},
		})

		p, err := ParsePackage(bytes.NewReader(b))
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrInvalidPackageVersion)
	})

	t.Run("InvalidAttachment", func(t *testing.T) {
		b, _ := json.Marshal(packageUpload{
			PackageMetadata: PackageMetadata{
				ID:   packageFullName,
				Name: packageFullName,
				Versions: map[string]*PackageMetadataVersion{
					packageVersion: {
						Name:    packageFullName,
						Version: packageVersion,
					},
				},
			},
			Attachments: map[string]*PackageAttachment{
				"dummy.tgz": {},
			},
		})

		p, err := ParsePackage(bytes.NewReader(b))
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrInvalidAttachment)
	})

	t.Run("InvalidData", func(t *testing.T) {
		filename := fmt.Sprintf("%s-%s.tgz", packageFullName, packageVersion)
		b, _ := json.Marshal(packageUpload{
			PackageMetadata: PackageMetadata{
				ID:   packageFullName,
				Name: packageFullName,
				Versions: map[string]*PackageMetadataVersion{
					packageVersion: {
						Name:    packageFullName,
						Version: packageVersion,
					},
				},
			},
			Attachments: map[string]*PackageAttachment{
				filename: {
					Data: "/",
				},
			},
		})

		p, err := ParsePackage(bytes.NewReader(b))
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrInvalidAttachment)
	})

	t.Run("InvalidIntegrity", func(t *testing.T) {
		filename := fmt.Sprintf("%s-%s.tgz", packageFullName, packageVersion)
		b, _ := json.Marshal(packageUpload{
			PackageMetadata: PackageMetadata{
				ID:   packageFullName,
				Name: packageFullName,
				Versions: map[string]*PackageMetadataVersion{
					packageVersion: {
						Name:    packageFullName,
						Version: packageVersion,
						Dist: PackageDistribution{
							Integrity: "sha512-test==",
						},
					},
				},
			},
			Attachments: map[string]*PackageAttachment{
				filename: {
					Data: data,
				},
			},
		})

		p, err := ParsePackage(bytes.NewReader(b))
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrInvalidIntegrity)
	})

	t.Run("InvalidIntegrity2", func(t *testing.T) {
		filename := fmt.Sprintf("%s-%s.tgz", packageFullName, packageVersion)
		b, _ := json.Marshal(packageUpload{
			PackageMetadata: PackageMetadata{
				ID:   packageFullName,
				Name: packageFullName,
				Versions: map[string]*PackageMetadataVersion{
					packageVersion: {
						Name:    packageFullName,
						Version: packageVersion,
						Dist: PackageDistribution{
							Integrity: integrity,
						},
					},
				},
			},
			Attachments: map[string]*PackageAttachment{
				filename: {
					Data: base64.StdEncoding.EncodeToString([]byte("data")),
				},
			},
		})

		p, err := ParsePackage(bytes.NewReader(b))
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrInvalidIntegrity)
	})

	t.Run("Valid", func(t *testing.T) {
		filename := fmt.Sprintf("%s-%s.tgz", packageFullName, packageVersion)
		b, _ := json.Marshal(packageUpload{
			PackageMetadata: PackageMetadata{
				ID:   packageFullName,
				Name: packageFullName,
				DistTags: map[string]string{
					packageTag: packageVersion,
				},
				Versions: map[string]*PackageMetadataVersion{
					packageVersion: {
						Name:        packageFullName,
						Version:     packageVersion,
						Description: packageDescription,
						Author:      User{Name: packageAuthor},
						License:     "MIT",
						Homepage:    "https://gitea.io/",
						Readme:      packageDescription,
						Dependencies: map[string]string{
							"package": "1.2.0",
						},
						Bin: map[string]string{
							"bin": packageBin,
						},
						Dist: PackageDistribution{
							Integrity: integrity,
						},
						Repository: repository,
					},
				},
			},
			Attachments: map[string]*PackageAttachment{
				filename: {
					Data: data,
				},
			},
		})

		p, err := ParsePackage(bytes.NewReader(b))
		assert.NotNil(t, p)
		assert.NoError(t, err)

		assert.Equal(t, packageFullName, p.Name)
		assert.Equal(t, packageVersion, p.Version)
		assert.Equal(t, []string{packageTag}, p.DistTags)
		assert.Equal(t, fmt.Sprintf("%s-%s.tgz", strings.Split(packageFullName, "/")[1], packageVersion), p.Filename)
		b, _ = base64.StdEncoding.DecodeString(data)
		assert.Equal(t, b, p.Data)
		assert.Equal(t, packageName, p.Metadata.Name)
		assert.Equal(t, packageScope, p.Metadata.Scope)
		assert.Equal(t, packageDescription, p.Metadata.Description)
		assert.Equal(t, packageDescription, p.Metadata.Readme)
		assert.Equal(t, packageAuthor, p.Metadata.Author)
		assert.Equal(t, packageBin, p.Metadata.Bin["bin"])
		assert.Equal(t, "MIT", p.Metadata.License)
		assert.Equal(t, "https://gitea.io/", p.Metadata.ProjectURL)
		assert.Contains(t, p.Metadata.Dependencies, "package")
		assert.Equal(t, "1.2.0", p.Metadata.Dependencies["package"])
		assert.Equal(t, repository.Type, p.Metadata.Repository.Type)
		assert.Equal(t, repository.URL, p.Metadata.Repository.URL)
	})
}
