// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package composer

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
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

// PackageInfo represents Composer package info
type PackageInfo struct {
	Filename string

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
	Comments    Comments          `json:"_comment,omitempty"`
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
		*l = values
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
		*c = values
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

type ReadSeekAt interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	Size() int64
}

func readPackageFileZip(r ReadSeekAt, filename string, limit int) ([]byte, error) {
	archive, err := zip.NewReader(r, r.Size())
	if err != nil {
		return nil, err
	}

	for _, file := range archive.File {
		filePath := path.Clean(file.Name)
		if util.AsciiEqualFold(filePath, filename) {
			f, err := archive.Open(file.Name)
			if err != nil {
				return nil, err
			}
			defer f.Close()

			return util.ReadWithLimit(f, limit)
		}
	}
	return nil, fs.ErrNotExist
}

func readPackageFileTar(r io.Reader, filename string, limit int) ([]byte, error) {
	tarReader := tar.NewReader(r)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		filePath := path.Clean(header.Name)
		if util.AsciiEqualFold(filePath, filename) {
			return util.ReadWithLimit(tarReader, limit)
		}
	}
	return nil, fs.ErrNotExist
}

const (
	pkgExtZip    = ".zip"
	pkgExtTarGz  = ".tar.gz"
	pkgExtTarBz2 = ".tar.bz2"
)

func detectPackageExtName(r ReadSeekAt) (string, error) {
	headBytes := make([]byte, 4)
	_, err := r.ReadAt(headBytes, 0)
	if err != nil {
		return "", err
	}
	_, err = r.Seek(0, io.SeekStart)
	if err != nil {
		return "", err
	}
	switch {
	case headBytes[0] == 'P' && headBytes[1] == 'K':
		return pkgExtZip, nil
	case string(headBytes[:3]) == "BZh":
		return pkgExtTarBz2, nil
	case headBytes[0] == 0x1f && headBytes[1] == 0x8b:
		return pkgExtTarGz, nil
	}
	return "", util.NewInvalidArgumentErrorf("not a valid package file")
}

func readPackageFile(pkgExt string, r ReadSeekAt, filename string, limit int) ([]byte, error) {
	_, err := r.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	switch pkgExt {
	case pkgExtZip:
		return readPackageFileZip(r, filename, limit)
	case pkgExtTarBz2:
		bzip2Reader := bzip2.NewReader(r)
		return readPackageFileTar(bzip2Reader, filename, limit)
	case pkgExtTarGz:
		gzReader, err := gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
		return readPackageFileTar(gzReader, filename, limit)
	}
	return nil, util.NewInvalidArgumentErrorf("not a valid package file")
}

// ParsePackage parses the metadata of a Composer package file
func ParsePackage(r ReadSeekAt, optVersion ...string) (*PackageInfo, error) {
	pkgExt, err := detectPackageExtName(r)
	if err != nil {
		return nil, err
	}
	dataComposerJSON, err := readPackageFile(pkgExt, r, "composer.json", 10*1024*1024)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrMissingComposerFile
	} else if err != nil {
		return nil, err
	}

	var cj struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Type    string `json:"type"`
		Metadata
	}
	if err := json.Unmarshal(dataComposerJSON, &cj); err != nil {
		return nil, err
	}

	if !nameMatch.MatchString(cj.Name) {
		return nil, ErrInvalidName
	}

	if cj.Version == "" {
		cj.Version = util.OptionalArg(optVersion)
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
	dataReadmeMd, _ := readPackageFile(pkgExt, r, cj.Readme, 10*1024)

	// FIXME: legacy problem, the "Readme" field is abused, it should always be the path to the readme file
	if len(dataReadmeMd) == 0 {
		cj.Readme = ""
	} else {
		cj.Readme = string(dataReadmeMd)
	}

	// FIXME: legacy format: strings.ToLower(fmt.Sprintf("%s.%s.zip", strings.ReplaceAll(cp.Name, "/", "-"), cp.Version)), doesn't read good
	pkgFilename := strings.ReplaceAll(cj.Name, "/", "-")
	if cj.Version != "" {
		pkgFilename += "." + cj.Version
	}
	pkgFilename += pkgExt
	return &PackageInfo{
		Filename: pkgFilename,
		Name:     cj.Name,
		Version:  cj.Version,
		Type:     cj.Type,
		Metadata: &cj.Metadata,
	}, nil
}
