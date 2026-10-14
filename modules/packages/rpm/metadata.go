// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rpm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
	"gitea.dev/modules/validation"

	"github.com/sassoftware/go-rpmutils"
)

const (
	PropertyMetadata     = "rpm.metadata"
	PropertyGroup        = "rpm.group"
	PropertyArchitecture = "rpm.architecture"

	SettingKeyPrivate = "rpm.key.private"
	SettingKeyPublic  = "rpm.key.public"

	RepositoryPackage = "_rpm"
	RepositoryVersion = "_repository"
)

const (
	// Can't use the syscall constants because they are not available for windows build.
	sIFMT  = 0xf000
	sIFDIR = 0x4000
	sIXUSR = 0x40
	sIXGRP = 0x8
	sIXOTH = 0x1
)

// caps far above any legitimate package, but low enough to stop metadata amplification
const (
	maxHeaderIndexEntries = 0xffff
	maxHeaderSize         = 64 * 1024 * 1024
	maxDependencyEntries  = 10000
	maxChangelogEntries   = 10000
	maxFileEntries        = 100000
)

var (
	ErrInvalidHeader  = util.NewInvalidArgumentErrorf("package header is invalid")
	ErrTooManyEntries = util.NewInvalidArgumentErrorf("package header contains too many entries")
)

// https://rpm-software-management.github.io/rpm/manual/spec.html
// https://refspecs.linuxbase.org/LSB_3.1.0/LSB-Core-generic/LSB-Core-generic/pkgformat.html

type Package struct {
	Name            string
	Version         string
	VersionMetadata *VersionMetadata
	FileMetadata    *FileMetadata
}

type VersionMetadata struct {
	License     string    `json:"license,omitempty"`
	ProjectURL  string    `json:"project_url,omitempty"`
	Summary     string    `json:"summary,omitempty"`
	Description string    `json:"description,omitempty"`
	Updates     []*Update `json:"updates,omitempty"`
}

type FileMetadata struct {
	Architecture  string `json:"architecture,omitempty"`
	Epoch         string `json:"epoch,omitempty"`
	Version       string `json:"version,omitempty"`
	Release       string `json:"release,omitempty"`
	Vendor        string `json:"vendor,omitempty"`
	Group         string `json:"group,omitempty"`
	Packager      string `json:"packager,omitempty"`
	SourceRpm     string `json:"source_rpm,omitempty"`
	BuildHost     string `json:"build_host,omitempty"`
	BuildTime     uint64 `json:"build_time,omitempty"`
	FileTime      uint64 `json:"file_time,omitempty"`
	InstalledSize uint64 `json:"installed_size,omitempty"`
	ArchiveSize   uint64 `json:"archive_size,omitempty"`

	Provides  []*Entry `json:"provide,omitempty"`
	Requires  []*Entry `json:"require,omitempty"`
	Conflicts []*Entry `json:"conflict,omitempty"`
	Obsoletes []*Entry `json:"obsolete,omitempty"`

	Files []*File `json:"files,omitempty"`

	Changelogs []*Changelog `json:"changelogs,omitempty"`
}

type Entry struct {
	Name    string `json:"name" xml:"name,attr"`
	Flags   string `json:"flags,omitempty" xml:"flags,attr,omitempty"`
	Version string `json:"version,omitempty" xml:"ver,attr,omitempty"`
	Epoch   string `json:"epoch,omitempty" xml:"epoch,attr,omitempty"`
	Release string `json:"release,omitempty" xml:"rel,attr,omitempty"`
}

type File struct {
	Path         string `json:"path" xml:",chardata"`
	Type         string `json:"type,omitempty" xml:"type,attr,omitempty"`
	IsExecutable bool   `json:"is_executable" xml:"-"`
}

type Changelog struct {
	Author string             `json:"author,omitempty" xml:"author,attr"`
	Date   timeutil.TimeStamp `json:"date,omitempty" xml:"date,attr"`
	Text   string             `json:"text,omitempty" xml:",chardata"`
}

// CheckHeaderSizes rejects RPM header size declarations which are too large or exceed the available data,
// because go-rpmutils allocates the declared sizes before reading. The returned reader yields the whole package.
func CheckHeaderSizes(r io.Reader) (io.Reader, error) {
	var buf bytes.Buffer
	if _, err := io.CopyN(&buf, r, 96); err != nil {
		return nil, ErrInvalidHeader
	}
	for _, isSignature := range []bool{true, false} {
		var intro [16]byte
		if _, err := io.ReadFull(r, intro[:]); err != nil {
			return nil, ErrInvalidHeader
		}
		buf.Write(intro[:])

		entries := int64(binary.BigEndian.Uint32(intro[8:12]))
		dataSize := int64(binary.BigEndian.Uint32(intro[12:16]))
		if isSignature {
			dataSize = (dataSize + 7) / 8 * 8 // the signature header is padded to 8 bytes
		}
		size := entries*16 + dataSize
		if entries > maxHeaderIndexEntries || size > maxHeaderSize {
			return nil, ErrInvalidHeader
		}
		// CopyN grows the buffer with the data actually read, not with the declared size
		if _, err := io.CopyN(&buf, r, size); err != nil {
			return nil, ErrInvalidHeader
		}
	}
	return io.MultiReader(&buf, r), nil
}

