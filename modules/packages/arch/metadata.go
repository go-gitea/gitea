// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	packages_module "code.gitea.io/gitea/modules/packages"

	"github.com/mholt/archiver/v3"
)

// JSON with pacakage parameters that are not related to specific
// architecture/distribution that will be stored in sql database.
type Metadata struct {
	URL          string   `json:"url"`
	Description  string   `json:"description"`
	Provides     []string `json:"provides,omitempty"`
	License      []string `json:"license,omitempty"`
	Depends      []string `json:"depends,omitempty"`
	OptDepends   []string `json:"opt_depends,omitempty"`
	MakeDepends  []string `json:"make_depends,omitempty"`
	CheckDepends []string `json:"check_depends,omitempty"`
}

// Package description file that will be saved as .desc file in object storage.
// This file will be used to create pacman database.
type DbDesc struct {
	Filename       string   `json:"filename"`
	Name           string   `json:"name"`
	Base           string   `json:"base"`
	Version        string   `json:"version"`
	Description    string   `json:"description"`
	CompressedSize int64    `json:"compressed_size"`
	InstalledSize  int64    `json:"installed_size"`
	MD5            string   `json:"md5"`
	SHA256         string   `json:"sha256"`
	ProjectURL     string   `json:"project_url"`
	BuildDate      int64    `json:"build_date"`
	Packager       string   `json:"packager"`
	Provides       []string `json:"provides,omitempty"`
	License        []string `json:"license,omitempty"`
	Arch           []string `json:"arch,omitempty"`
	Depends        []string `json:"depends,omitempty"`
	OptDepends     []string `json:"opt_depends,omitempty"`
	MakeDepends    []string `json:"make_depends,omitempty"`
	CheckDepends   []string `json:"check_depends,omitempty"`
	Backup         []string `json:"backup,omitempty"`
}

// Function that receives arch package archive data and returns it's metadata.
func ParseMetadata(file, distro string, b *packages_module.HashedBuffer) (*DbDesc, error) {
	pkginfo, err := getPkginfo(b)
	if err != nil {
		return nil, err
	}

	// Add package blob parameters to arch related desc.
	hashMD5, _, hashSHA256, _ := b.Sums()
	md := DbDesc{
		Filename:       file,
		Name:           file,
		CompressedSize: b.Size(),
		MD5:            hex.EncodeToString(hashMD5),
		SHA256:         hex.EncodeToString(hashSHA256),
	}

	for _, line := range strings.Split(pkginfo, "\n") {
		splt := strings.Split(line, " = ")
		if len(splt) != 2 {
			continue
		}
		var (
			parameter = splt[0]
			value     = splt[1]
		)

		switch parameter {
		case "pkgname":
			md.Name = value
		case "pkgbase":
			md.Base = value
		case "pkgver":
			md.Version = value
		case "pkgdesc":
			md.Description = value
		case "url":
			md.ProjectURL = value
		case "packager":
			md.Packager = value
		case "provides":
			md.Provides = append(md.Provides, value)
		case "license":
			md.License = append(md.License, value)
		case "arch":
			md.Arch = append(md.Arch, value)
		case "depend":
			md.Depends = append(md.Depends, value)
		case "optdepend":
			md.OptDepends = append(md.OptDepends, value)
		case "makedepend":
			md.MakeDepends = append(md.MakeDepends, value)
		case "checkdepend":
			md.CheckDepends = append(md.CheckDepends, value)
		case "backup":
			md.Backup = append(md.Backup, value)
		case "builddate":
			md.BuildDate, err = strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, err
			}
		case "size":
			md.InstalledSize, err = strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, err
			}
		}
	}

	return &md, nil
}

// Eject .PKGINFO file as string from package archive.
func getPkginfo(data io.Reader) (string, error) {
	br := bufio.NewReader(data)
	zstd := archiver.NewTarZstd()
	err := zstd.Open(br, int64(250000))
	if err != nil {
		return ``, err
	}
	for {
		f, err := zstd.Read()
		if err != nil {
			return ``, err
		}
		if f.Name() != ".PKGINFO" {
			continue
		}
		b, err := io.ReadAll(f)
		if err != nil {
			return ``, err
		}
		return string(b), nil
	}
}

// Create pacman package description file.
func (m *DbDesc) String() string {
	entries := []struct{ key, value string }{
		{"FILENAME", m.Filename},
		{"NAME", m.Name},
		{"BASE", m.Base},
		{"VERSION", m.Version},
		{"DESC", m.Description},
		{"CSIZE", fmt.Sprintf("%d", m.CompressedSize)},
		{"ISIZE", fmt.Sprintf("%d", m.InstalledSize)},
		{"MD5SUM", m.MD5},
		{"SHA256SUM", m.SHA256},
		{"URL", m.ProjectURL},
		{"LICENSE", strings.Join(m.License, "\n")},
		{"ARCH", strings.Join(m.Arch, "\n")},
		{"BUILDDATE", fmt.Sprintf("%d", m.BuildDate)},
		{"PACKAGER", m.Packager},
		{"PROVIDES", strings.Join(m.Provides, "\n")},
		{"DEPENDS", strings.Join(m.Depends, "\n")},
		{"OPTDEPENDS", strings.Join(m.OptDepends, "\n")},
		{"MAKEDEPENDS", strings.Join(m.MakeDepends, "\n")},
		{"CHECKDEPENDS", strings.Join(m.CheckDepends, "\n")},
	}

	var result string
	for _, e := range entries {
		if e.value != "" {
			result += fmt.Sprintf("%%%s%%\n%s\n\n", e.key, e.value)
		}
	}
	return result
}

// Create pacman database archive based on provided package metadata structs.
func CreatePacmanDb(entries map[string][]byte) (io.ReadSeeker, error) {
	out, err := packages_module.NewHashedBuffer()
	if err != nil {
		return nil, err
	}

	gw := gzip.NewWriter(out)
	tw := tar.NewWriter(gw)

	for name, content := range entries {
		header := &tar.Header{
			Name: name,
			Size: int64(len(content)),
			Mode: int64(os.ModePerm),
		}

		if err := tw.WriteHeader(header); err != nil {
			tw.Close()
			gw.Close()
			return nil, err
		}

		if _, err := tw.Write(content); err != nil {
			tw.Close()
			gw.Close()
			return nil, err
		}
	}

	tw.Close()
	gw.Close()

	return out, nil
}
