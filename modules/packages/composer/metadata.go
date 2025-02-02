// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package composer

import (
	"archive/zip"
	"io"
	"path"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/validation"

	"github.com/hashicorp/go-version"
)

// TypeProperty is the name of the property for Composer package types
const TypeProperty = "composer.type"

var (
	// ErrMissingComposerFile indicates a missing composer.json file
	ErrMissingComposerFile = util.NewInvalidArgumentErrorf("composer.json file is missing")
	// ErrInvalidName indicates an invalid package name
	ErrInvalidName = util.NewInvalidArgumentErrorf("package name is invalid")
	// ErrInvalidVersion indicates an invalid package version
	ErrInvalidVersion = util.NewInvalidArgumentErrorf("package version is invalid")
)

// Package represents a Composer package
type Package struct {
	Name     string
	Version  string
	Type     string
	Metadata *Metadata
}

// https://getcomposer.org/doc/04-schema.md

// Metadata represents the metadata of a Composer package
type Metadata struct {
	Description string            `json:"description,omitempty"`
	Readme      string            `json:"readme,omitempty"`
	Keywords    []string          `json:"keywords,omitempty"`
	Comments    Comments          `json:"_comments,omitempty"`
	Homepage    string            `json:"homepage,omitempty"`
	License     Licenses          `json:"license,omitempty"`
	Authors     []Author          `json:"authors,omitempty"`
	Bin         []string          `json:"bin,omitempty"`
	Autoload    map[string]any    `json:"autoload,omitempty"`
	AutoloadDev map[string]any    `json:"autoload-dev,omitempty"`
	Extra       map[string]any    `json:"extra,omitempty"`
	Require     map[string]string `json:"require,omitempty"`
	RequireDev  map[string]string `json:"require-dev,omitempty"`
	Suggest     map[string]string `json:"suggest,omitempty"`
	Provide     map[string]string `json:"provide,omitempty"`
}

// Licenses represents the licenses of a Composer package
type Licenses []string

// UnmarshalJSON reads from a string or array
func (l *Licenses) UnmarshalJSON(data []byte) error {
	switch data[0] {
	case '"':
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*l = Licenses{value}
	case '[':
		values := make([]string, 0, 5)
		if err := json.Unmarshal(data, &values); err != nil {
			return err
		}
		*l = Licenses(values)
	}
	return nil
}

// Comments represents the comments of a Composer package
type Comments []string

// UnmarshalJSON reads from a string or array
func (c *Comments) UnmarshalJSON(data []byte) error {
	switch data[0] {
	case '"':
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*c = Comments{value}
	case '[':
		values := make([]string, 0, 5)
		if err := json.Unmarshal(data, &values); err != nil {
			return err
		}
		*c = Comments(values)
	}
	return nil
}

// Author represents an author
type Author struct {
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
	Homepage string `json:"homepage,omitempty"`
}

var nameMatch = regexp.MustCompile(`\A[a-z0-9]([_\.-]?[a-z0-9]+)*/[a-z0-9](([_\.]?|-{0,2})[a-z0-9]+)*\z`)

// ParsePackage parses the metadata of a Composer package file
func ParsePackage(r io.ReaderAt, size int64) (*Package, error) {
	archive, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}

	for _, file := range archive.File {
		if strings.Count(file.Name, "/") > 1 {
			continue
		}
		if strings.HasSuffix(strings.ToLower(file.Name), "composer.json") {
			f, err := archive.Open(file.Name)
			if err != nil {
				return nil, err
			}
			defer f.Close()

			return ParseComposerFile(archive, path.Dir(file.Name), f)
		}
	}
	return nil, ErrMissingComposerFile
}

// ParseComposerFile parses a composer.json file to retrieve the metadata of a Composer package
func ParseComposerFile(archive *zip.Reader, pathPrefix string, r io.Reader) (*Package, error) {
	var cj struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Type    string `json:"type"`
		Metadata
	}
	if err := json.NewDecoder(r).Decode(&cj); err != nil {
		return nil, err
	}

	if !nameMatch.MatchString(cj.Name) {
		return nil, ErrInvalidName
	}

	if cj.Version != "" {
		if _, err := version.NewSemver(cj.Version); err != nil {
			return nil, ErrInvalidVersion
		}
	}

	if !validation.IsValidURL(cj.Homepage) {
		cj.Homepage = ""
	}

	if cj.Type == "" {
		cj.Type = "library"
	}

	if cj.Readme == "" {
		cj.Readme = "README.md"
	}
	f, err := archive.Open(path.Join(pathPrefix, cj.Readme))
	if err == nil {
		// 10kb limit for readme content
		buf, _ := io.ReadAll(io.LimitReader(f, 10*1024))
		cj.Readme = string(buf)
		_ = f.Close()
	} else {
		cj.Readme = ""
	}

	return &Package{
		Name:     cj.Name,
		Version:  cj.Version,
		Type:     cj.Type,
		Metadata: &cj.Metadata,
	}, nil
}
