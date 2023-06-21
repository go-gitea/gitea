// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
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

// Function takes path to directory with pacman database and updates package
// it with current metadata.
func (m *Metadata) PutToDb(dir string, mode fs.FileMode) error {
	descdir := path.Join(dir, m.Name+"-"+m.Version)
	err := os.MkdirAll(descdir, mode)
	if err != nil {
		return err
	}
	return os.WriteFile(path.Join(descdir, "desc"), []byte(m.GetDbDesc()), mode)
}

// Function takes raw database archive bytes and destination directory as
// arguements and unpacks database contents to destination directory.
func UnpackDb(src, dst string) error {
	return archiver.DefaultTarGz.Unarchive(src, dst)
}

// Function takes path to source directory with raw pacman description files
// for pacman database, creates db.tar.gz archive and related symlink for
// provided path.
func PackDb(src, dst string) error {
	if !strings.HasSuffix(dst, ".db.tar.gz") {
		return fmt.Errorf("dst should end with '.db.tar.gz': %s", dst)
	}
	symlink := strings.TrimSuffix(dst, ".tar.gz")
	if _, err := os.Stat(dst); err == nil {
		err = os.RemoveAll(dst)
		if err != nil {
			return err
		}
		err = os.RemoveAll(symlink)
		if err != nil {
			return err
		}
	}
	des, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	var pkgdescs []string
	for _, de := range des {
		pkgdescs = append(pkgdescs, path.Join(src, de.Name()))
	}
	err = archiver.DefaultTarGz.Archive(pkgdescs, dst)
	if err != nil {
		return err
	}
	return os.Symlink(dst, symlink)
}
