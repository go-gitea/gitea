// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/validation"

	"github.com/mholt/archiver/v3"
)

// Arch Linux Packages
// https://man.archlinux.org/man/PKGBUILD.5

const (
	PropertyDescription  = "arch.description"
	PropertyArch         = "arch.architecture"
	PropertyDistribution = "arch.distribution"

	SettingKeyPrivate = "arch.key.private"
	SettingKeyPublic  = "arch.key.public"

	RepositoryPackage = "_arch"
	RepositoryVersion = "_repository"
)

var (
	reName   = regexp.MustCompile(`^[a-zA-Z0-9@._+-]+$`)
	reVer    = regexp.MustCompile(`^[a-zA-Z0-9:_.+]+-+[0-9]+$`)
	reOptDep = regexp.MustCompile(`^[a-zA-Z0-9@._+-]+([<>]?=?([0-9]+:)?[a-zA-Z0-9@._+-]+)?(:.*)?$`)
	rePkgVer = regexp.MustCompile(`^[a-zA-Z0-9@._+-]+([<>]?=?([0-9]+:)?[a-zA-Z0-9@._+-]+)?$`)

	maxMagicLength = 0
	magics         = map[string]struct {
		magic    []byte
		archiver func() archiver.Reader
	}{
		"zst": {
			magic: []byte{0x28, 0xB5, 0x2F, 0xFD},
			archiver: func() archiver.Reader {
				return archiver.NewTarZstd()
			},
		},
		"xz": {
			magic: []byte{0xFD, 0x37, 0x7A, 0x58, 0x5A},
			archiver: func() archiver.Reader {
				return archiver.NewTarXz()
			},
		},
		"gz": {
			magic: []byte{0x1F, 0x8B},
			archiver: func() archiver.Reader {
				return archiver.NewTarGz()
			},
		},
	}
)

func init() {
	for _, i := range magics {
		if nLen := len(i.magic); nLen > maxMagicLength {
			maxMagicLength = nLen
		}
	}
}

type Package struct {
	Name            string `json:"name"`
	Version         string `json:"version"` // Includes version, release and epoch
	CompressType    string `json:"compress_type"`
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
	Conflicts    []string `json:"conflicts,omitempty"`
	Replaces     []string `json:"replaces,omitempty"`
	Backup       []string `json:"backup,omitempty"`
	XData        []string `json:"xdata,omitempty"`
}

// FileMetadata Metadata related to specific package file.
// This metadata might vary for different architecture and distribution.
type FileMetadata struct {
	CompressedSize int64  `json:"compressed_size"`
	InstalledSize  int64  `json:"installed_size"`
	MD5            string `json:"md5"`
	SHA256         string `json:"sha256"`
	BuildDate      int64  `json:"build_date"`
	Packager       string `json:"packager"`
	Arch           string `json:"arch"`
	PgpSigned      string `json:"pgp"`
}

// ParsePackage Function that receives arch package archive data and returns it's metadata.
func ParsePackage(r *packages.HashedBuffer) (*Package, error) {
	md5, _, sha256, _ := r.Sums()
	_, err := r.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	header := make([]byte, maxMagicLength)
	_, err = r.Read(header)
	if err != nil {
		return nil, err
	}
	_, err = r.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	var tarball archiver.Reader
	var tarballType string
	for tarType, info := range magics {
		if bytes.Equal(header[:len(info.magic)], info.magic) {
			tarballType = tarType
			tarball = info.archiver()
			break
		}
	}
	if tarballType == "" || tarball == nil {
		return nil, errors.New("not supported compression")
	}
	err = tarball.Open(r, 0)
	if err != nil {
		return nil, err
	}
	defer tarball.Close()

	var pkg *Package
	var mtree bool

	for {
		f, err := tarball.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch f.Name() {
		case ".PKGINFO":
			pkg, err = ParsePackageInfo(tarballType, f)
			if err != nil {
				_ = f.Close()
				return nil, err
			}
		case ".MTREE":
			mtree = true
		}
		_ = f.Close()
	}

	if pkg == nil {
		return nil, util.NewInvalidArgumentErrorf(".PKGINFO file not found")
	}

	if !mtree {
		return nil, util.NewInvalidArgumentErrorf(".MTREE file not found")
	}

	pkg.FileMetadata.CompressedSize = r.Size()
	pkg.FileMetadata.MD5 = hex.EncodeToString(md5)
	pkg.FileMetadata.SHA256 = hex.EncodeToString(sha256)

	return pkg, nil
}

