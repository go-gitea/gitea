// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package npm

import (
	"bytes"
	"compress/gzip"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"testing"

	"gitea.dev/modules/json"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		Type:      "gitea",
		URL:       "http://localhost:3000/gitea/test.git",
		Directory: "packages/test-package",
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
		assert.Equal(t, "MIT", string(p.Metadata.License))
		assert.Equal(t, "https://gitea.io/", p.Metadata.ProjectURL)
		assert.Contains(t, p.Metadata.Dependencies, "package")
		assert.Equal(t, "1.2.0", p.Metadata.Dependencies["package"])
		assert.Equal(t, repository.Type, p.Metadata.Repository.Type)
		assert.Equal(t, repository.URL, p.Metadata.Repository.URL)
		assert.Equal(t, repository.Directory, p.Metadata.Repository.Directory)
	})

	t.Run("ValidLicenseMap", func(t *testing.T) {
		packageJSON := `{
  "versions": {
		"0.1.1": {
			"name": "dev-null",
			"version": "0.1.1",
			"license": {
				"type": "MIT"
			},
			"dist": {
				"integrity": "sha256-"
			}
		}
	},
	"_attachments": {
		"foo": {
			"data": "AAAA"
		}
	}
}`
		p, err := ParsePackage(strings.NewReader(packageJSON))
		require.NoError(t, err)
		require.Equal(t, "MIT", string(p.Metadata.License))
	})

	t.Run("ValidRepositoryAndBinAsString", func(t *testing.T) {
		// npm allows "repository" and "bin" to be plain strings, not only objects.
		packageJSON := `{
  "versions": {
		"0.1.1": {
			"name": "dev-null",
			"version": "0.1.1",
			"bin": "./cli.js",
			"repository": "https://gitea.io/gitea/test.git",
			"dist": {
				"integrity": "sha256-"
			}
		}
	},
	"_attachments": {
		"foo": {
			"data": "AAAA"
		}
	}
}`
		p, err := ParsePackage(strings.NewReader(packageJSON))
		require.NoError(t, err)
		require.Equal(t, "https://gitea.io/gitea/test.git", p.Metadata.Repository.URL)
		// a string bin is named after the package
		require.Equal(t, "./cli.js", p.Metadata.Bin["dev-null"])
	})
}

// buildTarball assembles a gzipped tar with the given entries.
func buildTarball(files map[string]string) []byte {
	return test.WriteTarCompression(func(w io.Writer) io.WriteCloser { return gzip.NewWriter(w) }, files).Bytes()
}

func TestInspectTarball(t *testing.T) {
	cases := []struct {
		name                          string
		files                         map[string]string
		wantShrinkwrap, wantInstaller bool
	}{
		{
			name:  "empty",
			files: map[string]string{},
		},
		{
			name:           "shrinkwrap only",
			files:          map[string]string{"package/npm-shrinkwrap.json": "{}"},
			wantShrinkwrap: true,
		},
		{
			name:          "postinstall only",
			files:         map[string]string{"package/package.json": `{"scripts":{"postinstall":"echo hi"}}`},
			wantInstaller: true,
		},
		{
			name:          "preinstall",
			files:         map[string]string{"package/package.json": `{"scripts":{"preinstall":"noop"}}`},
			wantInstaller: true,
		},
		{
			name:          "install",
			files:         map[string]string{"package/package.json": `{"scripts":{"install":"noop"}}`},
			wantInstaller: true,
		},
		{
			name:  "whitespace-only script does not count",
			files: map[string]string{"package/package.json": `{"scripts":{"postinstall":"   "}}`},
		},
		{
			name:  "unrelated lifecycle script ignored",
			files: map[string]string{"package/package.json": `{"scripts":{"test":"jest"}}`},
		},
		{
			name: "both",
			files: map[string]string{
				"package/npm-shrinkwrap.json": "{}",
				"package/package.json":        `{"scripts":{"install":"go"}}`,
			},
			wantShrinkwrap: true,
			wantInstaller:  true,
		},
		{
			name: "nested shrinkwrap ignored",
			files: map[string]string{
				"package/subdir/npm-shrinkwrap.json": "{}",
			},
		},
		{
			name:  "leading ./ prefix stripped",
			files: map[string]string{"./package/npm-shrinkwrap.json": "{}"},
			// npm pack sometimes emits "./package/..." entries.
			wantShrinkwrap: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := buildTarball(c.files)
			gotShrink, gotInstaller := inspectTarball(data)
			assert.Equal(t, c.wantShrinkwrap, gotShrink, "shrinkwrap")
			assert.Equal(t, c.wantInstaller, gotInstaller, "installScript")
		})
	}

	t.Run("malformed gzip returns false", func(t *testing.T) {
		hasShrinkwrap, hasInstaller := inspectTarball([]byte("not a gzip"))
		assert.False(t, hasShrinkwrap)
		assert.False(t, hasInstaller)
	})

	t.Run("malformed package.json falls through", func(t *testing.T) {
		data := buildTarball(map[string]string{"package/package.json": "{ this is not json"})
		_, hasInstaller := inspectTarball(data)
		assert.False(t, hasInstaller)
	})
}

