// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swift

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
)

const (
	packageName          = "gitea"
	packageVersion       = "1.0.1"
	packageDescription   = "Package Description"
	packageRepositoryURL = "https://gitea.io/gitea/gitea"
	packageAuthor        = "KN4CK3R"
	packageLicense       = "MIT"
)

func TestParsePackage(t *testing.T) {
	createArchive := func(files map[string][]byte) *bytes.Reader {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		for filename, content := range files {
			w, _ := zw.Create(filename)
			w.Write(content)
		}
		zw.Close()
		return bytes.NewReader(buf.Bytes())
	}

	t.Run("MissingManifestFile", func(t *testing.T) {
		data := createArchive(map[string][]byte{"dummy.txt": {}})

		p, err := ParsePackage(data, data.Size(), nil)
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrMissingManifestFile)
	})

	t.Run("ManifestFileTooLarge", func(t *testing.T) {
		data := createArchive(map[string][]byte{
			"Package.swift": make([]byte, maxManifestFileSize+1),
		})

		p, err := ParsePackage(data, data.Size(), nil)
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrManifestFileTooLarge)
	})

	t.Run("WithoutMetadata", func(t *testing.T) {
		content1 := "// swift-tools-version:5.7\n//\n//  Package.swift"
		content2 := "// swift-tools-version:5.6\n//\n//  Package@swift-5.6.swift"

		data := createArchive(map[string][]byte{
			"Package.swift":           []byte(content1),
			"Package@swift-5.5.swift": []byte(content2),
		})

		p, err := ParsePackage(data, data.Size(), nil)
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

	t.Run("WithMetadata", func(t *testing.T) {
		data := createArchive(map[string][]byte{
			"Package.swift": []byte("// swift-tools-version:5.7\n//\n//  Package.swift"),
		})

		p, err := ParsePackage(
			data,
			data.Size(),
			strings.NewReader(`{"name":"`+packageName+`","version":"`+packageVersion+`","description":"`+packageDescription+`","keywords":["swift","package"],"license":"`+packageLicense+`","codeRepository":"`+packageRepositoryURL+`","author":{"givenName":"`+packageAuthor+`"},"repositoryURLs":["`+packageRepositoryURL+`"]}`),
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
		assert.Equal(t, packageAuthor, p.Metadata.Author.Name)
		assert.Equal(t, packageAuthor, p.Metadata.Author.GivenName)
		assert.Equal(t, packageRepositoryURL, p.Metadata.RepositoryURL)
		assert.ElementsMatch(t, []string{packageRepositoryURL}, p.RepositoryURLs)
	})

	t.Run("WithExplicitNameField", func(t *testing.T) {
		data := createArchive(map[string][]byte{
			"Package.swift": []byte("// swift-tools-version:5.7\n//\n//  Package.swift"),
		})

		authorName := "John Doe"
		p, err := ParsePackage(
			data,
			data.Size(),
			strings.NewReader(`{"name":"`+packageName+`","version":"`+packageVersion+`","description":"`+packageDescription+`","author":{"name":"`+authorName+`","givenName":"John","familyName":"Doe"}}`),
		)
		assert.NotNil(t, p)
		assert.NoError(t, err)

		assert.Equal(t, authorName, p.Metadata.Author.Name)
		assert.Equal(t, "John", p.Metadata.Author.GivenName)
		assert.Equal(t, "Doe", p.Metadata.Author.FamilyName)
	})

	t.Run("NameFieldGeneration", func(t *testing.T) {
		data := createArchive(map[string][]byte{
			"Package.swift": []byte("// swift-tools-version:5.7\n//\n//  Package.swift"),
		})

		// Test with only individual name components - Name should be auto-generated
		p, err := ParsePackage(
			data,
			data.Size(),
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
