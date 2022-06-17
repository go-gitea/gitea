// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oci

import (
	"strings"
)

const (
	MediaTypeImageManifest      = "application/vnd.oci.image.manifest.v1+json"
	MediaTypeImageIndex         = "application/vnd.oci.image.index.v1+json"
	MediaTypeDockerManifest     = "application/vnd.docker.distribution.manifest.v2+json"
	MediaTypeDockerManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
)

type MediaType string

// IsValid tests if the media type is in the OCI or Docker namespace
func (m MediaType) IsValid() bool {
	s := string(m)
	return strings.HasPrefix(s, "application/vnd.docker.") || strings.HasPrefix(s, "application/vnd.oci.")
}

// IsImageManifest tests if the media type is an image manifest
func (m MediaType) IsImageManifest() bool {
	s := string(m)
	return strings.EqualFold(s, MediaTypeDockerManifest) || strings.EqualFold(s, MediaTypeImageManifest)
}

// IsImageIndex tests if the media type is an image index
func (m MediaType) IsImageIndex() bool {
	s := string(m)
	return strings.EqualFold(s, MediaTypeDockerManifestList) || strings.EqualFold(s, MediaTypeImageIndex)
}
