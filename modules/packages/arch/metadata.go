// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/validation"
	"github.com/mholt/archiver/v3"
)

var (
	// https://man.archlinux.org/man/PKGBUILD.5
	reName   = regexp.MustCompile(`^[a-zA-Z0-9@._+-]+$`)
	reVer    = regexp.MustCompile(`^[a-zA-Z0-9:_.+]+-+[0-9]+$`)
	reOptDep = regexp.MustCompile(`^[a-zA-Z0-9@._+-]+$|^[a-zA-Z0-9@._+-]+(:.*)`)
	rePkgVer = regexp.MustCompile(`^[a-zA-Z0-9@._+-]+$|^[a-zA-Z0-9@._+-]+(>.*)|^[a-zA-Z0-9@._+-]+(<.*)|^[a-zA-Z0-9@._+-]+(=.*)`)
)

type Package struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	VersionMetadata VersionMetadata
	FileMetadata    FileMetadata
}

// Arch package metadata related to specific version.
// Version metadata the same across different architectures and distributions.
type VersionMetadata struct {
	Base         string   `json:"base"`
	Description  string   `json:"description"`
	ProjectURL   string   `json:"project_url"`
	Groups       []string `json:"groups,omitempty"`
	Provides     []string `json:"provides,omitempty"`
	License      []string `json:"license,omitempty"`
	Depends      []string `json:"depends,omitempty"`
	OptDepends   []string `json:"opt_depends,omitempty"`
	MakeDepends  []string `json:"make_depends,omitempty"`
	CheckDepends []string `json:"check_depends,omitempty"`
	Backup       []string `json:"backup,omitempty"`
}

// Metadata related to specific pakcage file.
// This metadata might vary for different architecture and distribution.
type FileMetadata struct {
	CompressedSize int64  `json:"compressed_size"`
	InstalledSize  int64  `json:"installed_size"`
	MD5            string `json:"md5"`
	SHA256         string `json:"sha256"`
	BuildDate      int64  `json:"build_date"`
	Packager       string `json:"packager"`
	Arch           string `json:"arch"`
}

// Function that receives arch package archive data and returns it's metadata.
func ParsePackage(r io.Reader, md5, sha256 []byte, size int64) (*Package, error) {
	zstd := archiver.NewTarZstd()
	err := zstd.Open(r, 0)
	if err != nil {
		return nil, err
	}
	defer zstd.Close()

	var pkg *Package
	var mtree bool

	for {
		f, err := zstd.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		defer f.Close()

		switch f.Name() {
		case ".PKGINFO":
			pkg, err = ParsePackageInfo(f)
			if err != nil {
				return nil, err
			}
		case ".MTREE":
			mtree = true
		}
	}

	if pkg == nil {
		return nil, util.NewInvalidArgumentErrorf(".PKGINFO file not found")
	}

	if !mtree {
		return nil, util.NewInvalidArgumentErrorf(".MTREE file not found")
	}

	pkg.FileMetadata.CompressedSize = size
	pkg.FileMetadata.SHA256 = hex.EncodeToString(sha256)
	pkg.FileMetadata.MD5 = hex.EncodeToString(md5)

	return pkg, nil
}

// Function that accepts reader for .PKGINFO file from package archive,
// validates all field according to PKGBUILD spec and returns package.
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
		case "pkgbase":
			p.VersionMetadata.Base = value
		case "pkgver":
			p.Version = value
		case "pkgdesc":
			p.VersionMetadata.Description = value
		case "url":
			p.VersionMetadata.ProjectURL = value
		case "packager":
			p.FileMetadata.Packager = value
		case "arch":
			p.FileMetadata.Arch = value
		case "provides":
			p.VersionMetadata.Provides = append(p.VersionMetadata.Provides, value)
		case "license":
			p.VersionMetadata.License = append(p.VersionMetadata.License, value)
		case "depend":
			p.VersionMetadata.Depends = append(p.VersionMetadata.Depends, value)
		case "optdepend":
			p.VersionMetadata.OptDepends = append(p.VersionMetadata.OptDepends, value)
		case "makedepend":
			p.VersionMetadata.MakeDepends = append(p.VersionMetadata.MakeDepends, value)
		case "checkdepend":
			p.VersionMetadata.CheckDepends = append(p.VersionMetadata.CheckDepends, value)
		case "backup":
			p.VersionMetadata.Backup = append(p.VersionMetadata.Backup, value)
		case "group":
			p.VersionMetadata.Groups = append(p.VersionMetadata.Groups, value)
		case "builddate":
			bd, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, err
			}
			p.FileMetadata.BuildDate = bd
		case "size":
			is, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, err
			}
			p.FileMetadata.InstalledSize = is
		}
	}

	return p, errors.Join(scanner.Err(), ValidatePackageSpec(p))
}

