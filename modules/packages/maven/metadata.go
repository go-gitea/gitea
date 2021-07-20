// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package maven

import (
	"encoding/xml"
	"io"

	"code.gitea.io/gitea/modules/validation"
)

// Metadata represents the metadata of a Maven package
type Metadata struct {
	GroupID      string        `json:"group_id"`
	ArtifactID   string        `json:"artifact_id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	ProjectURL   string        `json:"project_url"`
	Licenses     []string      `json:"licenses"`
	Dependencies []*Dependency `json:"dependencies"`
}

// Dependency represents a dependency of a Maven package
type Dependency struct {
	GroupID    string `json:"group_id"`
	ArtifactID string `json:"artifact_id"`
	Version    string `json:"version"`
}

type pomStruct struct {
	XMLName     xml.Name `xml:"project"`
	GroupID     string   `xml:"groupId"`
	ArtifactID  string   `xml:"artifactId"`
	Version     string   `xml:"version"`
	Name        string   `xml:"name"`
	Description string   `xml:"description"`
	URL         string   `xml:"url"`
	Licenses    []struct {
		Name         string `xml:"name"`
		URL          string `xml:"url"`
		Distribution string `xml:"distribution"`
	} `xml:"licenses>license"`
	Dependencies []struct {
		GroupID    string `xml:"groupId"`
		ArtifactID string `xml:"artifactId"`
		Version    string `xml:"version"`
		Scope      string `xml:"scope"`
	} `xml:"dependencies>dependency"`
}

// ParsePackageMetaData parses the metadata of a pom file
func ParsePackageMetaData(r io.Reader) (*Metadata, error) {
	var pom pomStruct
	if err := xml.NewDecoder(r).Decode(&pom); err != nil {
		return nil, err
	}

	if !validation.IsValidURL(pom.URL) {
		pom.URL = ""
	}

	licenses := make([]string, 0, len(pom.Licenses))
	for _, l := range pom.Licenses {
		if l.Name != "" {
			licenses = append(licenses, l.Name)
		}
	}

	dependencies := make([]*Dependency, 0, len(pom.Dependencies))
	for _, d := range pom.Dependencies {
		dependencies = append(dependencies, &Dependency{
			GroupID:    d.GroupID,
			ArtifactID: d.ArtifactID,
			Version:    d.Version,
		})
	}

	return &Metadata{
		GroupID:      pom.GroupID,
		ArtifactID:   pom.ArtifactID,
		Name:         pom.Name,
		Description:  pom.Description,
		ProjectURL:   pom.URL,
		Licenses:     licenses,
		Dependencies: dependencies,
	}, nil
}
