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
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mholt/archiver/v3"
)

// Metadata for arch package.
type Metadata struct {
	Filename          string
	Name              string
	Base              string
	Version           string
	Description       string
	CompressedSize    int64
	CompressedSizeMib string
	InstalledSize     int64
	InstalledSizeMib  string
	MD5               string
	SHA256            string
	URL               string
	BuildDate         int64
	BuildDateStr      string
	BaseDomain        string
	Packager          string
	Provides          []string
	License           []string
	Arch              []string
	Depends           []string
	OptDepends        []string
	MakeDepends       []string
	CheckDepends      []string
	Backup            []string
}

// Function that recieves arch package archive data and returns it's metadata.
func EjectMetadata(filename, domain string, pkg []byte) (*Metadata, error) {
	pkgreader := io.LimitReader(bytes.NewReader(pkg), 250000)
	var buf bytes.Buffer
	err := archiver.DefaultZstd.Decompress(pkgreader, &buf)
	if err != nil {
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, err
		}
	}
	splt := strings.Split(buf.String(), "PKGINFO")
	if len(splt) < 2 {
		return nil, errors.New("unable to eject .PKGINFO from archive")
	}
	raw := splt[1][0:10000]
	inssize := int64(len(pkg))
	compsize := ejectInt64(raw, "size")
	unixbuilddate := ejectInt64(raw, "builddate")
	return &Metadata{
		Filename:          filename,
		Name:              ejectString(raw, "pkgname"),
		Base:              ejectString(raw, "pkgbase"),
		Version:           ejectString(raw, "pkgver"),
		Description:       ejectString(raw, "pkgdesc"),
		CompressedSize:    inssize,
		CompressedSizeMib: ByteCountSI(inssize),
		InstalledSize:     compsize,
		InstalledSizeMib:  ByteCountSI(compsize),
		MD5:               md5sum(pkg),
		SHA256:            sha256sum(pkg),
		URL:               ejectString(raw, "url"),
		BuildDate:         unixbuilddate,
		BuildDateStr:      ReadableTime(unixbuilddate),
		BaseDomain:        domain,
		Packager:          ejectString(raw, "packager"),
		Provides:          ejectStrings(raw, "provides"),
		License:           ejectStrings(raw, "license"),
		Arch:              ejectStrings(raw, "arch"),
		Depends:           ejectStrings(raw, "depend"),
		OptDepends:        ejectStrings(raw, "optdepend"),
		MakeDepends:       ejectStrings(raw, "makedepend"),
		CheckDepends:      ejectStrings(raw, "checkdepend"),
		Backup:            ejectStrings(raw, "backup"),
	}, nil
}

func ejectString(raw, field string) string {
	splitted := strings.Split(raw, "\n"+field+" = ")
	if len(splitted) < 2 {
		return ``
	}
	return strings.Split(splitted[1], "\n")[0]
}

func ejectStrings(raw, field string) []string {
	splitted := strings.Split(raw, "\n"+field+" = ")
	if len(splitted) < 2 {
		return nil
	}
	var rez []string
	for i, v := range splitted {
		if i == 0 {
			continue
		}
		rez = append(rez, strings.Split(v, "\n")[0])
	}
	return rez
}

func ejectInt64(raw, field string) int64 {
	splitted := strings.Split(raw, "\n"+field+" = ")
	if len(splitted) < 2 {
		return 0
	}
	i, err := strconv.ParseInt(strings.Split(splitted[1], "\n")[0], 10, 64)
	if err != nil {
		return 0
	}
	return i
}

func ByteCountSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

func ReadableTime(unix int64) string {
	return time.Unix(unix, 0).Format(time.DateTime)
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

	// Add new package entry to list.
	entries[md.Name+"-"+md.Version+"/desc"] = []byte(md.GetDbDesc())

	fmt.Println(entries)

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
		fmt.Println(err)
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
