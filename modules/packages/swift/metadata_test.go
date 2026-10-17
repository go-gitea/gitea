// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swift

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"

	"gitea.dev/modules/test"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
)

const (
	packageName          = "gitea"
	packageVersion       = "1.0.1"
	packageDescription   = "Package Description"
	packageRepositoryURL = "https://gitea.io/gitea/gitea"
	packageLicenseURL    = "https://opensource.org/license/mit"
	packageAuthor        = "KN4CK3R"
	packageLicense       = "MIT"
)

// writeOrderedZipArchive writes name/content pairs in the given order, which map based test.WriteZipArchive cannot do
func writeOrderedZipArchive(entries [][2]string) *bytes.Buffer {
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for _, entry := range entries {
		w, _ := zw.Create(entry[0])
		_, _ = w.Write([]byte(entry[1]))
	}
	_ = zw.Close()
	return buf
}

func TestParsePackage(t *testing.T) {
	t.Run("MissingManifestFile", func(t *testing.T) {
		data := test.WriteZipArchive(map[string]string{"dummy.txt": ""})
		p, err := ParsePackage(bytes.NewReader(data.Bytes()), int64(data.Len()), nil)
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrMissingManifestFile)
	})

	t.Run("ManifestFileTooLarge", func(t *testing.T) {
		data := test.WriteZipArchive(map[string]string{
			"Package.swift": strings.Repeat("a", maxManifestFileSize+1),
		})
		p, err := ParsePackage(bytes.NewReader(data.Bytes()), int64(data.Len()), nil)
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrManifestFileTooLarge)
	})

	t.Run("TooManyManifestFiles", func(t *testing.T) {
		entries := make([][2]string, 0, maxManifestFiles+1)
		entries = append(entries, [2]string{"Package.swift", "// swift-tools-version:5.7"})
		for i := range maxManifestFiles {
			entries = append(entries, [2]string{fmt.Sprintf("Package@swift-5.%d.swift", i), "// swift-tools-version:5.7"})
		}

		data := writeOrderedZipArchive(entries)
		p, err := ParsePackage(bytes.NewReader(data.Bytes()), int64(data.Len()), nil)
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrManifestFilesTooLarge)
	})

	t.Run("WithoutMetadata", func(t *testing.T) {
		content1 := "// swift-tools-version:5.7\n//\n//  Package.swift"
		content2 := "// swift-tools-version:5.6\n//\n//  Package@swift-5.6.swift"

		data := test.WriteZipArchive(map[string]string{
			"Package.swift":           content1,
			"Package@swift-5.5.swift": content2,
		})

		p, err := ParsePackage(bytes.NewReader(data.Bytes()), int64(data.Len()), nil)
		assert.NotNil(t, p)
		assert.NoError(t, err)

		assert.NotNil(t, p.Metadata)
		assert.Empty(t, p.RepositoryURLs)
		assert.Len(t, p.Metadata.Manifests, 2)
		m := p.Metadata.Manifests[""]
		assert.Equal(t, "5.7", m.ToolsVersion)
		assert.Equal(t, content1, m.Content)
		m = p.Metadata.Manifests["5.5"]
		assert.Equal(t, "5.6", m.ToolsVersion)
		assert.Equal(t, content2, m.Content)
	})

	t.Run("IgnoresNestedManifests", func(t *testing.T) {
		rootManifest := "// swift-tools-version:5.7\n//\n//  Package.swift"
		rootAltManifest := "// swift-tools-version:5.5\n//\n//  Package@swift-5.5.swift"
		rootPatchAltManifest := "// swift-tools-version:5.7.1\n//\n//  Package@swift-5.7.1.swift"
		nestedManifest := "// swift-tools-version:6.3\n//\n//  nested fixture package"

		data := writeOrderedZipArchive([][2]string{
			{"Package.swift", rootManifest},
			{"Package@swift-5.5.swift", rootAltManifest},
			{"Package@swift-5.7.1.swift", rootPatchAltManifest},
			{"Benchmarks/Package.swift", nestedManifest},
			{"Utils/Fixtures/PlainPackage/Package.swift", nestedManifest},
		})

		p, err := ParsePackage(bytes.NewReader(data.Bytes()), int64(data.Len()), nil)
		assert.NotNil(t, p)
		assert.NoError(t, err)

		assert.Len(t, p.Metadata.Manifests, 3)
		assert.Equal(t, rootManifest, p.Metadata.Manifests[""].Content)
		assert.Equal(t, "5.7", p.Metadata.Manifests[""].ToolsVersion)
		assert.Equal(t, rootAltManifest, p.Metadata.Manifests["5.5"].Content)
		assert.Equal(t, rootPatchAltManifest, p.Metadata.Manifests["5.7.1"].Content)
	})

	t.Run("IgnoresNestedManifestsInPrefixedArchive", func(t *testing.T) {
		rootManifest := "// swift-tools-version:5.7\n//\n//  Package.swift"

		// `swift package archive-source` produces archives with a single top level directory
		data := writeOrderedZipArchive([][2]string{
			{"gitea-1.0.1/Package.swift", rootManifest},
			{"gitea-1.0.1/Tests/Fixtures/Package.swift", "// swift-tools-version:6.3"},
		})

		p, err := ParsePackage(bytes.NewReader(data.Bytes()), int64(data.Len()), nil)
		assert.NotNil(t, p)
		assert.NoError(t, err)

		assert.Len(t, p.Metadata.Manifests, 1)
		assert.Equal(t, rootManifest, p.Metadata.Manifests[""].Content)
	})

	t.Run("AltManifestOnlyInRootDirectory", func(t *testing.T) {
		// a deeper Package.swift belongs to a nested package and must not stand in for the missing root manifest
		data := test.WriteZipArchive(map[string]string{
			"Package@swift-5.5.swift": "// swift-tools-version:5.5",
			"Sub/Package.swift":       "// swift-tools-version:5.7",
		})

		p, err := ParsePackage(bytes.NewReader(data.Bytes()), int64(data.Len()), nil)
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrMissingManifestFile)
	})

	t.Run("ManifestDirectoryTieBreak", func(t *testing.T) {
		contentA := "// swift-tools-version:5.7\n// A"
		contentB := "// swift-tools-version:5.7\n// B"

		// at equal depth the name decides, never the archive order
		data := writeOrderedZipArchive([][2]string{
			{"a/Package.swift", contentA},
			{"b/Package.swift", contentB},
		})

		p, err := ParsePackage(bytes.NewReader(data.Bytes()), int64(data.Len()), nil)
		assert.NotNil(t, p)
		assert.NoError(t, err)
		assert.Len(t, p.Metadata.Manifests, 1)
		assert.Equal(t, contentA, p.Metadata.Manifests[""].Content)
	})

	t.Run("WithMetadata", func(t *testing.T) {
		data := test.WriteZipArchive(map[string]string{
			"Package.swift": "// swift-tools-version:5.7\n//\n//  Package.swift",
		})

		p, err := ParsePackage(
			bytes.NewReader(data.Bytes()), int64(data.Len()),
			strings.NewReader(`{"name":"`+packageName+`","version":"`+packageVersion+`","description":"`+packageDescription+`","keywords":["swift","package"],"license":"`+packageLicense+`","licenseURL":"`+packageLicenseURL+`","codeRepository":"`+packageRepositoryURL+`","author":{"givenName":"`+packageAuthor+`"},"repositoryURLs":["`+packageRepositoryURL+`"]}`),
		)
		assert.NotNil(t, p)
		assert.NoError(t, err)

		assert.NotNil(t, p.Metadata)
		assert.Len(t, p.Metadata.Manifests, 1)
		m := p.Metadata.Manifests[""]
		assert.Equal(t, "5.7", m.ToolsVersion)

		assert.Equal(t, packageDescription, p.Metadata.Description)
		assert.ElementsMatch(t, []string{"swift", "package"}, p.Metadata.Keywords)
		assert.Equal(t, packageLicense, p.Metadata.License)
		assert.Equal(t, packageLicenseURL, p.Metadata.LicenseURL)
		assert.Equal(t, packageAuthor, p.Metadata.Author.Name)
		assert.Equal(t, packageAuthor, p.Metadata.Author.GivenName)
		assert.Equal(t, packageRepositoryURL, p.Metadata.RepositoryURL)
		assert.ElementsMatch(t, []string{packageRepositoryURL}, p.RepositoryURLs)
	})

	t.Run("WithExplicitNameField", func(t *testing.T) {
		data := test.WriteZipArchive(map[string]string{
			"Package.swift": "// swift-tools-version:5.7\n//\n//  Package.swift",
		})

		authorName := "John Doe"
		p, err := ParsePackage(
			bytes.NewReader(data.Bytes()), int64(data.Len()),
			strings.NewReader(`{"name":"`+packageName+`","version":"`+packageVersion+`","description":"`+packageDescription+`","author":{"name":"`+authorName+`","givenName":"John","familyName":"Doe"}}`),
		)
		assert.NotNil(t, p)
		assert.NoError(t, err)

		assert.Equal(t, authorName, p.Metadata.Author.Name)
		assert.Equal(t, "John", p.Metadata.Author.GivenName)
		assert.Equal(t, "Doe", p.Metadata.Author.FamilyName)
	})

	t.Run("WithEmptyJSONMetadata", func(t *testing.T) {
		data := test.WriteZipArchive(map[string]string{
			"Package.swift": "// swift-tools-version:5.7\n//\n//  Package.swift",
		})

		p, err := ParsePackage(
			bytes.NewReader(data.Bytes()), int64(data.Len()),
			strings.NewReader(`{}`),
		)
		assert.NotNil(t, p)
		assert.NoError(t, err)
		assert.NotNil(t, p.Metadata)
		assert.Empty(t, p.Metadata.Author.Name)
		assert.Empty(t, p.RepositoryURLs)
	})

	t.Run("NameFieldGeneration", func(t *testing.T) {
		data := test.WriteZipArchive(map[string]string{
			"Package.swift": "// swift-tools-version:5.7\n//\n//  Package.swift",
		})

		// Test with only individual name components - Name should be auto-generated
		p, err := ParsePackage(
			bytes.NewReader(data.Bytes()), int64(data.Len()),
			strings.NewReader(`{"author":{"givenName":"John","middleName":"Q","familyName":"Doe"}}`),
		)
		assert.NotNil(t, p)
		assert.NoError(t, err)
		assert.Equal(t, "John Q Doe", p.Metadata.Author.Name)
		assert.Equal(t, "John", p.Metadata.Author.GivenName)
		assert.Equal(t, "Q", p.Metadata.Author.MiddleName)
		assert.Equal(t, "Doe", p.Metadata.Author.FamilyName)
	})
}

