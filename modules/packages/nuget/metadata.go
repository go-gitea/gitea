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
	PackageType   PackageType
	ID            string
	Version       string
	Metadata      *Metadata
	NuspecContent *bytes.Buffer
}

// Metadata represents the metadata of a Nuget package
type Metadata struct {
	Authors                  string `json:"authors,omitempty"`
	Copyright                string `json:"copyright,omitempty"`
	Description              string `json:"description,omitempty"`
	DevelopmentDependency    bool   `json:"development_dependency,omitempty"`
	IconURL                  string `json:"icon_url,omitempty"`
	Language                 string `json:"language,omitempty"`
	LicenseURL               string `json:"license_url,omitempty"`
	MinClientVersion         string `json:"min_client_version,omitempty"`
	Owners                   string `json:"owners,omitempty"`
	ProjectURL               string `json:"project_url,omitempty"`
	Readme                   string `json:"readme,omitempty"`
	ReleaseNotes             string `json:"release_notes,omitempty"`
	RepositoryURL            string `json:"repository_url,omitempty"`
	RequireLicenseAcceptance bool   `json:"require_license_acceptance"`
	Summary                  string `json:"summary,omitempty"`
	Tags                     string `json:"tags,omitempty"`
	Title                    string `json:"title,omitempty"`

	Dependencies map[string][]Dependency `json:"dependencies,omitempty"`
}

// Dependency represents a dependency of a Nuget package
type Dependency struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// https://learn.microsoft.com/en-us/nuget/reference/nuspec
// https://github.com/NuGet/NuGet.Client/blob/dev/src/NuGet.Core/NuGet.Packaging/compiler/resources/nuspec.xsd
type nuspecPackage struct {
	Metadata struct {
		// required fields
		Authors     string `xml:"authors"`
		Description string `xml:"description"`
		ID          string `xml:"id"`
		Version     string `xml:"version"`

		// optional fields
		Copyright                string `xml:"copyright"`
		DevelopmentDependency    bool   `xml:"developmentDependency"`
		IconURL                  string `xml:"iconUrl"`
		Language                 string `xml:"language"`
		LicenseURL               string `xml:"licenseUrl"`
		MinClientVersion         string `xml:"minClientVersion,attr"`
		Owners                   string `xml:"owners"`
		ProjectURL               string `xml:"projectUrl"`
		Readme                   string `xml:"readme"`
		ReleaseNotes             string `xml:"releaseNotes"`
		RequireLicenseAcceptance bool   `xml:"requireLicenseAcceptance"`
		Summary                  string `xml:"summary"`
		Tags                     string `xml:"tags"`
		Title                    string `xml:"title"`

		Dependencies struct {
			Dependency []struct {
				ID      string `xml:"id,attr"`
				Version string `xml:"version,attr"`
				Exclude string `xml:"exclude,attr"`
			} `xml:"dependency"`
			Group []struct {
				TargetFramework string `xml:"targetFramework,attr"`
				Dependency      []struct {
					ID      string `xml:"id,attr"`
					Version string `xml:"version,attr"`
					Exclude string `xml:"exclude,attr"`
				} `xml:"dependency"`
			} `xml:"group"`
		} `xml:"dependencies"`
		PackageTypes struct {
			PackageType []struct {
				Name string `xml:"name,attr"`
			} `xml:"packageType"`
		} `xml:"packageTypes"`
		Repository struct {
			URL string `xml:"url,attr"`
		} `xml:"repository"`
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

			return ParseNuspecMetaData(archive, f)
		}
	}
	return nil, ErrMissingNuspecFile
}

// ParseNuspecMetaData parses a Nuspec file to retrieve the metadata of a Nuget package
func ParseNuspecMetaData(archive *zip.Reader, r io.Reader) (*Package, error) {
	var nuspecBuf bytes.Buffer
	var p nuspecPackage
	if err := xml.NewDecoder(io.TeeReader(r, &nuspecBuf)).Decode(&p); err != nil {
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
		Authors:                  p.Metadata.Authors,
		Copyright:                p.Metadata.Copyright,
		Description:              p.Metadata.Description,
		DevelopmentDependency:    p.Metadata.DevelopmentDependency,
		IconURL:                  p.Metadata.IconURL,
		Language:                 p.Metadata.Language,
		LicenseURL:               p.Metadata.LicenseURL,
		MinClientVersion:         p.Metadata.MinClientVersion,
		Owners:                   p.Metadata.Owners,
		ProjectURL:               p.Metadata.ProjectURL,
		ReleaseNotes:             p.Metadata.ReleaseNotes,
		RepositoryURL:            p.Metadata.Repository.URL,
		RequireLicenseAcceptance: p.Metadata.RequireLicenseAcceptance,
		Summary:                  p.Metadata.Summary,
		Tags:                     p.Metadata.Tags,
		Title:                    p.Metadata.Title,

		Dependencies: make(map[string][]Dependency),
	}

	if p.Metadata.Readme != "" {
		f, err := archive.Open(p.Metadata.Readme)
		if err == nil {
			buf, _ := io.ReadAll(f)
			m.Readme = string(buf)
			_ = f.Close()
		}
	}

	if len(p.Metadata.Dependencies.Dependency) > 0 {
		deps := make([]Dependency, 0, len(p.Metadata.Dependencies.Dependency))
		for _, dep := range p.Metadata.Dependencies.Dependency {
			if dep.ID == "" || dep.Version == "" {
				continue
			}
			deps = append(deps, Dependency{
				ID:      dep.ID,
				Version: dep.Version,
			})
		}
		m.Dependencies[""] = deps
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
		PackageType:   packageType,
		ID:            p.Metadata.ID,
		Version:       toNormalizedVersion(v),
		Metadata:      m,
		NuspecContent: &nuspecBuf,
	}, nil
}

// https://learn.microsoft.com/en-us/nuget/concepts/package-versioning#normalized-version-numbers
// https://github.com/NuGet/NuGet.Client/blob/dccbd304b11103e08b97abf4cf4bcc1499d9235a/src/NuGet.Core/NuGet.Versioning/VersionFormatter.cs#L121
func toNormalizedVersion(v *version.Version) string {
	var buf bytes.Buffer
	segments := v.Segments64()
	_, _ = fmt.Fprintf(&buf, "%d.%d.%d", segments[0], segments[1], segments[2])
	if len(segments) > 3 && segments[3] > 0 {
		_, _ = fmt.Fprintf(&buf, ".%d", segments[3])
	}
	pre := v.Prerelease()
	if pre != "" {
		_, _ = fmt.Fprint(&buf, "-", pre)
	}
	return buf.String()
}
