// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package maven

import (
	"encoding/xml"
	"strings"
)

// MetadataResponse https://maven.apache.org/ref/3.2.5/maven-repository-metadata/repository-metadata.html
type MetadataResponse struct {
	XMLName    xml.Name `xml:"metadata"`
	GroupID    string   `xml:"groupId"`
	ArtifactID string   `xml:"artifactId"`
	Release    string   `xml:"versioning>release,omitempty"`
	Latest     string   `xml:"versioning>latest"`
	Version    []string `xml:"versioning>versions>version"`
}

func createMetadataResponse(packages []*Package) *MetadataResponse {
	sortedPackages := sortPackagesByCreationASC(packages)

	var release *Package

	versions := make([]string, 0, len(sortedPackages))
	for _, p := range sortedPackages {
		if !strings.HasSuffix(p.Version, "-SNAPSHOT") {
			release = p
		}
		versions = append(versions, p.Version)
	}

	latest := sortedPackages[len(sortedPackages)-1]

	resp := &MetadataResponse{
		GroupID:    latest.Metadata.GroupID,
		ArtifactID: latest.Metadata.ArtifactID,
		Latest:     latest.Version,
		Version:    versions,
	}
	if release != nil {
		resp.Release = release.Version
	}
	return resp
}