// ParsePackage parses the RPM package file
func ParsePackage(r io.Reader) (*Package, error) {
	r, err := CheckHeaderSizes(r)
	if err != nil {
		return nil, err
	}

	rpm, err := rpmutils.ReadRpm(r)
	if err != nil {
		return nil, err
	}

	nevra, err := rpm.Header.GetNEVRA()
	if err != nil {
		return nil, err
	}

	version := fmt.Sprintf("%s-%s", nevra.Version, nevra.Release)
	if nevra.Epoch != "" && nevra.Epoch != "0" {
		version = fmt.Sprintf("%s-%s", nevra.Epoch, version)
	}

	p := &Package{
		Name:    nevra.Name,
		Version: version,
		VersionMetadata: &VersionMetadata{
			Summary:     getString(rpm.Header, rpmutils.SUMMARY),
			Description: getString(rpm.Header, rpmutils.DESCRIPTION),
			License:     getString(rpm.Header, rpmutils.LICENSE),
			ProjectURL:  getString(rpm.Header, rpmutils.URL),
		},
		FileMetadata: &FileMetadata{
			Architecture:  nevra.Arch,
			Epoch:         nevra.Epoch,
			Version:       nevra.Version,
			Release:       nevra.Release,
			Vendor:        getString(rpm.Header, rpmutils.VENDOR),
			Group:         getString(rpm.Header, rpmutils.GROUP),
			Packager:      getString(rpm.Header, rpmutils.PACKAGER),
			SourceRpm:     getString(rpm.Header, rpmutils.SOURCERPM),
			BuildHost:     getString(rpm.Header, rpmutils.BUILDHOST),
			BuildTime:     getUInt64(rpm.Header, rpmutils.BUILDTIME),
			FileTime:      getUInt64(rpm.Header, rpmutils.FILEMTIMES),
			InstalledSize: getUInt64(rpm.Header, rpmutils.SIZE),
			ArchiveSize:   getUInt64(rpm.Header, rpmutils.SIG_PAYLOADSIZE),
		},
	}

	fm := p.FileMetadata
	if fm.Provides, err = getEntries(rpm.Header, rpmutils.PROVIDENAME, rpmutils.PROVIDEVERSION, rpmutils.PROVIDEFLAGS); err != nil {
		return nil, err
	}
	if fm.Requires, err = getEntries(rpm.Header, rpmutils.REQUIRENAME, rpmutils.REQUIREVERSION, rpmutils.REQUIREFLAGS); err != nil {
		return nil, err
	}
	if fm.Conflicts, err = getEntries(rpm.Header, rpmutils.CONFLICTNAME, rpmutils.CONFLICTVERSION, rpmutils.CONFLICTFLAGS); err != nil {
		return nil, err
	}
	if fm.Obsoletes, err = getEntries(rpm.Header, rpmutils.OBSOLETENAME, rpmutils.OBSOLETEVERSION, rpmutils.OBSOLETEFLAGS); err != nil {
		return nil, err
	}
	if fm.Files, err = getFiles(rpm.Header); err != nil {
		return nil, err
	}
	if fm.Changelogs, err = getChangelogs(rpm.Header); err != nil {
		return nil, err
	}

	if !validation.IsValidURL(p.VersionMetadata.ProjectURL) {
		p.VersionMetadata.ProjectURL = ""
	}

	return p, nil
}

func getString(h *rpmutils.RpmHeader, tag int) string {
	values, err := h.GetStrings(tag)
	if err != nil || len(values) < 1 {
		return ""
	}
	return values[0]
}

func getUInt64(h *rpmutils.RpmHeader, tag int) uint64 {
	values, err := h.GetUint64s(tag)
	if err != nil || len(values) < 1 {
		return 0
	}
	return values[0]
}

func getEntries(h *rpmutils.RpmHeader, namesTag, versionsTag, flagsTag int) ([]*Entry, error) {
	names, err := h.GetStrings(namesTag)
	if err != nil || len(names) == 0 {
		return nil, nil
	}
	if len(names) > maxDependencyEntries {
		return nil, ErrTooManyEntries
	}
	flags, err := h.GetUint64s(flagsTag)
	if err != nil || len(flags) == 0 {
		return nil, nil
	}
	versions, err := h.GetStrings(versionsTag)
	if err != nil || len(versions) == 0 {
		return nil, nil
	}
	if len(names) != len(flags) || len(names) != len(versions) {
		return nil, nil
	}

	entries := make([]*Entry, 0, len(names))
	for i := range names {
		e := &Entry{
			Name: names[i],
		}

		flags := flags[i]
		if (flags&rpmutils.RPMSENSE_GREATER) != 0 && (flags&rpmutils.RPMSENSE_EQUAL) != 0 {
			e.Flags = "GE"
		} else if (flags&rpmutils.RPMSENSE_LESS) != 0 && (flags&rpmutils.RPMSENSE_EQUAL) != 0 {
			e.Flags = "LE"
		} else if (flags & rpmutils.RPMSENSE_GREATER) != 0 {
			e.Flags = "GT"
		} else if (flags & rpmutils.RPMSENSE_LESS) != 0 {
			e.Flags = "LT"
		} else if (flags & rpmutils.RPMSENSE_EQUAL) != 0 {
			e.Flags = "EQ"
		}

		version := versions[i]
		if version != "" {
			parts := strings.Split(version, "-")

			versionParts := strings.Split(parts[0], ":")
			if len(versionParts) == 2 {
				e.Version = versionParts[1]
				e.Epoch = versionParts[0]
			} else {
				e.Version = versionParts[0]
				e.Epoch = "0"
			}

			if len(parts) > 1 {
				e.Release = parts[1]
			}
		}

		entries = append(entries, e)
	}
	return entries, nil
}