// Arch package validation according to PKGBUILD specification.
func ValidatePackageSpec(p *Package) error {
	if !reName.MatchString(p.Name) {
		return util.NewInvalidArgumentErrorf("invalid package name")
	}
	if !reName.MatchString(p.VersionMetadata.Base) {
		return util.NewInvalidArgumentErrorf("invalid package base")
	}
	if !reVer.MatchString(p.Version) {
		return util.NewInvalidArgumentErrorf("invalid package version")
	}
	if p.FileMetadata.Arch == "" {
		return util.NewInvalidArgumentErrorf("architecture should be specified")
	}
	if p.VersionMetadata.ProjectURL != "" {
		if !validation.IsValidURL(p.VersionMetadata.ProjectURL) {
			return util.NewInvalidArgumentErrorf("invalid project URL")
		}
	}
	for _, cd := range p.VersionMetadata.CheckDepends {
		if !rePkgVer.MatchString(cd) {
			return util.NewInvalidArgumentErrorf("invalid check dependency: " + cd)
		}
	}
	for _, d := range p.VersionMetadata.Depends {
		if !rePkgVer.MatchString(d) {
			return util.NewInvalidArgumentErrorf("invalid dependency: " + d)
		}
	}
	for _, md := range p.VersionMetadata.MakeDepends {
		if !rePkgVer.MatchString(md) {
			return util.NewInvalidArgumentErrorf("invalid make dependency: " + md)
		}
	}
	for _, p := range p.VersionMetadata.Provides {
		if !rePkgVer.MatchString(p) {
			return util.NewInvalidArgumentErrorf("invalid provides: " + p)
		}
	}
	for _, od := range p.VersionMetadata.OptDepends {
		if !reOptDep.MatchString(od) {
			return util.NewInvalidArgumentErrorf("invalid optional dependency: " + od)
		}
	}
	for _, bf := range p.VersionMetadata.Backup {
		if strings.HasPrefix(bf, "/") {
			return util.NewInvalidArgumentErrorf("backup file contains leading forward slash")
		}
	}
	return nil
}

// Create pacman package description file.
func (p *Package) Desc() string {
	entries := [40]string{
		"FILENAME", fmt.Sprintf("%s-%s-%s.pkg.tar.zst", p.Name, p.Version, p.FileMetadata.Arch),
		"NAME", p.Name,
		"BASE", p.VersionMetadata.Base,
		"VERSION", p.Version,
		"DESC", p.VersionMetadata.Description,
		"GROUPS", strings.Join(p.VersionMetadata.Groups, "\n"),
		"CSIZE", fmt.Sprintf("%d", p.FileMetadata.CompressedSize),
		"ISIZE", fmt.Sprintf("%d", p.FileMetadata.InstalledSize),
		"MD5SUM", p.FileMetadata.MD5,
		"SHA256SUM", p.FileMetadata.SHA256,
		"URL", p.VersionMetadata.ProjectURL,
		"LICENSE", strings.Join(p.VersionMetadata.License, "\n"),
		"ARCH", p.FileMetadata.Arch,
		"BUILDDATE", fmt.Sprintf("%d", p.FileMetadata.BuildDate),
		"PACKAGER", p.FileMetadata.Packager,
		"PROVIDES", strings.Join(p.VersionMetadata.Provides, "\n"),
		"DEPENDS", strings.Join(p.VersionMetadata.Depends, "\n"),
		"OPTDEPENDS", strings.Join(p.VersionMetadata.OptDepends, "\n"),
		"MAKEDEPENDS", strings.Join(p.VersionMetadata.MakeDepends, "\n"),
		"CHECKDEPENDS", strings.Join(p.VersionMetadata.CheckDepends, "\n"),
	}

	var result string
	for i := 0; i < 40; i += 2 {
		if entries[i+1] != "" {
			result += fmt.Sprintf("%%%s%%\n%s\n\n", entries[i], entries[i+1])
		}
	}
	return result
}

// Create pacman database archive based on provided package metadata structs.
func CreatePacmanDb(entries map[string][]byte) (*bytes.Buffer, error) {
	var b bytes.Buffer

	gw := gzip.NewWriter(&b)
	tw := tar.NewWriter(gw)

	for name, content := range entries {
		header := &tar.Header{
			Name: name,
			Size: int64(len(content)),
			Mode: int64(os.ModePerm),
		}

		if err := tw.WriteHeader(header); err != nil {
			return nil, errors.Join(err, tw.Close(), gw.Close())
		}

		if _, err := tw.Write(content); err != nil {
			return nil, errors.Join(err, tw.Close(), gw.Close())
		}
	}

	return &b, errors.Join(tw.Close(), gw.Close())
}
