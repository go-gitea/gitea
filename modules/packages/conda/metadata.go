// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conda

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/validation"

	"github.com/klauspost/compress/zstd"
)

var (
	ErrInvalidStructure = util.SilentWrap{Message: "package structure is invalid", Err: util.ErrInvalidArgument}
	ErrInvalidName      = util.SilentWrap{Message: "package name is invalid", Err: util.ErrInvalidArgument}
	ErrInvalidVersion   = util.SilentWrap{Message: "package version is invalid", Err: util.ErrInvalidArgument}
)

const (
	PropertyName     = "conda.name"
	PropertyChannel  = "conda.channel"
	PropertySubdir   = "conda.subdir"
	PropertyMetadata = "conda.metadata"
)

// Package represents a Conda package
type Package struct {
	Name            string
	Version         string
	Subdir          string
	VersionMetadata *VersionMetadata
	FileMetadata    *FileMetadata
}

// VersionMetadata represents the metadata of a Conda package
type VersionMetadata struct {
	Description      string `json:"description,omitempty"`
	Summary          string `json:"summary,omitempty"`
	ProjectURL       string `json:"project_url,omitempty"`
	RepositoryURL    string `json:"repository_url,omitempty"`
	DocumentationURL string `json:"documentation_url,omitempty"`
	License          string `json:"license,omitempty"`
	LicenseFamily    string `json:"license_family,omitempty"`
}

// FileMetadata represents the metadata of a Conda package file
type FileMetadata struct {
	IsCondaPackage bool     `json:"is_conda"`
	Architecture   string   `json:"architecture,omitempty"`
	NoArch         string   `json:"noarch,omitempty"`
	Build          string   `json:"build,omitempty"`
	BuildNumber    int64    `json:"build_number,omitempty"`
	Dependencies   []string `json:"dependencies,omitempty"`
	Platform       string   `json:"platform,omitempty"`
	Timestamp      int64    `json:"timestamp,omitempty"`
}

type index struct {
	Name          string   `json:"name"`
	Version       string   `json:"version"`
	Architecture  string   `json:"arch"`
	NoArch        string   `json:"noarch"`
	Build         string   `json:"build"`
	BuildNumber   int64    `json:"build_number"`
	Dependencies  []string `json:"depends"`
	License       string   `json:"license"`
	LicenseFamily string   `json:"license_family"`
	Platform      string   `json:"platform"`
	Subdir        string   `json:"subdir"`
	Timestamp     int64    `json:"timestamp"`
}

type about struct {
	Description      string `json:"description"`
	Summary          string `json:"summary"`
	ProjectURL       string `json:"home"`
	RepositoryURL    string `json:"dev_url"`
	DocumentationURL string `json:"doc_url"`
}

type ReaderAndReaderAt interface {
	io.Reader
	io.ReaderAt
}

// ParsePackageBZ2 parses the Conda package file compressed with bzip2
func ParsePackageBZ2(r io.Reader) (*Package, error) {
	gzr := bzip2.NewReader(r)

	return parsePackageTar(gzr)
}

// ParsePackageConda parses the Conda package file compressed with zip and zstd
func ParsePackageConda(r io.ReaderAt, size int64) (*Package, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}

	for _, file := range zr.File {
		if strings.HasPrefix(file.Name, "info-") && strings.HasSuffix(file.Name, ".tar.zst") {
			f, err := zr.Open(file.Name)
			if err != nil {
				return nil, err
			}
			defer f.Close()

			dec, err := zstd.NewReader(f)
			if err != nil {
				return nil, err
			}
			defer dec.Close()

			p, err := parsePackageTar(dec)
			if p != nil {
				p.FileMetadata.IsCondaPackage = true
			}
			return p, err
		}
	}

	return nil, ErrInvalidStructure
}

func parsePackageTar(r io.Reader) (*Package, error) {
	var i *index
	var a *about

	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		if hdr.Name == "info/index.json" {
			if err := json.NewDecoder(tr).Decode(&i); err != nil {
				return nil, err
			}

			if !checkName(i.Name) {
				return nil, ErrInvalidName
			}

			if !checkVersion(i.Version) {
				return nil, ErrInvalidVersion
			}

			if a != nil {
				break // stop loop if both files were found
			}
		} else if hdr.Name == "info/about.json" {
			if err := json.NewDecoder(tr).Decode(&a); err != nil {
				return nil, err
			}

			if !validation.IsValidURL(a.ProjectURL) {
				a.ProjectURL = ""
			}
			if !validation.IsValidURL(a.RepositoryURL) {
				a.RepositoryURL = ""
			}
			if !validation.IsValidURL(a.DocumentationURL) {
				a.DocumentationURL = ""
			}

			if i != nil {
				break // stop loop if both files were found
			}
		}
	}

	if i == nil {
		return nil, ErrInvalidStructure
	}
	if a == nil {
		a = &about{}
	}

	return &Package{
		Name:    i.Name,
		Version: i.Version,
		Subdir:  i.Subdir,
		VersionMetadata: &VersionMetadata{
			License:          i.License,
			LicenseFamily:    i.LicenseFamily,
			Description:      a.Description,
			Summary:          a.Summary,
			ProjectURL:       a.ProjectURL,
			RepositoryURL:    a.RepositoryURL,
			DocumentationURL: a.DocumentationURL,
		},
		FileMetadata: &FileMetadata{
			Architecture: i.Architecture,
			NoArch:       i.NoArch,
			Build:        i.Build,
			BuildNumber:  i.BuildNumber,
			Dependencies: i.Dependencies,
			Platform:     i.Platform,
			Timestamp:    i.Timestamp,
		},
	}, nil
}

// https://github.com/conda/conda-build/blob/db9a728a9e4e6cfc895637ca3221117970fc2663/conda_build/metadata.py#L1393
func checkName(name string) bool {
	if name == "" {
		return false
	}
	if name != strings.ToLower(name) {
		return false
	}
	return !checkBadCharacters(name, "!")
}

// https://github.com/conda/conda-build/blob/db9a728a9e4e6cfc895637ca3221117970fc2663/conda_build/metadata.py#L1403
func checkVersion(version string) bool {
	if version == "" {
		return false
	}
	return !checkBadCharacters(version, "-")
}

func checkBadCharacters(s, additional string) bool {
	if strings.ContainsAny(s, "=@#$%^&*:;\"'\\|<>?/ ") {
		return true
	}
	return strings.ContainsAny(s, additional)
}