func getFiles(h *rpmutils.RpmHeader) ([]*File, error) {
	baseNames, _ := h.GetStrings(rpmutils.BASENAMES)
	if len(baseNames) > maxFileEntries {
		return nil, ErrTooManyEntries
	}
	dirNames, _ := h.GetStrings(rpmutils.DIRNAMES)
	dirIndexes, _ := h.GetUint32s(rpmutils.DIRINDEXES)
	fileFlags, _ := h.GetUint32s(rpmutils.FILEFLAGS)
	fileModes, _ := h.GetUint32s(rpmutils.FILEMODES)

	files := make([]*File, 0, len(baseNames))
	for i := range baseNames {
		if len(dirIndexes) <= i {
			continue
		}
		dirIndex := dirIndexes[i]
		if len(dirNames) <= int(dirIndex) {
			continue
		}

		var fileType string
		var isExecutable bool
		if i < len(fileFlags) && (fileFlags[i]&rpmutils.RPMFILE_GHOST) != 0 {
			fileType = "ghost"
		} else if i < len(fileModes) {
			if (fileModes[i] & sIFMT) == sIFDIR {
				fileType = "dir"
			} else {
				mode := fileModes[i] & ^uint32(sIFMT)
				isExecutable = (mode&sIXUSR) != 0 || (mode&sIXGRP) != 0 || (mode&sIXOTH) != 0
			}
		}

		files = append(files, &File{
			Path:         dirNames[dirIndex] + baseNames[i],
			Type:         fileType,
			IsExecutable: isExecutable,
		})
	}

	return files, nil
}

func getChangelogs(h *rpmutils.RpmHeader) ([]*Changelog, error) {
	texts, err := h.GetStrings(rpmutils.CHANGELOGTEXT)
	if err != nil || len(texts) == 0 {
		return nil, nil
	}
	if len(texts) > maxChangelogEntries {
		return nil, ErrTooManyEntries
	}
	authors, err := h.GetStrings(rpmutils.CHANGELOGNAME)
	if err != nil || len(authors) == 0 {
		return nil, nil
	}
	times, err := h.GetUint32s(rpmutils.CHANGELOGTIME)
	if err != nil || len(times) == 0 {
		return nil, nil
	}
	if len(texts) != len(authors) || len(texts) != len(times) {
		return nil, nil
	}

	changelogs := make([]*Changelog, 0, len(texts))
	for i := range texts {
		changelogs = append(changelogs, &Changelog{
			Author: authors[i],
			Date:   timeutil.TimeStamp(times[i]),
			Text:   texts[i],
		})
	}
	return changelogs, nil
}

type DateAttr struct {
	Date string `xml:"date,attr" json:"date"`
}

type Update struct {
	From        string        `xml:"from,attr" json:"from"`
	Status      string        `xml:"status,attr" json:"status"`
	Type        string        `xml:"type,attr" json:"type"`
	Version     string        `xml:"version,attr" json:"version"`
	ID          string        `xml:"id" json:"id"`
	Title       string        `xml:"title" json:"title"`
	Severity    string        `xml:"severity" json:"severity"`
	Description string        `xml:"description" json:"description"`
	Issued      *DateAttr     `xml:"issued" json:"issued"`
	Updated     *DateAttr     `xml:"updated" json:"updated"`
	References  []*Reference  `xml:"references>reference" json:"references"`
	PkgList     []*Collection `xml:"pkglist>collection" json:"pkg_list"`
}

type Reference struct {
	Href  string `xml:"href,attr" json:"href"`
	ID    string `xml:"id,attr" json:"id"`
	Title string `xml:"title,attr" json:"title"`
	Type  string `xml:"type,attr" json:"type"`
}

type Collection struct {
	Short    string           `xml:"short,attr" json:"short"`
	Packages []*UpdatePackage `xml:"package" json:"packages"`
}

type UpdatePackage struct {
	Arch     string `xml:"arch,attr" json:"arch"`
	Name     string `xml:"name,attr" json:"name"`
	Release  string `xml:"release,attr" json:"release"`
	Src      string `xml:"src,attr" json:"src"`
	Version  string `xml:"version,attr" json:"version"`
	Filename string `xml:"filename" json:"filename"`
}
