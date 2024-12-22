// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/validation"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

const (
	PropertyRepository   = "arch.repository"
	PropertyArchitecture = "arch.architecture"
	PropertySignature    = "arch.signature"
	PropertyMetadata     = "arch.metadata"

	SettingKeyPrivate = "arch.key.private"
	SettingKeyPublic  = "arch.key.public"

	RepositoryPackage = "_arch"
	RepositoryVersion = "_repository"

	AnyArch = "any"
)

var (
	ErrMissingPKGINFOFile  = util.NewInvalidArgumentErrorf(".PKGINFO file is missing")
	ErrUnsupportedFormat   = util.NewInvalidArgumentErrorf("unsupported package container format")
	ErrInvalidName         = util.NewInvalidArgumentErrorf("package name is invalid")
	ErrInvalidVersion      = util.NewInvalidArgumentErrorf("package version is invalid")
	ErrInvalidArchitecture = util.NewInvalidArgumentErrorf("package architecture is invalid")

	// https://man.archlinux.org/man/PKGBUILD.5
	namePattern = regexp.MustCompile(`\A[a-zA-Z0-9@._+-]+\z`)
	// (epoch:pkgver-pkgrel)
	versionPattern = regexp.MustCompile(`\A(?:\d:)?[\w.+~]+(?:-[-\w.+~]+)?\z`)
)

type Package struct {
	Name                     string
	Version                  string
	VersionMetadata          VersionMetadata
	FileMetadata             FileMetadata
	FileCompressionExtension string
}

type VersionMetadata struct {
	Description string   `json:"description,omitempty"`
	ProjectURL  string   `json:"project_url,omitempty"`
	Licenses    []string `json:"licenses,omitempty"`
}

type FileMetadata struct {
	Architecture  string   `json:"architecture"`
	Base          string   `json:"base,omitempty"`
	InstalledSize int64    `json:"installed_size,omitempty"`
	BuildDate     int64    `json:"build_date,omitempty"`
	Packager      string   `json:"packager,omitempty"`
	Groups        []string `json:"groups,omitempty"`
	Provides      []string `json:"provides,omitempty"`
	Replaces      []string `json:"replaces,omitempty"`
	Depends       []string `json:"depends,omitempty"`
	OptDepends    []string `json:"opt_depends,omitempty"`
	MakeDepends   []string `json:"make_depends,omitempty"`
	CheckDepends  []string `json:"check_depends,omitempty"`
	Conflicts     []string `json:"conflicts,omitempty"`
	XData         []string `json:"xdata,omitempty"`
	Backup        []string `json:"backup,omitempty"`
	Files         []string `json:"files,omitempty"`
}

// ParsePackage parses an Arch package file
func ParsePackage(r io.Reader) (*Package, error) {
	header := make([]byte, 10)
	n, err := util.ReadAtMost(r, header)
	if err != nil {
		return nil, err
	}

	r = io.MultiReader(bytes.NewReader(header[:n]), r)

	var inner io.Reader
	var compressionType string
	if bytes.HasPrefix(header, []byte{0x28, 0xB5, 0x2F, 0xFD}) { // zst
		zr, err := zstd.NewReader(r)
		if err != nil {
			return nil, err
		}
		defer zr.Close()

		inner = zr
		compressionType = "zst"
	} else if bytes.HasPrefix(header, []byte{0xFD, 0x37, 0x7A, 0x58, 0x5A}) { // xz
		xzr, err := xz.NewReader(r)
		if err != nil {
			return nil, err
		}

		inner = xzr
		compressionType = "xz"
	} else if bytes.HasPrefix(header, []byte{0x1F, 0x8B}) { // gz
		gzr, err := gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
		defer gzr.Close()

		inner = gzr
		compressionType = "gz"
	} else {
		return nil, ErrUnsupportedFormat
	}

	var p *Package
	files := make([]string, 0, 10)

	tr := tar.NewReader(inner)
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

		filename := hd.FileInfo().Name()
		if filename == ".PKGINFO" {
			p, err = ParsePackageInfo(tr)
			if err != nil {
				return nil, err
			}
		} else if !strings.HasPrefix(filename, ".") {
			files = append(files, hd.Name)
		}
	}

	if p == nil {
		return nil, ErrMissingPKGINFOFile
	}

	p.FileMetadata.Files = files
	p.FileCompressionExtension = compressionType

	return p, nil
}

// ParsePackageInfo parses a .PKGINFO file to retrieve the metadata
// https://man.archlinux.org/man/PKGBUILD.5
// https://gitlab.archlinux.org/pacman/pacman/-/blob/master/lib/libalpm/be_package.c#L161
func ParsePackageInfo(r io.Reader) (*Package, error) {
	p := &Package{}

	s := bufio.NewScanner(r)
	for s.Scan() {
		line := s.Text()

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
		case "pkgbase":
			p.FileMetadata.Base = value
		case "pkgver":
			p.Version = value
		case "pkgdesc":
			p.VersionMetadata.Description = value
		case "url":
			p.VersionMetadata.ProjectURL = value
		case "packager":
			p.FileMetadata.Packager = value
		case "arch":
			p.FileMetadata.Architecture = value
		case "license":
			p.VersionMetadata.Licenses = append(p.VersionMetadata.Licenses, value)
		case "provides":
			p.FileMetadata.Provides = append(p.FileMetadata.Provides, value)
		case "depend":
			p.FileMetadata.Depends = append(p.FileMetadata.Depends, value)
		case "replaces":
			p.FileMetadata.Replaces = append(p.FileMetadata.Replaces, value)
		case "optdepend":
			p.FileMetadata.OptDepends = append(p.FileMetadata.OptDepends, value)
		case "makedepend":
			p.FileMetadata.MakeDepends = append(p.FileMetadata.MakeDepends, value)
		case "checkdepend":
			p.FileMetadata.CheckDepends = append(p.FileMetadata.CheckDepends, value)
		case "conflict":
			p.FileMetadata.Conflicts = append(p.FileMetadata.Conflicts, value)
		case "backup":
			p.FileMetadata.Backup = append(p.FileMetadata.Backup, value)
		case "group":
			p.FileMetadata.Groups = append(p.FileMetadata.Groups, value)
		case "builddate":
			date, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, err
			}
			p.FileMetadata.BuildDate = date
		case "size":
			size, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, err
			}
			p.FileMetadata.InstalledSize = size
		case "xdata":
			p.FileMetadata.XData = append(p.FileMetadata.XData, value)
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}

	if !namePattern.MatchString(p.Name) {
		return nil, ErrInvalidName
	}
	if !versionPattern.MatchString(p.Version) {
		return nil, ErrInvalidVersion
	}
	if p.FileMetadata.Architecture == "" {
		return nil, ErrInvalidArchitecture
	}

	if !validation.IsValidURL(p.VersionMetadata.ProjectURL) {
		p.VersionMetadata.ProjectURL = ""
	}

	return p, nil
}