func TestParsePackageDeprecation(t *testing.T) {
	pkg := "@scope/test-package"
	t.Run("InvalidName", func(t *testing.T) {
		body := `{"name":"","versions":{"1.0.0":{"deprecated":"msg"}}}`
		_, err := ParsePackageDeprecation(strings.NewReader(body))
		assert.ErrorIs(t, err, ErrInvalidPackageName)
	})

	t.Run("PublishRejected", func(t *testing.T) {
		body := fmt.Sprintf(`{"name":%q,"versions":{"1.0.0":{"name":%q,"version":"1.0.0","deprecated":"msg"}},"_attachments":{"x.tgz":{"data":"AAAA"}}}`, pkg, pkg)
		_, err := ParsePackageDeprecation(strings.NewReader(body))
		assert.ErrorIs(t, err, ErrInvalidPackage)
	})

	t.Run("ExplicitKeyPresent", func(t *testing.T) {
		body := fmt.Sprintf(`{"name":%q,"versions":{"1.0.0":{"deprecated":"gone"},"1.0.1":{"deprecated":""}}}`, pkg)
		dep, err := ParsePackageDeprecation(strings.NewReader(body))
		require.NoError(t, err)
		assert.Equal(t, pkg, dep.PackageName)
		assert.Equal(t, "gone", dep.Versions["1.0.0"])
		gotEmpty, hasEmpty := dep.Versions["1.0.1"]
		assert.True(t, hasEmpty, "empty-string deprecated key should be preserved")
		assert.Empty(t, gotEmpty)
	})

	t.Run("AbsentKeySkipped", func(t *testing.T) {
		body := fmt.Sprintf(`{"name":%q,"versions":{"1.0.0":{"deprecated":"gone"},"1.0.1":{"version":"1.0.1"}}}`, pkg)
		dep, err := ParsePackageDeprecation(strings.NewReader(body))
		require.NoError(t, err)
		assert.Contains(t, dep.Versions, "1.0.0")
		assert.NotContains(t, dep.Versions, "1.0.1", "absent deprecated key must not surface as empty string")
	})

	t.Run("MixedShapes", func(t *testing.T) {
		body := fmt.Sprintf(`{"name":%q,"versions":{"1.0.0":{"deprecated":"msg"},"1.0.1":{},"1.0.2":{"deprecated":""},"1.0.3":null}}`, pkg)
		dep, err := ParsePackageDeprecation(strings.NewReader(body))
		require.NoError(t, err)
		assert.Equal(t, "msg", dep.Versions["1.0.0"])
		assert.NotContains(t, dep.Versions, "1.0.1")
		_, has102 := dep.Versions["1.0.2"]
		assert.True(t, has102)
		assert.NotContains(t, dep.Versions, "1.0.3")
	})
}

func TestParseUpload(t *testing.T) {
	pkg := "@scope/test-package"

	t.Run("dispatches deprecate on missing _attachments", func(t *testing.T) {
		body := fmt.Sprintf(`{"name":%q,"versions":{"1.0.0":{"deprecated":"gone"}}}`, pkg)
		p, dep, err := ParseUpload(strings.NewReader(body))
		require.NoError(t, err)
		assert.Nil(t, p)
		require.NotNil(t, dep)
		assert.Equal(t, "gone", dep.Versions["1.0.0"])
	})

	t.Run("dispatches publish when _attachments present", func(t *testing.T) {
		// Reuse a minimal tarball with a package.json.
		data := buildTarball(map[string]string{"package/package.json": `{}`})
		integrity := "sha512-" + base64Sha512(t, data)
		body := fmt.Sprintf(
			`{"name":%q,"versions":{"1.0.0":{"name":%q,"version":"1.0.0","dist":{"integrity":%q}}},"_attachments":{"x.tgz":{"data":%q}}}`,
			pkg, pkg, integrity, base64.StdEncoding.EncodeToString(data),
		)
		p, dep, err := ParseUpload(strings.NewReader(body))
		require.NoError(t, err)
		assert.Nil(t, dep)
		require.NotNil(t, p)
		assert.Equal(t, pkg, p.Name)
	})

	t.Run("publish whose readme mentions deprecated is not misrouted", func(t *testing.T) {
		// The old fast-path used a substring check for "deprecated"; make sure
		// the new dispatch keys off _attachments only.
		data := buildTarball(map[string]string{"package/package.json": `{}`})
		integrity := "sha512-" + base64Sha512(t, data)
		body := fmt.Sprintf(
			`{"name":%q,"versions":{"1.0.0":{"name":%q,"version":"1.0.0","readme":"this package is deprecated!","dist":{"integrity":%q}}},"_attachments":{"x.tgz":{"data":%q}}}`,
			pkg, pkg, integrity, base64.StdEncoding.EncodeToString(data),
		)
		p, dep, err := ParseUpload(strings.NewReader(body))
		require.NoError(t, err)
		assert.Nil(t, dep)
		require.NotNil(t, p)
	})

	t.Run("invalid json errors out", func(t *testing.T) {
		_, _, err := ParseUpload(strings.NewReader("not json"))
		assert.Error(t, err)
	})
}

func base64Sha512(t *testing.T, data []byte) string {
	t.Helper()
	h := sha512.Sum512(data)
	return base64.StdEncoding.EncodeToString(h[:])
}
