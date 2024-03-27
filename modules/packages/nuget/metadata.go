// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package nuget

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/validation"

	"github.com/hashicorp/go-version"
)

var (
	// ErrMissingNuspecFile indicates a missing Nuspec file
	ErrMissingNuspecFile = util.NewInvalidArgumentErrorf("Nuspec file is missing")
	// ErrNuspecFileTooLarge indicates a Nuspec file which is too large
	ErrNuspecFileTooLarge = util.NewInvalidArgumentErrorf("Nuspec file is too large")
	// ErrNuspecInvalidID indicates an invalid id in the Nuspec file
	ErrNuspecInvalidID = util.NewInvalidArgumentErrorf("Nuspec file contains an invalid id")
	// ErrNuspecInvalidVersion indicates an invalid version in the Nuspec file
	ErrNuspecInvalidVersion = util.NewInvalidArgumentErrorf("Nuspec file contains an invalid version")
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

type nuspecPackageType struct {
	Name string `xml:"name,attr"`
}

type nuspecPackageTypes struct {
	PackageType []nuspecPackageType `xml:"packageType"`
}

type nuspecRepository struct {
	URL  string `xml:"url,attr,omitempty"`
	Type string `xml:"type,attr,omitempty"`
}
type nuspecDependency struct {
	ID      string `xml:"id,attr"`
	Version string `xml:"version,attr"`
	Exclude string `xml:"exclude,attr,omitempty"`
}

type nuspecGroup struct {
	TargetFramework string             `xml:"targetFramework,attr"`
	Dependency      []nuspecDependency `xml:"dependency"`
}

type nuspecDependencies struct {
	Group []nuspecGroup `xml:"group"`
}

type nuspeceMetadata struct {
	ID                       string              `xml:"id"`
	Version                  string              `xml:"version"`
	Authors                  string              `xml:"authors"`
	RequireLicenseAcceptance bool                `xml:"requireLicenseAcceptance,omitempty"`
	ProjectURL               string              `xml:"projectUrl,omitempty"`
	Description              string              `xml:"description"`
	ReleaseNotes             string              `xml:"releaseNotes,omitempty"`
	PackageTypes             *nuspecPackageTypes `xml:"packageTypes,omitempty"`
	Repository               *nuspecRepository   `xml:"repository,omitempty"`
	Dependencies             *nuspecDependencies `xml:"dependencies,omitempty"`
}

type nuspecPackage struct {
	XMLName  xml.Name        `xml:"package"`
	Xmlns    string          `xml:"xmlns,attr"`
	Metadata nuspeceMetadata `xml:"metadata"`
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
	if p.Metadata.PackageTypes != nil {
		for _, pt := range p.Metadata.PackageTypes.PackageType {
			if pt.Name == "SymbolsPackage" {
				packageType = SymbolsPackage
				break
			}
		}
	}

	m := &Metadata{
		Description:              p.Metadata.Description,
		ReleaseNotes:             p.Metadata.ReleaseNotes,
		Authors:                  p.Metadata.Authors,
		ProjectURL:               p.Metadata.ProjectURL,
		RequireLicenseAcceptance: p.Metadata.RequireLicenseAcceptance,
		Dependencies:             make(map[string][]Dependency),
	}
	if p.Metadata.Repository != nil {
		m.RepositoryURL = p.Metadata.Repository.URL
	}
	if p.Metadata.Dependencies != nil {
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
	}
	return &Package{
		PackageType: packageType,
		ID:          p.Metadata.ID,
		Version:     toNormalizedVersion(v),
		Metadata:    m,
	}, nil
}

// https://learn.microsoft.com/en-us/nuget/concepts/package-versioning#normalized-version-numbers
// https://github.com/NuGet/NuGet.Client/blob/dccbd304b11103e08b97abf4cf4bcc1499d9235a/src/NuGet.Core/NuGet.Versioning/VersionFormatter.cs#L121
func toNormalizedVersion(v *version.Version) string {
	var buf bytes.Buffer
	segments := v.Segments64()
	fmt.Fprintf(&buf, "%d.%d.%d", segments[0], segments[1], segments[2])
	if len(segments) > 3 && segments[3] > 0 {
		fmt.Fprintf(&buf, ".%d", segments[3])
	}
	pre := v.Prerelease()
	if pre != "" {
		fmt.Fprint(&buf, "-", pre)
	}
	return buf.String()
}

// returning any here because we use a private type and we don't need the type for xml marshalling
func GenerateNuspec(pd *Package) any {
	m := nuspeceMetadata{
		ID:                       pd.ID,
		Version:                  pd.Version,
		Authors:                  pd.Metadata.Authors,
		Description:              pd.Metadata.Description,
		ProjectURL:               pd.Metadata.ProjectURL,
		RequireLicenseAcceptance: pd.Metadata.RequireLicenseAcceptance,
	}

	if pd.Metadata.RepositoryURL != "" {
		m.Repository = &nuspecRepository{
			URL: pd.Metadata.RepositoryURL,
		}
	}

	groups := len(pd.Metadata.Dependencies)
	if groups > 0 {
		m.Dependencies = &nuspecDependencies{
			Group: make([]nuspecGroup, 0, groups),
		}

		for tgf, deps := range pd.Metadata.Dependencies {
			if len(deps) == 0 {
				continue
			}
			gDeps := make([]nuspecDependency, 0, len(deps))
			for _, dep := range deps {
				gDeps = append(gDeps, nuspecDependency{
					ID:      dep.ID,
					Version: dep.Version,
				})
			}

			m.Dependencies.Group = append(m.Dependencies.Group, nuspecGroup{
				TargetFramework: tgf,
				Dependency:      gDeps,
			})
		}
	}

	return &nuspecPackage{
		Xmlns:    "http://schemas.microsoft.com/packaging/2010/07/nuspec.xsd",
		Metadata: m,
	}
}
