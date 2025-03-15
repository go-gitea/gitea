// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"

	"code.gitea.io/gitea/modules/json"
)

const (
	PropertyTerraformState = "terraform.state"
)

// Metadata represents the Terraform backend metadata
// Updated to align with TerraformState structure
// Includes additional metadata fields like Description, Author, and URLs
type Metadata struct {
	Version          int             `json:"version"`
	TerraformVersion string          `json:"terraform_version,omitempty"`
	Serial           uint64          `json:"serial"`
	Lineage          string          `json:"lineage"`
	Outputs          map[string]any  `json:"outputs,omitempty"`
	Resources        []ResourceState `json:"resources,omitempty"`
	Description      string          `json:"description,omitempty"`
	Author           string          `json:"author,omitempty"`
	ProjectURL       string          `json:"project_url,omitempty"`
	RepositoryURL    string          `json:"repository_url,omitempty"`
}

// ResourceState represents the state of a resource
type ResourceState struct {
	Mode      string          `json:"mode"`
	Type      string          `json:"type"`
	Name      string          `json:"name"`
	Provider  string          `json:"provider"`
	Instances []InstanceState `json:"instances"`
}

// InstanceState represents the state of a resource instance
type InstanceState struct {
	SchemaVersion int            `json:"schema_version"`
	Attributes    map[string]any `json:"attributes"`
}

// ParseMetadataFromState retrieves metadata from the archive with Terraform state
func ParseMetadataFromState(r io.Reader) (*Metadata, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hd, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if hd.Typeflag != tar.TypeReg {
			continue
		}

		// Looking for the state.json file
		if hd.Name == "state.json" {
			return ParseStateFile(tr)
		}
	}

	return nil, errors.New("state.json not found in archive")
}

// ParseStateFile parses the state.json file and returns Terraform metadata
func ParseStateFile(r io.Reader) (*Metadata, error) {
	var stateData Metadata
	if err := json.NewDecoder(r).Decode(&stateData); err != nil {
		return nil, err
	}
	return &stateData, nil
}
