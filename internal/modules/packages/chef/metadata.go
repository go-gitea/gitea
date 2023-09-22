// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package chef

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"regexp"
	"strings"

	"code.gitea.io/gitea/internal/modules/json"
	"code.gitea.io/gitea/internal/modules/util"
	"code.gitea.io/gitea/internal/modules/validation"
)

const (
	KeyBits          = 4096
	SettingPublicPem = "chef.public_pem"
)

var (
	ErrMissingMetadataFile = util.NewInvalidArgumentErrorf("metadata.json file is missing")
	ErrInvalidName         = util.NewInvalidArgumentErrorf("package name is invalid")
	ErrInvalidVersion      = util.NewInvalidArgumentErrorf("package version is invalid")

	namePattern    = regexp.MustCompile(`\A\S+\z`)
	versionPattern = regexp.MustCompile(`\A\d+\.\d+(?:\.\d+)?\z`)
)

// Package represents a Chef package
type Package struct {
	Name     string
	Version  string
	Metadata *Metadata
}

// Metadata represents the metadata of a Chef package
type Metadata struct {
	Description     string            `json:"description,omitempty"`
	LongDescription string            `json:"long_description,omitempty"`
	Author          string            `json:"author,omitempty"`
	License         string            `json:"license,omitempty"`
	RepositoryURL   string            `json:"repository_url,omitempty"`
	Dependencies    map[string]string `json:"dependencies,omitempty"`
}

type chefMetadata struct {
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	LongDescription    string            `json:"long_description"`
	Maintainer         string            `json:"maintainer"`
	MaintainerEmail    string            `json:"maintainer_email"`
	License            string            `json:"license"`
	Platforms          map[string]string `json:"platforms"`
	Dependencies       map[string]string `json:"dependencies"`
	Providing          map[string]string `json:"providing"`
	Recipes            map[string]string `json:"recipes"`
	Version            string            `json:"version"`
	SourceURL          string            `json:"source_url"`
	IssuesURL          string            `json:"issues_url"`
	Privacy            bool              `json:"privacy"`
	ChefVersions       [][]string        `json:"chef_versions"`
	Gems               [][]string        `json:"gems"`
	EagerLoadLibraries bool              `json:"eager_load_libraries"`
}

// ParsePackage parses the Chef package file
func ParsePackage(r io.Reader) (*Package, error) {
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

		if strings.Count(hd.Name, "/") != 1 {
			continue
		}

		if hd.FileInfo().Name() == "metadata.json" {
			return ParseChefMetadata(tr)
		}
	}

	return nil, ErrMissingMetadataFile
}

// ParseChefMetadata parses a metadata.json file to retrieve the metadata of a Chef package
func ParseChefMetadata(r io.Reader) (*Package, error) {
	var cm chefMetadata
	if err := json.NewDecoder(r).Decode(&cm); err != nil {
		return nil, err
	}

	if !namePattern.MatchString(cm.Name) {
		return nil, ErrInvalidName
	}

	if !versionPattern.MatchString(cm.Version) {
		return nil, ErrInvalidVersion
	}

	if !validation.IsValidURL(cm.SourceURL) {
		cm.SourceURL = ""
	}

	return &Package{
		Name:    cm.Name,
		Version: cm.Version,
		Metadata: &Metadata{
			Description:     cm.Description,
			LongDescription: cm.LongDescription,
			Author:          cm.Maintainer,
			License:         cm.License,
			RepositoryURL:   cm.SourceURL,
			Dependencies:    cm.Dependencies,
		},
	}, nil
}
