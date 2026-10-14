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

	"gitea.dev/modules/packages"
	"gitea.dev/modules/util"
	"gitea.dev/modules/validation"

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

	// cap on the multi-value .PKGINFO entries (depends, provides, ...) summed over all keys
	maxPackageInfoEntries = 10000
)

var (
	ErrMissingPKGINFOFile  = util.NewInvalidArgumentErrorf(".PKGINFO file is missing")
	ErrUnsupportedFormat   = util.NewInvalidArgumentErrorf("unsupported package container format")
	ErrInvalidName         = util.NewInvalidArgumentErrorf("package name is invalid")
	ErrInvalidVersion      = util.NewInvalidArgumentErrorf("package version is invalid")
	ErrInvalidArchitecture = util.NewInvalidArgumentErrorf("package architecture is invalid")
	ErrTooManyEntries      = util.NewInvalidArgumentErrorf(".PKGINFO file contains too many entries")

	// https://man.archlinux.org/man/PKGBUILD.5
	namePattern = regexp.MustCompile(`\A[a-zA-Z0-9@._+-]+\z`)
	// (epoch:pkgver-pkgrel)
	versionPattern = regexp.MustCompile(`\A(?:\d:)?[\w.+~]+(?:-[-\w.+~]+)?\z`)

	// caps on the accumulated package file list (vars so tests can lower them); far above
	// any legitimate package, but low enough to stop metadata amplification
	maxFileEntries   = 100000
	maxFileNameBytes = 16 * 1024 * 1024
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
	files := packages.NewBoundedFileList(maxFileEntries, maxFileNameBytes)

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
			if strings.ContainsAny(hd.Name, "\n\r") {
				continue // a newline would forge extra lines in the pacman index
			}
			if err := files.Add(hd.Name); err != nil {
				return nil, err
			}
		}
	}

	if p == nil {
		return nil, ErrMissingPKGINFOFile
	}

	p.FileMetadata.Files = files.Files()
	p.FileCompressionExtension = compressionType

	return p, nil
}

// ParsePackageInfo parses a .PKGINFO file to retrieve the metadata
// https://man.archlinux.org/man/PKGBUILD.5
// https://gitlab.archlinux.org/pacman/pacman/-/blob/master/lib/libalpm/be_package.c#L161
func ParsePackageInfo(r io.Reader) (*Package, error) {
	p := &Package{}

	lists := map[string]*[]string{
		"license":     &p.VersionMetadata.Licenses,
		"provides":    &p.FileMetadata.Provides,
		"depend":      &p.FileMetadata.Depends,
		"replaces":    &p.FileMetadata.Replaces,
		"optdepend":   &p.FileMetadata.OptDepends,
		"makedepend":  &p.FileMetadata.MakeDepends,
		"checkdepend": &p.FileMetadata.CheckDepends,
		"conflict":    &p.FileMetadata.Conflicts,
		"backup":      &p.FileMetadata.Backup,
		"group":       &p.FileMetadata.Groups,
		"xdata":       &p.FileMetadata.XData,
	}
	entries := 0

	s := bufio.NewScanner(util.NewLimitedReader(r, packages.MaxMetadataFileSize))
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

		if list, ok := lists[key]; ok {
			if entries++; entries > maxPackageInfoEntries {
				return nil, ErrTooManyEntries
			}
			*list = append(*list, value)
			continue
		}

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