func TestTrimmedVersionString(t *testing.T) {
	cases := []struct {
		Version  *version.Version
		Expected string
	}{
		{
			Version:  version.Must(version.NewVersion("1")),
			Expected: "1.0",
		},
		{
			Version:  version.Must(version.NewVersion("1.0")),
			Expected: "1.0",
		},
		{
			Version:  version.Must(version.NewVersion("1.0.0")),
			Expected: "1.0",
		},
		{
			Version:  version.Must(version.NewVersion("1.0.1")),
			Expected: "1.0.1",
		},
		{
			Version:  version.Must(version.NewVersion("1.0+meta")),
			Expected: "1.0",
		},
		{
			Version:  version.Must(version.NewVersion("1.0.0+meta")),
			Expected: "1.0",
		},
		{
			Version:  version.Must(version.NewVersion("1.0.1+meta")),
			Expected: "1.0.1",
		},
	}

	for _, c := range cases {
		assert.Equal(t, c.Expected, TrimmedVersionString(c.Version))
	}
}

func TestPersonNameString(t *testing.T) {
	cases := []struct {
		Name     string
		Person   Person
		Expected string
	}{
		{
			Name:     "GivenNameOnly",
			Person:   Person{GivenName: "John"},
			Expected: "John",
		},
		{
			Name:     "GivenAndFamily",
			Person:   Person{GivenName: "John", FamilyName: "Doe"},
			Expected: "John Doe",
		},
		{
			Name:     "FullName",
			Person:   Person{GivenName: "John", MiddleName: "Q", FamilyName: "Doe"},
			Expected: "John Q Doe",
		},
		{
			Name:     "MiddleAndFamily",
			Person:   Person{MiddleName: "Q", FamilyName: "Doe"},
			Expected: "Q Doe",
		},
		{
			Name:     "Empty",
			Person:   Person{},
			Expected: "",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			assert.Equal(t, c.Expected, c.Person.String())
		})
	}
}
