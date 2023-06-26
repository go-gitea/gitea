// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mholt/archiver/v3"
)

// Metadata for arch package.
type Metadata struct {
	Filename       string
	Name           string
	Base           string
	Version        string
	Description    string
	CompressedSize int64
	InstalledSize  int64
	MD5            string
	SHA256         string
	URL            string
	BuildDate      int64
	BaseDomain     string
	Packager       string
	Provides       []string
	License        []string
	Arch           []string
	Depends        []string
	OptDepends     []string
	MakeDepends    []string
	CheckDepends   []string
	Backup         []string
}

// Function that recieves arch package archive data and returns it's metadata.
func EjectMetadata(filename, domain string, pkg []byte) (*Metadata, error) {
	pkginfo, err := getPkginfo(pkg)
	if err != nil {
		return nil, err
	}
	var md = Metadata{
		Filename:       filename,
		BaseDomain:     domain,
		CompressedSize: int64(len(pkg)),
		MD5:            md5sum(pkg),
		SHA256:         sha256sum(pkg),
	}
	for _, line := range strings.Split(pkginfo, "\n") {
		splt := strings.Split(line, " = ")
		if len(splt) != 2 {
			continue
		}
		switch splt[0] {
		case "pkgname":
			md.Name = splt[1]
		case "pkgbase":
			md.Base = splt[1]
		case "pkgver":
			md.Version = splt[1]
		case "pkgdesc":
			md.Description = splt[1]
		case "url":
			md.URL = splt[1]
		case "packager":
			md.Packager = splt[1]
		case "builddate":
			num, err := strconv.ParseInt(splt[1], 10, 64)
			if err != nil {
				return nil, err
			}
			md.BuildDate = num
		case "size":
			num, err := strconv.ParseInt(splt[1], 10, 64)
			if err != nil {
				return nil, err
			}
			md.InstalledSize = num
		case "provides":
			md.Provides = append(md.Provides, splt[1])
		case "license":
			md.License = append(md.License, splt[1])
		case "arch":
			md.Arch = append(md.Arch, splt[1])
		case "depend":
			md.Depends = append(md.Depends, splt[1])
		case "optdepend":
			md.Depends = append(md.OptDepends, splt[1])
		case "makedepend":
			md.Depends = append(md.MakeDepends, splt[1])
		case "checkdepend":
			md.Depends = append(md.CheckDepends, splt[1])
		case "backup":
			md.Depends = append(md.Backup, splt[1])
		}
	}
	return &md, nil
}

func getPkginfo(data []byte) (string, error) {
	pkgreader := io.LimitReader(bytes.NewReader(data), 250000)
	zstd := archiver.NewTarZstd()
	err := zstd.Open(pkgreader, int64(250000))
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

func md5sum(data []byte) string {
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])
}

func sha256sum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// This function returns pacman package description in unarchived raw database
// format.
func (m *Metadata) GetDbDesc() string {
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

// Join database or package names to prevent collisions with same packages in
// different user spaces. Skips empty strings and returns name joined with
// dots.
func Join(s ...string) string {
	rez := ""
	for i, v := range s {
		if v == "" {
			continue
		}
		if i+1 == len(s) {
			rez += v
			continue
		}
		rez += v + "."
	}
	return rez
}

// Add or update existing package entry in database archived data.
func UpdatePacmanDbEntry(db []byte, md *Metadata) ([]byte, error) {
	// Read existing entries in archive.
	entries, err := readEntries(db)
	if err != nil {
		return nil, err
	}

	// Remove entries related old package versions.
	entries = CleanOldEntries(entries, md.Name)

	// Add new package entry to list.
	entries[md.Name+"-"+md.Version+"/desc"] = []byte(md.GetDbDesc())

	var out bytes.Buffer

	// Write entries to new buffer and return it.
	err = writeToArchive(entries, &out)
	if err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// Clean entries for old package versions from pacman database.
func CleanOldEntries(entries map[string][]byte, pkg string) map[string][]byte {
	out := map[string][]byte{}
	for entry, value := range entries {
		splt := strings.Split(entry, "-")
		basename := strings.Join(splt[0:len(splt)-2], "-")
		if pkg != basename {
			out[entry] = value
		}
	}
	return out
}

// Add or update existing package entry in database archived data.
func RemoveDbEntry(db []byte, pkg, ver string) ([]byte, error) {
	// Read existing entries in archive.
	entries, err := readEntries(db)
	if err != nil {
		return nil, err
	}

	// Add new package entry to list.
	delete(entries, pkg+"-"+ver+"/desc")

	var out bytes.Buffer

	// Write entries to new buffer and return it.
	err = writeToArchive(entries, &out)
	if err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// Read database entries containing in pacman archive.
func readEntries(dbarchive []byte) (map[string][]byte, error) {
	gzf, err := gzip.NewReader(bytes.NewReader(dbarchive))
	if err != nil {
		return map[string][]byte{}, nil
	}

	var entries = map[string][]byte{}

	tarReader := tar.NewReader(gzf)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		if header.Typeflag == tar.TypeReg {
			content, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, err
			}
			entries[header.Name] = content
		}
	}
	return entries, nil
}

// Write pacman package entries to empty buffer.
func writeToArchive(files map[string][]byte, buf io.Writer) error {
	gw := gzip.NewWriter(buf)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Size: int64(len(content)),
			Mode: int64(os.ModePerm),
		}

		err := tw.WriteHeader(hdr)
		if err != nil {
			return err
		}

		_, err = io.Copy(tw, bytes.NewReader(content))
		if err != nil {
			return err
		}
	}
	return nil
}
