// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pub

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/validation"

	"github.com/hashicorp/go-version"
	"gopkg.in/yaml.v3"
)

var (
	ErrMissingPubspecFile  = util.NewInvalidArgumentErrorf("Pubspec file is missing")
	ErrPubspecFileTooLarge = util.NewInvalidArgumentErrorf("Pubspec file is too large")
	ErrInvalidName         = util.NewInvalidArgumentErrorf("package name is invalid")
	ErrInvalidVersion      = util.NewInvalidArgumentErrorf("package version is invalid")
)

var namePattern = regexp.MustCompile(`\A[a-zA-Z_][a-zA-Z0-9_]*\z`)

// https://github.com/dart-lang/pub-dev/blob/4d582302a8d10152a5cd6129f65bf4f4dbca239d/pkg/pub_package_reader/lib/pub_package_reader.dart#L143
const maxPubspecFileSize = 128 * 1024

// Package represents a Pub package
type Package struct {
	Name     string
	Version  string
	Metadata *Metadata
}

// Metadata represents the metadata of a Pub package
type Metadata struct {
	Description      string `json:"description,omitempty"`
	ProjectURL       string `json:"project_url,omitempty"`
	RepositoryURL    string `json:"repository_url,omitempty"`
	DocumentationURL string `json:"documentation_url,omitempty"`
	Readme           string `json:"readme,omitempty"`
	Pubspec          any    `json:"pubspec"`
}

type pubspecPackage struct {
	Name          string `yaml:"name"`
	Version       string `yaml:"version"`
	Description   string `yaml:"description"`
	Homepage      string `yaml:"homepage"`
	Repository    string `yaml:"repository"`
	Documentation string `yaml:"documentation"`
}

// ParsePackage parses the Pub package file
func ParsePackage(r io.Reader) (*Package, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	var p *Package
	var readme string

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

		if hd.Name == "pubspec.yaml" {
			if hd.Size > maxPubspecFileSize {
				return nil, ErrPubspecFileTooLarge
			}
			p, err = ParsePubspecMetadata(tr)
			if err != nil {
				return nil, err
			}
		} else if strings.ToLower(hd.Name) == "readme.md" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			readme = string(data)
		}
	}

	if p == nil {
		return nil, ErrMissingPubspecFile
	}

	p.Metadata.Readme = readme

	return p, nil
}

// ParsePubspecMetadata parses a Pubspec file to retrieve the metadata of a Pub package
func ParsePubspecMetadata(r io.Reader) (*Package, error) {
	buf, err := io.ReadAll(io.LimitReader(r, maxPubspecFileSize))
	if err != nil {
		return nil, err
	}

	var p pubspecPackage
	if err := yaml.Unmarshal(buf, &p); err != nil {
		return nil, err
	}

	if !namePattern.MatchString(p.Name) {
		return nil, ErrInvalidName
	}

	v, err := version.NewSemver(p.Version)
	if err != nil {
		return nil, ErrInvalidVersion
	}

	if !validation.IsValidURL(p.Homepage) {
		p.Homepage = ""
	}
	if !validation.IsValidURL(p.Repository) {
		p.Repository = ""
	}

	var pubspec any
	if err := yaml.Unmarshal(buf, &pubspec); err != nil {
		return nil, err
	}

	return &Package{
		Name:    p.Name,
		Version: v.String(),
		Metadata: &Metadata{
			Description:      p.Description,
			ProjectURL:       p.Homepage,
			RepositoryURL:    p.Repository,
			DocumentationURL: p.Documentation,
			Pubspec:          pubspec,
		},
	}, nil
}
