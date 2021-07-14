// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package nuget

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"io"
	"path/filepath"
	"regexp"
	"strings"

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

var idmatch = regexp.MustCompile(`\A\w+(?:[.-]\w+)*\z`)

const maxNuspecFileSize = 3 * 1024 * 1024

// Metadata represents the metadata of a Nuget package
type Metadata struct {
	ID            string                  `json:"-"`
	Version       string                  `json:"-"`
	Description   string                  `json:"description"`
	Summary       string                  `json:"summary"`
	ReleaseNotes  string                  `json:"release_notes"`
	Authors       string                  `json:"authors"`
	ProjectURL    string                  `json:"project_url"`
	RepositoryURL string                  `json:"repository_url"`
	Dependencies  map[string][]Dependency `json:"dependencies"`
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
		Summary                  string `xml:"summary"`
		ReleaseNotes             string `xml:"releaseNotes"`
		Repository               struct {
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
func ParsePackageMetaData(r io.ReaderAt, size int64) (*Metadata, error) {
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
func ParseNuspecMetaData(r io.Reader) (*Metadata, error) {
	var p nuspecPackage
	dec := xml.NewDecoder(r)
	err := dec.Decode(&p)
	if err != nil {
		return nil, err
	}

	if !idmatch.MatchString(p.Metadata.ID) {
		return nil, ErrNuspecInvalidID
	}

	v, err := version.NewSemver(p.Metadata.Version)
	if err != nil {
		return nil, ErrNuspecInvalidVersion
	}

	m := &Metadata{
		ID:            p.Metadata.ID,
		Version:       v.String(),
		Description:   p.Metadata.Description,
		Summary:       p.Metadata.Summary,
		ReleaseNotes:  p.Metadata.ReleaseNotes,
		Authors:       p.Metadata.Authors,
		ProjectURL:    p.Metadata.ProjectURL,
		RepositoryURL: p.Metadata.Repository.URL,
		Dependencies:  make(map[string][]Dependency),
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
		m.Dependencies[group.TargetFramework] = deps
	}
	return m, nil
}
