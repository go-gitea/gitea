// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"strings"

	"code.gitea.io/gitea/modules/json"

	"xorm.io/xorm"
)

func ChangeContainerMetadataMultiArch(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	type PackageVersion struct {
		ID           int64  `xorm:"pk"`
		MetadataJSON string `xorm:"metadata_json"`
	}

	type PackageBlob struct{}

	// Get all relevant packages (manifest list images have a container.manifest.reference property)

	var pvs []*PackageVersion
	err := sess.
		Table("package_version").
		Select("id, metadata_json").
		Where("id IN (SELECT DISTINCT ref_id FROM package_property WHERE ref_type = 0 AND name = 'container.manifest.reference')").
		Find(&pvs)
	if err != nil {
		return err
	}

	type MetadataOld struct {
		Type             string            `json:"type"`
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
		MultiArch        map[string]string `json:"multiarch,omitempty"`
	}

	type Manifest struct {
		Platform string `json:"platform"`
		Digest   string `json:"digest"`
		Size     int64  `json:"size"`
	}

	type MetadataNew struct {
		Type             string            `json:"type"`
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

	for _, pv := range pvs {
		var old *MetadataOld
		if err := json.Unmarshal([]byte(pv.MetadataJSON), &old); err != nil {
			return err
		}

		// Calculate the size of every contained manifest

		manifests := make([]*Manifest, 0, len(old.MultiArch))
		for platform, digest := range old.MultiArch {
			size, err := sess.
				Table("package_blob").
				Join("INNER", "package_file", "package_blob.id = package_file.blob_id").
				Join("INNER", "package_version pv", "pv.id = package_file.version_id").
				Join("INNER", "package_version pv2", "pv2.package_id = pv.package_id").
				Where("pv.lower_version = ? AND pv2.id = ?", strings.ToLower(digest), pv.ID).
				SumInt(new(PackageBlob), "size")
			if err != nil {
				return err
			}

			manifests = append(manifests, &Manifest{
				Platform: platform,
				Digest:   digest,
				Size:     size,
			})
		}

		// Convert to new metadata format

		new := &MetadataNew{
			Type:             old.Type,
			IsTagged:         old.IsTagged,
			Platform:         old.Platform,
			Description:      old.Description,
			Authors:          old.Authors,
			Licenses:         old.Licenses,
			ProjectURL:       old.ProjectURL,
			RepositoryURL:    old.RepositoryURL,
			DocumentationURL: old.DocumentationURL,
			Labels:           old.Labels,
			ImageLayers:      old.ImageLayers,
			Manifests:        manifests,
		}

		metadataJSON, err := json.Marshal(new)
		if err != nil {
			return err
		}

		pv.MetadataJSON = string(metadataJSON)

		if _, err := sess.ID(pv.ID).Update(pv); err != nil {
			return err
		}
	}

	return sess.Commit()
}
