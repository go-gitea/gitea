// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package maven

import (
	"encoding/xml"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
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

// pds is expected to be sorted ascending by CreatedUnix
func createMetadataResponse(pds []*packages_model.PackageDescriptor, groupID, artifactID string) *MetadataResponse {
	var release *packages_model.PackageDescriptor

	versions := make([]string, 0, len(pds))
	for _, pd := range pds {
		if !strings.HasSuffix(pd.Version.Version, "-SNAPSHOT") {
			release = pd
		}
		versions = append(versions, pd.Version.Version)
	}

	latest := pds[len(pds)-1]

	resp := &MetadataResponse{
		GroupID:    groupID,
		ArtifactID: artifactID,
		Latest:     latest.Version.Version,
		Version:    versions,
	}
	if release != nil {
		resp.Release = release.Version.Version
	}
	return resp
}
