// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cran

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"io"
	"path"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

const (
	PropertyType     = "cran.type"
	PropertyPlatform = "cran.platform"
	PropertyRVersion = "cran.rvserion"

	TypeSource = "source"
	TypeBinary = "binary"
)

var (
	ErrMissingDescriptionFile = util.NewInvalidArgumentErrorf("DESCRIPTION file is missing")
	ErrInvalidName            = util.NewInvalidArgumentErrorf("package name is invalid")
	ErrInvalidVersion         = util.NewInvalidArgumentErrorf("package version is invalid")
)

var (
	fieldPattern         = regexp.MustCompile(`\A\S+:`)
	namePattern          = regexp.MustCompile(`\A[a-zA-Z][a-zA-Z0-9\.]*[a-zA-Z0-9]\z`)
	versionPattern       = regexp.MustCompile(`\A[0-9]+(?:[.\-][0-9]+){1,3}\z`)
	authorReplacePattern = regexp.MustCompile(`[\[\(].+?[\]\)]`)
)

// Package represents a CRAN package
type Package struct {
	Name          string
	Version       string
	FileExtension string
	Metadata      *Metadata
}

// Metadata represents the metadata of a CRAN package
type Metadata struct {
	Title            string   `json:"title,omitempty"`
	Description      string   `json:"description,omitempty"`
	ProjectURL       []string `json:"project_url,omitempty"`
	License          string   `json:"license,omitempty"`
	Authors          []string `json:"authors,omitempty"`
	Depends          []string `json:"depends,omitempty"`
	Imports          []string `json:"imports,omitempty"`
	Suggests         []string `json:"suggests,omitempty"`
	LinkingTo        []string `json:"linking_to,omitempty"`
	NeedsCompilation bool     `json:"needs_compilation"`
}

type ReaderReaderAt interface {
	io.Reader
	io.ReaderAt
}

// ParsePackage reads the package metadata from a CRAN package
// .zip and .tar.gz/.tgz files are supported.
func ParsePackage(r ReaderReaderAt, size int64) (*Package, error) {
	magicBytes := make([]byte, 2)
	if _, err := r.ReadAt(magicBytes, 0); err != nil {
		return nil, err
	}

	if magicBytes[0] == 0x1F && magicBytes[1] == 0x8B {
		return parsePackageTarGz(r)
	}
	return parsePackageZip(r, size)
}

func parsePackageTarGz(r io.Reader) (*Package, error) {
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

		if strings.Count(hd.Name, "/") > 1 {
			continue
		}

		if path.Base(hd.Name) == "DESCRIPTION" {
			p, err := ParseDescription(tr)
			if p != nil {
				p.FileExtension = ".tar.gz"
			}
			return p, err
		}
	}

	return nil, ErrMissingDescriptionFile
}

func parsePackageZip(r io.ReaderAt, size int64) (*Package, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}

	for _, file := range zr.File {
		if strings.Count(file.Name, "/") > 1 {
			continue
		}

		if path.Base(file.Name) == "DESCRIPTION" {
			f, err := zr.Open(file.Name)
			if err != nil {
				return nil, err
			}
			defer f.Close()

			p, err := ParseDescription(f)
			if p != nil {
				p.FileExtension = ".zip"
			}
			return p, err
		}
	}

	return nil, ErrMissingDescriptionFile
}

// ParseDescription parses a DESCRIPTION file to retrieve the metadata of a CRAN package
func ParseDescription(r io.Reader) (*Package, error) {
	p := &Package{
		Metadata: &Metadata{},
	}

	scanner := bufio.NewScanner(r)

	var b strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !fieldPattern.MatchString(line) {
			b.WriteRune(' ')
			b.WriteString(line)
			continue
		}

		if err := setField(p, b.String()); err != nil {
			return nil, err
		}

		b.Reset()
		b.WriteString(line)
	}

	if err := setField(p, b.String()); err != nil {
		return nil, err
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return p, nil
}

func setField(p *Package, data string) error {
	if data == "" {
		return nil
	}

	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		return nil
	}

	name := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	switch name {
	case "Package":
		if !namePattern.MatchString(value) {
			return ErrInvalidName
		}
		p.Name = value
	case "Version":
		if !versionPattern.MatchString(value) {
			return ErrInvalidVersion
		}
		p.Version = value
	case "Title":
		p.Metadata.Title = value
	case "Description":
		p.Metadata.Description = value
	case "URL":
		p.Metadata.ProjectURL = splitAndTrim(value)
	case "License":
		p.Metadata.License = value
	case "Author":
		p.Metadata.Authors = splitAndTrim(authorReplacePattern.ReplaceAllString(value, ""))
	case "Depends":
		p.Metadata.Depends = splitAndTrim(value)
	case "Imports":
		p.Metadata.Imports = splitAndTrim(value)
	case "Suggests":
		p.Metadata.Suggests = splitAndTrim(value)
	case "LinkingTo":
		p.Metadata.LinkingTo = splitAndTrim(value)
	case "NeedsCompilation":
		p.Metadata.NeedsCompilation = value == "yes"
	}

	return nil
}

func splitAndTrim(s string) []string {
	items := strings.Split(s, ", ")
	for i := range items {
		items[i] = strings.TrimSpace(items[i])
	}
	return items
}