// ParsePackageInfo Function that accepts reader for .PKGINFO file from package archive,
// validates all field according to PKGBUILD spec and returns package.
func ParsePackageInfo(compressType string, r io.Reader) (*Package, error) {
	p := &Package{
		CompressType: compressType,
	}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "#") {
			continue
		}

		key, value, find := strings.Cut(line, "=")
		if !find {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
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
		case "conflict":
			p.VersionMetadata.Conflicts = append(p.VersionMetadata.Conflicts, value)
		case "replaces":
			p.VersionMetadata.Replaces = append(p.VersionMetadata.Replaces, value)
		case "xdata":
			p.VersionMetadata.XData = append(p.VersionMetadata.XData, value)
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
		default:
			return nil, util.NewInvalidArgumentErrorf("property is not supported %s", key)
		}
	}

	return p, errors.Join(scanner.Err(), ValidatePackageSpec(p))
}

// ValidatePackageSpec Arch package validation according to PKGBUILD specification.
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
	for _, checkDepend := range p.VersionMetadata.CheckDepends {
		if !rePkgVer.MatchString(checkDepend) {
			return util.NewInvalidArgumentErrorf("invalid check dependency: %s", checkDepend)
		}
	}
	for _, depend := range p.VersionMetadata.Depends {
		if !rePkgVer.MatchString(depend) {
			return util.NewInvalidArgumentErrorf("invalid dependency: %s", depend)
		}
	}
	for _, makeDepend := range p.VersionMetadata.MakeDepends {
		if !rePkgVer.MatchString(makeDepend) {
			return util.NewInvalidArgumentErrorf("invalid make dependency: %s ", makeDepend)
		}
	}
	for _, provide := range p.VersionMetadata.Provides {
		if !rePkgVer.MatchString(provide) {
			return util.NewInvalidArgumentErrorf("invalid provides: %s", provide)
		}
	}
	for _, conflict := range p.VersionMetadata.Conflicts {
		if !rePkgVer.MatchString(conflict) {
			return util.NewInvalidArgumentErrorf("invalid conflicts: %s", conflict)
		}
	}
	for _, replace := range p.VersionMetadata.Replaces {
		if !rePkgVer.MatchString(replace) {
			return util.NewInvalidArgumentErrorf("invalid replaces: %s", replace)
		}
	}
	for _, optDepend := range p.VersionMetadata.OptDepends {
		if !reOptDep.MatchString(optDepend) {
			return util.NewInvalidArgumentErrorf("invalid optional dependency: %s", optDepend)
		}
	}
	for _, backup := range p.VersionMetadata.Backup {
		if strings.HasPrefix(backup, "/") {
			return util.NewInvalidArgumentErrorf("backup file contains leading forward slash")
		}
	}
	return nil
}

// Desc Create pacman package description file.
func (p *Package) Desc() string {
	entries := []string{
		"FILENAME", fmt.Sprintf("%s-%s-%s.pkg.tar.%s", p.Name, p.Version, p.FileMetadata.Arch, p.CompressType),
		"NAME", p.Name,
		"BASE", p.VersionMetadata.Base,
		"VERSION", p.Version,
		"DESC", p.VersionMetadata.Description,
		"GROUPS", strings.Join(p.VersionMetadata.Groups, "\n"),
		"CSIZE", fmt.Sprintf("%d", p.FileMetadata.CompressedSize),
		"ISIZE", fmt.Sprintf("%d", p.FileMetadata.InstalledSize),
		"MD5SUM", p.FileMetadata.MD5,
		"SHA256SUM", p.FileMetadata.SHA256,
		"PGPSIG", p.FileMetadata.PgpSigned,
		"URL", p.VersionMetadata.ProjectURL,
		"LICENSE", strings.Join(p.VersionMetadata.License, "\n"),
		"ARCH", p.FileMetadata.Arch,
		"BUILDDATE", fmt.Sprintf("%d", p.FileMetadata.BuildDate),
		"PACKAGER", p.FileMetadata.Packager,
		"REPLACES", strings.Join(p.VersionMetadata.Replaces, "\n"),
		"CONFLICTS", strings.Join(p.VersionMetadata.Conflicts, "\n"),
		"PROVIDES", strings.Join(p.VersionMetadata.Provides, "\n"),
		"DEPENDS", strings.Join(p.VersionMetadata.Depends, "\n"),
		"OPTDEPENDS", strings.Join(p.VersionMetadata.OptDepends, "\n"),
		"MAKEDEPENDS", strings.Join(p.VersionMetadata.MakeDepends, "\n"),
		"CHECKDEPENDS", strings.Join(p.VersionMetadata.CheckDepends, "\n"),
	}

	var buf bytes.Buffer
	for i := 0; i < len(entries); i += 2 {
		if entries[i+1] != "" {
			_, _ = fmt.Fprintf(&buf, "%%%s%%\n%s\n\n", entries[i], entries[i+1])
		}
	}
	return buf.String()
}
