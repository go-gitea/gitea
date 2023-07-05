// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package helm

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/validation"

	"github.com/hashicorp/go-version"
	"gopkg.in/yaml.v3"
)

var (
	// ErrMissingChartFile indicates a missing Chart.yaml file
	ErrMissingChartFile = util.NewInvalidArgumentErrorf("Chart.yaml file is missing")
	// ErrInvalidName indicates an invalid package name
	ErrInvalidName = util.NewInvalidArgumentErrorf("package name is invalid")
	// ErrInvalidVersion indicates an invalid package version
	ErrInvalidVersion = util.NewInvalidArgumentErrorf("package version is invalid")
	// ErrInvalidChart indicates an invalid chart
	ErrInvalidChart = util.NewInvalidArgumentErrorf("chart is invalid")
)

// Metadata for a Chart file. This models the structure of a Chart.yaml file.
type Metadata struct {
	APIVersion   string            `json:"api_version" yaml:"apiVersion"`
	Type         string            `json:"type,omitempty" yaml:"type,omitempty"`
	Name         string            `json:"name" yaml:"name"`
	Version      string            `json:"version" yaml:"version"`
	AppVersion   string            `json:"app_version,omitempty" yaml:"appVersion,omitempty"`
	Home         string            `json:"home,omitempty" yaml:"home,omitempty"`
	Sources      []string          `json:"sources,omitempty" yaml:"sources,omitempty"`
	Description  string            `json:"description,omitempty" yaml:"description,omitempty"`
	Keywords     []string          `json:"keywords,omitempty" yaml:"keywords,omitempty"`
	Maintainers  []*Maintainer     `json:"maintainers,omitempty" yaml:"maintainers,omitempty"`
	Icon         string            `json:"icon,omitempty" yaml:"icon,omitempty"`
	Condition    string            `json:"condition,omitempty" yaml:"condition,omitempty"`
	Tags         string            `json:"tags,omitempty" yaml:"tags,omitempty"`
	Deprecated   bool              `json:"deprecated,omitempty" yaml:"deprecated,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	KubeVersion  string            `json:"kube_version,omitempty" yaml:"kubeVersion,omitempty"`
	Dependencies []*Dependency     `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
}

type Maintainer struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	Email string `json:"email,omitempty" yaml:"email,omitempty"`
	URL   string `json:"url,omitempty" yaml:"url,omitempty"`
}

type Dependency struct {
	Name         string   `json:"name" yaml:"name"`
	Version      string   `json:"version,omitempty" yaml:"version,omitempty"`
	Repository   string   `json:"repository" yaml:"repository"`
	Condition    string   `json:"condition,omitempty" yaml:"condition,omitempty"`
	Tags         []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Enabled      bool     `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	ImportValues []any    `json:"import_values,omitempty" yaml:"import-values,omitempty"`
	Alias        string   `json:"alias,omitempty" yaml:"alias,omitempty"`
}

// ParseChartArchive parses the metadata of a Helm archive
func ParseChartArchive(r io.Reader) (*Metadata, error) {
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

		if hd.FileInfo().Name() == "Chart.yaml" {
			if strings.Count(hd.Name, "/") != 1 {
				continue
			}

			return ParseChartFile(tr)
		}
	}

	return nil, ErrMissingChartFile
}

// ParseChartFile parses a Chart.yaml file to retrieve the metadata of a Helm chart
func ParseChartFile(r io.Reader) (*Metadata, error) {
	var metadata *Metadata
	if err := yaml.NewDecoder(r).Decode(&metadata); err != nil {
		return nil, err
	}

	if metadata.APIVersion == "" {
		return nil, ErrInvalidChart
	}

	if metadata.Type != "" && metadata.Type != "application" && metadata.Type != "library" {
		return nil, ErrInvalidChart
	}

	if metadata.Name == "" {
		return nil, ErrInvalidName
	}

	if _, err := version.NewSemver(metadata.Version); err != nil {
		return nil, ErrInvalidVersion
	}

	if !validation.IsValidURL(metadata.Home) {
		metadata.Home = ""
	}

	return metadata, nil
}
