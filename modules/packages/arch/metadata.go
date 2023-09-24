// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	pkg_module "code.gitea.io/gitea/modules/packages"

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
	OptDepends   []string `json:"opt-depends,omitempty"`
	MakeDepends  []string `json:"make-depends,omitempty"`
	CheckDepends []string `json:"check-depends,omitempty"`
	Backup       []string `json:"backup,omitempty"`
}

// Package description file that will be saved as .desc file in object storage.
// This file will be used to create pacman database.
type DbDesc struct {
	Filename       string   `json:"filename"`
	Name           string   `json:"name"`
	Base           string   `json:"base"`
	Version        string   `json:"version"`
	Description    string   `json:"description"`
	CompressedSize int64    `json:"compressed-size"`
	InstalledSize  int64    `json:"installed-size"`
	MD5            string   `json:"md5"`
	SHA256         string   `json:"sha256"`
	URL            string   `json:"url"`
	BuildDate      int64    `json:"build-date"`
	Packager       string   `json:"packager"`
	Provides       []string `json:"provides,omitempty"`
	License        []string `json:"license,omitempty"`
	Arch           []string `json:"arch,omitempty"`
	Depends        []string `json:"depends,omitempty"`
	OptDepends     []string `json:"opt-depends,omitempty"`
	MakeDepends    []string `json:"make-depends,omitempty"`
	CheckDepends   []string `json:"check-depends,omitempty"`
	Backup         []string `json:"backup,omitempty"`
}

type EjectParams struct {
	Filename     string
	Distribution string
	Buffer       *pkg_module.HashedBuffer
}

// Function that receives arch package archive data and returns it's metadata.
func EjectMetadata(p *EjectParams) (*DbDesc, error) {
	pkginfo, err := getPkginfo(p.Buffer)
	if err != nil {
		return nil, err
	}

	// Add package blob parameters to arch related desc.
	hashMD5, _, hashSHA256, _ := p.Buffer.Sums()
	md := DbDesc{
		Filename:       p.Filename,
		Name:           p.Filename,
		CompressedSize: p.Buffer.Size(),
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
			md.URL = value
		case "packager":
			md.Packager = value
		case "builddate":
			num, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, err
			}
			md.BuildDate = num
		case "size":
			num, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, err
			}
			md.InstalledSize = num
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
	return strings.Join(rmEmptyStrings([]string{
		formatField("FILENAME", m.Filename),
		formatField("NAME", m.Name),
		formatField("BASE", m.Base),
		formatField("VERSION", m.Version),
		formatField("DESC", m.Description),
		formatField("CSIZE", m.CompressedSize),
		formatField("ISIZE", m.InstalledSize),
		formatField("MD5SUM", m.MD5),
		formatField("SHA256SUM", m.SHA256),
		formatField("URL", m.URL),
		formatField("LICENSE", m.License),
		formatField("ARCH", m.Arch),
		formatField("BUILDDATE", m.BuildDate),
		formatField("PACKAGER", m.Packager),
		formatField("PROVIDES", m.Provides),
		formatField("DEPENDS", m.Depends),
		formatField("OPTDEPENDS", m.OptDepends),
		formatField("MAKEDEPENDS", m.MakeDepends),
		formatField("CHECKDEPENDS", m.CheckDepends),
	}), "\n\n") + "\n\n"
}

func formatField(field string, value any) string {
	switch value := value.(type) {
	case []string:
		if value == nil {
			return ``
		}
		val := strings.Join(value, "\n")
		return fmt.Sprintf("%%%s%%\n%s", field, val)
	case string:
		return fmt.Sprintf("%%%s%%\n%s", field, value)
	case int64:
		return fmt.Sprintf("%%%s%%\n%d", field, value)
	}
	return ``
}

func rmEmptyStrings(s []string) []string {
	var r []string
	for _, str := range s {
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}

// Create pacman database archive based on provided package metadata structs.
func CreatePacmanDb(entries map[string][]byte) ([]byte, error) {
	var out bytes.Buffer

	gw := gzip.NewWriter(&out)
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

	return out.Bytes(), nil
}
