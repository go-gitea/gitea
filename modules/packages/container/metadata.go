// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import (
	"fmt"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/packages/container/helm"
	"code.gitea.io/gitea/modules/validation"

	oci "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	PropertyRepository        = "container.repository"
	PropertyDigest            = "container.digest"
	PropertyMediaType         = "container.mediatype"
	PropertyManifestTagged    = "container.manifest.tagged"
	PropertyManifestReference = "container.manifest.reference"

	DefaultPlatform = "linux/amd64"

	labelLicenses      = "org.opencontainers.image.licenses"
	labelURL           = "org.opencontainers.image.url"
	labelSource        = "org.opencontainers.image.source"
	labelDocumentation = "org.opencontainers.image.documentation"
	labelDescription   = "org.opencontainers.image.description"
	labelAuthors       = "org.opencontainers.image.authors"
)

type ImageType string

const (
	TypeOCI  ImageType = "oci"
	TypeHelm ImageType = "helm"
)

// Name gets the name of the image type
func (it ImageType) Name() string {
	switch it {
	case TypeHelm:
		return "Helm Chart"
	default:
		return "OCI / Docker"
	}
}

// Metadata represents the metadata of a Container package
type Metadata struct {
	Type             ImageType         `json:"type"`
	IsTagged         bool              `json:"is_tagged"`
	Platform         string            `json:"platform,omitempty"`
	Description      string            `json:"description,omitempty"`
	Authors          []string          `json:"authors,omitempty"`
	Licenses         string            `json:"license,omitempty"`
	ProjectURL       string            `json:"project_url,omitempty"`
	RepositoryURL    string            `json:"repository_url,omitempty"`
	DocumentationURL string            `json:"documentation_url,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
	ImageLayers      []string          `json:"layer_creation,omitempty"`
	Manifests        []*Manifest       `json:"manifests,omitempty"`
}

type Manifest struct {
	Platform string `json:"platform"`
	Digest   string `json:"digest"`
	Size     int64  `json:"size"`
}

// ParseImageConfig parses the metadata of an image config
func ParseImageConfig(mt string, r io.Reader) (*Metadata, error) {
	if strings.EqualFold(mt, helm.ConfigMediaType) {
		return parseHelmConfig(r)
	}

	// fallback to OCI Image Config
	return parseOCIImageConfig(r)
}

func parseOCIImageConfig(r io.Reader) (*Metadata, error) {
	var image oci.Image
	if err := json.NewDecoder(r).Decode(&image); err != nil {
		return nil, err
	}

	platform := DefaultPlatform
	if image.OS != "" && image.Architecture != "" {
		platform = fmt.Sprintf("%s/%s", image.OS, image.Architecture)
		if image.Variant != "" {
			platform = fmt.Sprintf("%s/%s", platform, image.Variant)
		}
	}

	imageLayers := make([]string, 0, len(image.History))
	for _, history := range image.History {
		cmd := history.CreatedBy
		if i := strings.Index(cmd, "#(nop) "); i != -1 {
			cmd = strings.TrimSpace(cmd[i+7:])
		}
		if cmd != "" {
			imageLayers = append(imageLayers, cmd)
		}
	}

	metadata := &Metadata{
		Type:             TypeOCI,
		Platform:         platform,
		Licenses:         image.Config.Labels[labelLicenses],
		ProjectURL:       image.Config.Labels[labelURL],
		RepositoryURL:    image.Config.Labels[labelSource],
		DocumentationURL: image.Config.Labels[labelDocumentation],
		Description:      image.Config.Labels[labelDescription],
		Labels:           image.Config.Labels,
		ImageLayers:      imageLayers,
	}

	if authors, ok := image.Config.Labels[labelAuthors]; ok {
		metadata.Authors = []string{authors}
	}

	if !validation.IsValidURL(metadata.ProjectURL) {
		metadata.ProjectURL = ""
	}
	if !validation.IsValidURL(metadata.RepositoryURL) {
		metadata.RepositoryURL = ""
	}
	if !validation.IsValidURL(metadata.DocumentationURL) {
		metadata.DocumentationURL = ""
	}

	return metadata, nil
}

func parseHelmConfig(r io.Reader) (*Metadata, error) {
	var config helm.Metadata
	if err := json.NewDecoder(r).Decode(&config); err != nil {
		return nil, err
	}

	metadata := &Metadata{
		Type:        TypeHelm,
		Description: config.Description,
		ProjectURL:  config.Home,
	}

	if len(config.Maintainers) > 0 {
		authors := make([]string, 0, len(config.Maintainers))
		for _, maintainer := range config.Maintainers {
			authors = append(authors, maintainer.Name)
		}
		metadata.Authors = authors
	}

	if len(config.Sources) > 0 && validation.IsValidURL(config.Sources[0]) {
		metadata.RepositoryURL = config.Sources[0]
	}
	if !validation.IsValidURL(metadata.ProjectURL) {
		metadata.ProjectURL = ""
	}

	return metadata, nil
}
