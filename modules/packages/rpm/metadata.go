// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rpm

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/validation"
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

	senseLess    = 0x2
	senseGreater = 0x4
	senseEqual   = 0x8
	fileGhost    = 0x40
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

// ParsePackage parses the RPM package file
func ParsePackage(r io.Reader) (*Package, error) {
	_, sig, h, err := readHeaders(r)
	if err != nil {
		return nil, err
	}

	name, ver, rel, arch := h.getString(tagName), h.getString(tagVersion), h.getString(tagRelease), h.getString(tagArch)
	if name == "" || ver == "" || rel == "" || arch == "" {
		return nil, ErrInvalidPackage
	}
	epoch := strconv.FormatUint(h.getUint(tagEpoch), 10)

	version := fmt.Sprintf("%s-%s", ver, rel)
	if epoch != "0" {
		version = fmt.Sprintf("%s-%s", epoch, version)
	}

	p := &Package{
		Name:    name,
		Version: version,
		VersionMetadata: &VersionMetadata{
			Summary:     h.getString(tagSummary),
			Description: h.getString(tagDescription),
			License:     h.getString(tagLicense),
			ProjectURL:  h.getString(tagURL),
		},
		FileMetadata: &FileMetadata{
			Architecture:  arch,
			Epoch:         epoch,
			Version:       ver,
			Release:       rel,
			Vendor:        h.getString(tagVendor),
			Group:         h.getString(tagGroup),
			Packager:      h.getString(tagPackager),
			SourceRpm:     h.getString(tagSourceRpm),
			BuildHost:     h.getString(tagBuildHost),
			BuildTime:     h.getUint(tagBuildTime),
			FileTime:      h.getUint(tagFileMTimes),
			InstalledSize: h.getUint(tagSize),
			ArchiveSize:   sig.getUint(sigTagPayloadSize),

			Provides:   getEntries(h, tagProvideName, tagProvideVersion, tagProvideFlags),
			Requires:   getEntries(h, tagRequireName, tagRequireVersion, tagRequireFlags),
			Conflicts:  getEntries(h, tagConflictName, tagConflictVersion, tagConflictFlags),
			Obsoletes:  getEntries(h, tagObsoleteName, tagObsoleteVersion, tagObsoleteFlags),
			Files:      getFiles(h),
			Changelogs: getChangelogs(h),
		},
	}

	if !validation.IsValidURL(p.VersionMetadata.ProjectURL) {
		p.VersionMetadata.ProjectURL = ""
	}

	return p, nil
}

func getEntries(h *header, namesTag, versionsTag, flagsTag uint32) []*Entry {
	names, flags, versions := h.getStrings(namesTag), h.getUints(flagsTag), h.getStrings(versionsTag)
	if len(names) == 0 || len(names) != len(flags) || len(names) != len(versions) {
		return nil
	}

	entries := make([]*Entry, 0, len(names))
	for i := range names {
		e := &Entry{
			Name: names[i],
		}

		flags := flags[i]
		if (flags&senseGreater) != 0 && (flags&senseEqual) != 0 {
			e.Flags = "GE"
		} else if (flags&senseLess) != 0 && (flags&senseEqual) != 0 {
			e.Flags = "LE"
		} else if (flags & senseGreater) != 0 {
			e.Flags = "GT"
		} else if (flags & senseLess) != 0 {
			e.Flags = "LT"
		} else if (flags & senseEqual) != 0 {
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
	return entries
}

func getFiles(h *header) []*File {
	baseNames := h.getStrings(tagBaseNames)
	dirNames := h.getStrings(tagDirNames)
	dirIndexes := h.getUints(tagDirIndexes)
	fileFlags := h.getUints(tagFileFlags)
	fileModes := h.getUints(tagFileModes)

	files := make([]*File, 0, len(baseNames))
	for i := range baseNames {
		if i >= len(dirIndexes) || dirIndexes[i] >= uint64(len(dirNames)) {
			continue
		}
		dirIndex := dirIndexes[i]

		var fileType string
		var isExecutable bool
		if i < len(fileFlags) && (fileFlags[i]&fileGhost) != 0 {
			fileType = "ghost"
		} else if i < len(fileModes) {
			if (fileModes[i] & sIFMT) == sIFDIR {
				fileType = "dir"
			} else {
				mode := fileModes[i] &^ sIFMT
				isExecutable = (mode&sIXUSR) != 0 || (mode&sIXGRP) != 0 || (mode&sIXOTH) != 0
			}
		}

		files = append(files, &File{
			Path:         dirNames[dirIndex] + baseNames[i],
			Type:         fileType,
			IsExecutable: isExecutable,
		})
	}

	return files
}

func getChangelogs(h *header) []*Changelog {
	texts, authors, times := h.getStrings(tagChangelogText), h.getStrings(tagChangelogName), h.getUints(tagChangelogTime)
	if len(texts) == 0 || len(texts) != len(authors) || len(texts) != len(times) {
		return nil
	}

	changelogs := make([]*Changelog, 0, len(texts))
	for i := range texts {
		changelogs = append(changelogs, &Changelog{
			Author: authors[i],
			Date:   timeutil.TimeStamp(times[i]),
			Text:   texts[i],
		})
	}
	return changelogs
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
