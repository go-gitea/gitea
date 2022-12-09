// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package nuget

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/validation"

	"github.com/hashicorp/go-version"
)

var (
	// ErrMissingNuspecFile indicates a missing Nuspec file
	ErrMissingNuspecFile = errors.New("Nuspec file is missing")
	// ErrNuspecFileTooLarge indicates a Nuspec file which is too large
	ErrNuspecFileTooLarge = errors.New("Nuspec file is too large")
	// ErrNuspecInvalidID indicates an invalid id in the Nuspec file
	ErrNuspecInvalidID = errors.New("Nuspec file contains an invalid id")
	// ErrNuspecInvalidVersion indicates an invalid version in the Nuspec file
	ErrNuspecInvalidVersion = errors.New("Nuspec file contains an invalid version")
)

// PackageType specifies the package type the metadata describes
type PackageType int

const (
	// DependencyPackage represents a package (*.nupkg)
	DependencyPackage PackageType = iota + 1
	// SymbolsPackage represents a symbol package (*.snupkg)
	SymbolsPackage

	PropertySymbolID = "nuget.symbol.id"
)

var idmatch = regexp.MustCompile(`\A\w+(?:[.-]\w+)*\z`)

const maxNuspecFileSize = 3 * 1024 * 1024

// Package represents a Nuget package
type Package struct {
	PackageType PackageType
	ID          string
	Version     string
	Metadata    *Metadata
}

// Metadata represents the metadata of a Nuget package
type Metadata struct {
	Description              string                  `json:"description,omitempty"`
	ReleaseNotes             string                  `json:"release_notes,omitempty"`
	Authors                  string                  `json:"authors,omitempty"`
	ProjectURL               string                  `json:"project_url,omitempty"`
	RepositoryURL            string                  `json:"repository_url,omitempty"`
	RequireLicenseAcceptance bool                    `json:"require_license_acceptance"`
	Dependencies             map[string][]Dependency `json:"dependencies,omitempty"`
}

// Dependency represents a dependency of a Nuget package
type Dependency struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type nuspecPackage struct {
	Metadata struct {
		ID                       string `xml:"id"`
		Version                  string `xml:"version"`
		Authors                  string `xml:"authors"`
		RequireLicenseAcceptance bool   `xml:"requireLicenseAcceptance"`
		ProjectURL               string `xml:"projectUrl"`
		Description              string `xml:"description"`
		ReleaseNotes             string `xml:"releaseNotes"`
		PackageTypes             struct {
			PackageType []struct {
				Name string `xml:"name,attr"`
			} `xml:"packageType"`
		} `xml:"packageTypes"`
		Repository struct {
			URL string `xml:"url,attr"`
		} `xml:"repository"`
		Dependencies struct {
			Group []struct {
				TargetFramework string `xml:"targetFramework,attr"`
				Dependency      []struct {
					ID      string `xml:"id,attr"`
					Version string `xml:"version,attr"`
					Exclude string `xml:"exclude,attr"`
				} `xml:"dependency"`
			} `xml:"group"`
		} `xml:"dependencies"`
	} `xml:"metadata"`
}

// ParsePackageMetaData parses the metadata of a Nuget package file
func ParsePackageMetaData(r io.ReaderAt, size int64) (*Package, error) {
	archive, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}

	for _, file := range archive.File {
		if filepath.Dir(file.Name) != "." {
			continue
		}
		if strings.HasSuffix(strings.ToLower(file.Name), ".nuspec") {
			if file.UncompressedSize64 > maxNuspecFileSize {
				return nil, ErrNuspecFileTooLarge
			}
			f, err := archive.Open(file.Name)
			if err != nil {
				return nil, err
			}
			defer f.Close()

			return ParseNuspecMetaData(f)
		}
	}
	return nil, ErrMissingNuspecFile
}

// ParseNuspecMetaData parses a Nuspec file to retrieve the metadata of a Nuget package
func ParseNuspecMetaData(r io.Reader) (*Package, error) {
	var p nuspecPackage
	if err := xml.NewDecoder(r).Decode(&p); err != nil {
		return nil, err
	}

	if !idmatch.MatchString(p.Metadata.ID) {
		return nil, ErrNuspecInvalidID
	}

	v, err := version.NewSemver(p.Metadata.Version)
	if err != nil {
		return nil, ErrNuspecInvalidVersion
	}

	if !validation.IsValidURL(p.Metadata.ProjectURL) {
		p.Metadata.ProjectURL = ""
	}

	packageType := DependencyPackage
	for _, pt := range p.Metadata.PackageTypes.PackageType {
		if pt.Name == "SymbolsPackage" {
			packageType = SymbolsPackage
			break
		}
	}

	m := &Metadata{
		Description:              p.Metadata.Description,
		ReleaseNotes:             p.Metadata.ReleaseNotes,
		Authors:                  p.Metadata.Authors,
		ProjectURL:               p.Metadata.ProjectURL,
		RepositoryURL:            p.Metadata.Repository.URL,
		RequireLicenseAcceptance: p.Metadata.RequireLicenseAcceptance,
		Dependencies:             make(map[string][]Dependency),
	}

	for _, group := range p.Metadata.Dependencies.Group {
		deps := make([]Dependency, 0, len(group.Dependency))
		for _, dep := range group.Dependency {
			if dep.ID == "" || dep.Version == "" {
				continue
			}
			deps = append(deps, Dependency{
				ID:      dep.ID,
				Version: dep.Version,
			})
		}
		if len(deps) > 0 {
			m.Dependencies[group.TargetFramework] = deps
		}
	}
	return &Package{
		PackageType: packageType,
		ID:          p.Metadata.ID,
		Version:     v.String(),
		Metadata:    m,
	}, nil
}
