// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package maven

import (
	"encoding/xml"
	"errors"
	"io"
	"strconv"

	"gitea.dev/modules/util"
	"gitea.dev/modules/validation"

	"golang.org/x/net/html/charset"
)

// Metadata represents the metadata of a Maven package
type Metadata struct {
	GroupID      string        `json:"group_id,omitempty"`
	ArtifactID   string        `json:"artifact_id,omitempty"`
	Name         string        `json:"name,omitempty"`
	Description  string        `json:"description,omitempty"`
	ProjectURL   string        `json:"project_url,omitempty"`
	Licenses     []string      `json:"licenses,omitempty"`
	Dependencies []*Dependency `json:"dependencies,omitempty"`
}

// Dependency represents a dependency of a Maven package
type Dependency struct {
	GroupID    string `json:"group_id,omitempty"`
	ArtifactID string `json:"artifact_id,omitempty"`
	Version    string `json:"version,omitempty"`
}

// SnapshotMetadata holds the build number and advertised files for a snapshot version.
type SnapshotMetadata struct {
	BuildNumber int      `json:"build_number,omitempty"`
	Files       []string `json:"files,omitempty"`
}

type pomStruct struct {
	XMLName xml.Name `xml:"project"`

	Parent struct {
		GroupID    string `xml:"groupId"`
		ArtifactID string `xml:"artifactId"`
		Version    string `xml:"version"`
	} `xml:"parent"`

	GroupID     string `xml:"groupId"`
	ArtifactID  string `xml:"artifactId"`
	Version     string `xml:"version"`
	Name        string `xml:"name"`
	Description string `xml:"description"`
	URL         string `xml:"url"`

	Licenses []struct {
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

type snapshotMetadataStruct struct {
	XMLName    xml.Name `xml:"metadata"`
	GroupID    string   `xml:"groupId"`
	ArtifactID string   `xml:"artifactId"`
	Version    string   `xml:"version"`
	Versioning struct {
		LastUpdated string `xml:"lastUpdated"`
		Snapshot    struct {
			Timestamp   string `xml:"timestamp"`
			BuildNumber string `xml:"buildNumber"`
		} `xml:"snapshot"`
		SnapshotVersions []struct {
			Extension  string `xml:"extension"`
			Classifier string `xml:"classifier"`
			Value      string `xml:"value"`
			Updated    string `xml:"updated"`
		} `xml:"snapshotVersions>snapshotVersion"`
	} `xml:"versioning"`
}

// ParsePackageMetaData parses the metadata of a pom file
func ParsePackageMetaData(r io.Reader) (*Metadata, error) {
	var pom pomStruct

	dec := xml.NewDecoder(r)
	dec.CharsetReader = charset.NewReaderLabel
	if err := dec.Decode(&pom); err != nil {
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

	pomGroupID := pom.GroupID
	if pomGroupID == "" {
		// the current module could inherit parent: https://maven.apache.org/pom.html#Inheritance
		pomGroupID = pom.Parent.GroupID
	}
	if pomGroupID == "" {
		return nil, util.ErrInvalidArgument
	}
	return &Metadata{
		GroupID:      pomGroupID,
		ArtifactID:   pom.ArtifactID,
		Name:         pom.Name,
		Description:  pom.Description,
		ProjectURL:   pom.URL,
		Licenses:     licenses,
		Dependencies: dependencies,
	}, nil
}

// ParseSnapshotVersionMetadata parses the build number and advertised files of a Maven snapshot.
func ParseSnapshotVersionMetadata(r io.Reader) (*SnapshotMetadata, error) {
	var metadata snapshotMetadataStruct

	dec := xml.NewDecoder(r)
	dec.CharsetReader = charset.NewReaderLabel
	if err := dec.Decode(&metadata); err != nil {
		return nil, err
	}

	buildNumber, err := strconv.Atoi(metadata.Versioning.Snapshot.BuildNumber)
	if err != nil {
		return nil, errors.New("invalid or missing build number in snapshot metadata")
	}

	if metadata.ArtifactID == "" {
		return nil, errors.New("missing artifact ID in snapshot metadata")
	}
	var files []string
	for _, snapshotVersion := range metadata.Versioning.SnapshotVersions {
		if snapshotVersion.Value == "" || snapshotVersion.Extension == "" {
			return nil, errors.New("invalid snapshot version in metadata")
		}
		filename := metadata.ArtifactID + "-" + snapshotVersion.Value
		if snapshotVersion.Classifier != "" {
			filename += "-" + snapshotVersion.Classifier
		}
		files = append(files, filename+"."+snapshotVersion.Extension)
	}

	return &SnapshotMetadata{
		BuildNumber: buildNumber,
		Files:       files,
	}, nil
}
