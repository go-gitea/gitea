// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package alpine

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/validation"
)

var (
	ErrMissingPKGINFOFile = util.NewInvalidArgumentErrorf("PKGINFO file is missing")
	ErrInvalidName        = util.NewInvalidArgumentErrorf("package name is invalid")
	ErrInvalidVersion     = util.NewInvalidArgumentErrorf("package version is invalid")
)

const (
	PropertyMetadata     = "alpine.metadata"
	PropertyBranch       = "alpine.branch"
	PropertyRepository   = "alpine.repository"
	PropertyArchitecture = "alpine.architecture"

	SettingKeyPrivate = "alpine.key.private"
	SettingKeyPublic  = "alpine.key.public"

	RepositoryPackage = "_alpine"
	RepositoryVersion = "_repository"
)

// https://wiki.alpinelinux.org/wiki/Apk_spec

// Package represents an Alpine package
type Package struct {
	Name            string
	Version         string
	VersionMetadata VersionMetadata
	FileMetadata    FileMetadata
}

// Metadata of an Alpine package
type VersionMetadata struct {
	Description string `json:"description,omitempty"`
	License     string `json:"license,omitempty"`
	ProjectURL  string `json:"project_url,omitempty"`
	Maintainer  string `json:"maintainer,omitempty"`
}

type FileMetadata struct {
	Checksum     string   `json:"checksum"`
	Packager     string   `json:"packager,omitempty"`
	BuildDate    int64    `json:"build_date,omitempty"`
	Size         int64    `json:"size,omitempty"`
	Architecture string   `json:"architecture,omitempty"`
	Origin       string   `json:"origin,omitempty"`
	CommitHash   string   `json:"commit_hash,omitempty"`
	InstallIf    string   `json:"install_if,omitempty"`
	Provides     []string `json:"provides,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
}

// ParsePackage parses the Alpine package file
func ParsePackage(r io.Reader) (*Package, error) {
	// Alpine packages are concated .tar.gz streams. Usually the first stream contains the package metadata.

	br := bufio.NewReader(r) // needed for gzip Multistream

	h := sha1.New()

	gzr, err := gzip.NewReader(&teeByteReader{br, h})
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	for {
		gzr.Multistream(false)

		tr := tar.NewReader(gzr)
		for {
			hd, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}

			if hd.Name == ".PKGINFO" {
				p, err := ParsePackageInfo(tr)
				if err != nil {
					return nil, err
				}

				// drain the reader
				for {
					if _, err := tr.Next(); err != nil {
						break
					}
				}

				p.FileMetadata.Checksum = "Q1" + base64.StdEncoding.EncodeToString(h.Sum(nil))

				return p, nil
			}
		}

		h = sha1.New()

		err = gzr.Reset(&teeByteReader{br, h})
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	return nil, ErrMissingPKGINFOFile
}

// ParsePackageInfo parses a PKGINFO file to retrieve the metadata of an Alpine package
func ParsePackageInfo(r io.Reader) (*Package, error) {
	p := &Package{}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "#") {
			continue
		}

		i := strings.IndexRune(line, '=')
		if i == -1 {
			continue
		}

		key := strings.TrimSpace(line[:i])
		value := strings.TrimSpace(line[i+1:])

		switch key {
		case "pkgname":
			p.Name = value
		case "pkgver":
			p.Version = value
		case "pkgdesc":
			p.VersionMetadata.Description = value
		case "url":
			p.VersionMetadata.ProjectURL = value
		case "builddate":
			n, err := strconv.ParseInt(value, 10, 64)
			if err == nil {
				p.FileMetadata.BuildDate = n
			}
		case "size":
			n, err := strconv.ParseInt(value, 10, 64)
			if err == nil {
				p.FileMetadata.Size = n
			}
		case "arch":
			p.FileMetadata.Architecture = value
		case "origin":
			p.FileMetadata.Origin = value
		case "commit":
			p.FileMetadata.CommitHash = value
		case "maintainer":
			p.VersionMetadata.Maintainer = value
		case "packager":
			p.FileMetadata.Packager = value
		case "license":
			p.VersionMetadata.License = value
		case "install_if":
			p.FileMetadata.InstallIf = value
		case "provides":
			if value != "" {
				p.FileMetadata.Provides = append(p.FileMetadata.Provides, value)
			}
		case "depend":
			if value != "" {
				p.FileMetadata.Dependencies = append(p.FileMetadata.Dependencies, value)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if p.Name == "" {
		return nil, ErrInvalidName
	}

	if p.Version == "" {
		return nil, ErrInvalidVersion
	}

	if !validation.IsValidURL(p.VersionMetadata.ProjectURL) {
		p.VersionMetadata.ProjectURL = ""
	}

	return p, nil
}

// Same as io.TeeReader but implements io.ByteReader
type teeByteReader struct {
	r *bufio.Reader
	w io.Writer
}

func (t *teeByteReader) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	if n > 0 {
		if n, err := t.w.Write(p[:n]); err != nil {
			return n, err
		}
	}
	return n, err
}

func (t *teeByteReader) ReadByte() (byte, error) {
	b, err := t.r.ReadByte()
	if err == nil {
		if _, err := t.w.Write([]byte{b}); err != nil {
			return 0, err
		}
	}
	return b, err
}
