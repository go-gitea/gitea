// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swift

import (
	"archive/zip"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/validation"

	"github.com/hashicorp/go-version"
)

var (
	ErrMissingManifestFile    = util.NewInvalidArgumentErrorf("Package.swift file is missing")
	ErrManifestFileTooLarge   = util.NewInvalidArgumentErrorf("Package.swift file is too large")
	ErrInvalidManifestVersion = util.NewInvalidArgumentErrorf("manifest version is invalid")

	manifestPattern     = regexp.MustCompile(`\APackage(?:@swift-(\d+(?:\.\d+)?(?:\.\d+)?))?\.swift\z`)
	toolsVersionPattern = regexp.MustCompile(`\A// swift-tools-version:(\d+(?:\.\d+)?(?:\.\d+)?)`)
)

const (
	maxManifestFileSize = 128 * 1024

	PropertyScope         = "swift.scope"
	PropertyName          = "swift.name"
	PropertyRepositoryURL = "swift.repository_url"
)

// Package represents a Swift package
type Package struct {
	RepositoryURLs []string
	Metadata       *Metadata
}

// Metadata represents the metadata of a Swift package
type Metadata struct {
	Description   string               `json:"description,omitempty"`
	Keywords      []string             `json:"keywords,omitempty"`
	RepositoryURL string               `json:"repository_url,omitempty"`
	License       string               `json:"license,omitempty"`
	Author        Person               `json:"author"`
	Manifests     map[string]*Manifest `json:"manifests,omitempty"`
}

// Manifest represents a Package.swift file
type Manifest struct {
	Content      string `json:"content"`
	ToolsVersion string `json:"tools_version,omitempty"`
}

// https://schema.org/SoftwareSourceCode
type SoftwareSourceCode struct {
	Context             []string            `json:"@context"`
	Type                string              `json:"@type"`
	Name                string              `json:"name"`
	Version             string              `json:"version"`
	Description         string              `json:"description,omitempty"`
	Keywords            []string            `json:"keywords,omitempty"`
	CodeRepository      string              `json:"codeRepository,omitempty"`
	License             string              `json:"license,omitempty"`
	Author              Person              `json:"author"`
	ProgrammingLanguage ProgrammingLanguage `json:"programmingLanguage"`
	RepositoryURLs      []string            `json:"repositoryURLs,omitempty"`
}

// https://schema.org/ProgrammingLanguage
type ProgrammingLanguage struct {
	Type string `json:"@type"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// https://schema.org/Person
type Person struct {
	Type       string `json:"@type,omitempty"`
	GivenName  string `json:"givenName,omitempty"`
	MiddleName string `json:"middleName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
}

func (p Person) String() string {
	var sb strings.Builder
	if p.GivenName != "" {
		sb.WriteString(p.GivenName)
	}
	if p.MiddleName != "" {
		if sb.Len() > 0 {
			sb.WriteRune(' ')
		}
		sb.WriteString(p.MiddleName)
	}
	if p.FamilyName != "" {
		if sb.Len() > 0 {
			sb.WriteRune(' ')
		}
		sb.WriteString(p.FamilyName)
	}
	return sb.String()
}

// ParsePackage parses the Swift package upload
func ParsePackage(sr io.ReaderAt, size int64, mr io.Reader) (*Package, error) {
	zr, err := zip.NewReader(sr, size)
	if err != nil {
		return nil, err
	}

	p := &Package{
		Metadata: &Metadata{
			Manifests: make(map[string]*Manifest),
		},
	}

	for _, file := range zr.File {
		manifestMatch := manifestPattern.FindStringSubmatch(path.Base(file.Name))
		if len(manifestMatch) == 0 {
			continue
		}

		if file.UncompressedSize64 > maxManifestFileSize {
			return nil, ErrManifestFileTooLarge
		}

		f, err := zr.Open(file.Name)
		if err != nil {
			return nil, err
		}

		content, err := io.ReadAll(f)

		if err := f.Close(); err != nil {
			return nil, err
		}

		if err != nil {
			return nil, err
		}

		swiftVersion := ""
		if len(manifestMatch) == 2 && manifestMatch[1] != "" {
			v, err := version.NewSemver(manifestMatch[1])
			if err != nil {
				return nil, ErrInvalidManifestVersion
			}
			swiftVersion = TrimmedVersionString(v)
		}

		manifest := &Manifest{
			Content: string(content),
		}

		toolsMatch := toolsVersionPattern.FindStringSubmatch(manifest.Content)
		if len(toolsMatch) == 2 {
			v, err := version.NewSemver(toolsMatch[1])
			if err != nil {
				return nil, ErrInvalidManifestVersion
			}

			manifest.ToolsVersion = TrimmedVersionString(v)
		}

		p.Metadata.Manifests[swiftVersion] = manifest
	}

	if _, found := p.Metadata.Manifests[""]; !found {
		return nil, ErrMissingManifestFile
	}

	if mr != nil {
		var ssc *SoftwareSourceCode
		if err := json.NewDecoder(mr).Decode(&ssc); err != nil {
			return nil, err
		}

		p.Metadata.Description = ssc.Description
		p.Metadata.Keywords = ssc.Keywords
		p.Metadata.License = ssc.License
		p.Metadata.Author = Person{
			GivenName:  ssc.Author.GivenName,
			MiddleName: ssc.Author.MiddleName,
			FamilyName: ssc.Author.FamilyName,
		}

		p.Metadata.RepositoryURL = ssc.CodeRepository
		if !validation.IsValidURL(p.Metadata.RepositoryURL) {
			p.Metadata.RepositoryURL = ""
		}

		p.RepositoryURLs = ssc.RepositoryURLs
	}

	return p, nil
}

// TrimmedVersionString returns the version string without the patch segment if it is zero
func TrimmedVersionString(v *version.Version) string {
	segments := v.Segments64()

	var b strings.Builder
	fmt.Fprintf(&b, "%d.%d", segments[0], segments[1])
	if segments[2] != 0 {
		fmt.Fprintf(&b, ".%d", segments[2])
	}
	return b.String()
}
