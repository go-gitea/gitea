// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package debian

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"io"
	"net/mail"
	"regexp"
	"strings"
	"sync"

	"gitea.dev/modules/util"
	"gitea.dev/modules/validation"
	"gitea.dev/modules/zstd"

	"github.com/blakesmith/ar"
	"github.com/ulikunitz/xz"
)

const (
	PropertyDistribution               = "debian.distribution"
	PropertyComponent                  = "debian.component"
	PropertyArchitecture               = "debian.architecture"
	PropertyControl                    = "debian.control"
	PropertyRepositoryIncludeInRelease = "debian.repository.include_in_release"

	SettingKeyPrivate = "debian.key.private"
	SettingKeyPublic  = "debian.key.public"

	RepositoryPackage = "_debian"
	RepositoryVersion = "_repository"

	controlTar = "control.tar"
)

var GlobalVars = sync.OnceValue(func() (ret struct {
	ErrMissingControlFile     error
	ErrUnsupportedCompression error
	ErrInvalidName            error
	ErrInvalidVersion         error
	ErrInvalidArchitecture    error

	namePattern    *regexp.Regexp
	versionPattern *regexp.Regexp
	symbolPattern  *regexp.Regexp
},
) {
	ret.ErrMissingControlFile = util.NewInvalidArgumentErrorf("control file is missing")
	ret.ErrUnsupportedCompression = util.NewInvalidArgumentErrorf("unsupported compression algorithm")
	ret.ErrInvalidName = util.NewInvalidArgumentErrorf("package name is invalid")
	ret.ErrInvalidVersion = util.NewInvalidArgumentErrorf("package version is invalid")
	ret.ErrInvalidArchitecture = util.NewInvalidArgumentErrorf("package architecture is invalid")

	// https://www.debian.org/doc/debian-policy/ch-controlfields.html#source
	ret.namePattern = regexp.MustCompile(`\A[a-z0-9][a-z0-9+-.]+\z`)
	// https://www.debian.org/doc/debian-policy/ch-controlfields.html#version
	ret.versionPattern = regexp.MustCompile(`\A(?:(0|[1-9][0-9]*):)?[a-zA-Z0-9.+~]+(?:-[a-zA-Z0-9.+-~]+)?\z`)

	// distribution and component are taken from the request path and written
	// verbatim into the generated line-based Release and Packages indices (and
	// into the pool/<distribution>/<component> paths referenced from them), so
	// they must be restricted to a character set that cannot break that format.
	ret.symbolPattern = regexp.MustCompile(`\A[a-zA-Z0-9][a-zA-Z0-9.~+_-]*\z`)
	return ret
})

type Package struct {
	Name         string
	Version      string
	Architecture string
	Control      string
	Metadata     *Metadata
}

type Metadata struct {
	Maintainer   string   `json:"maintainer,omitempty"`
	ProjectURL   string   `json:"project_url,omitempty"`
	Description  string   `json:"description,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
}

func IsValidDistributionOrComponent(s string) bool {
	return GlobalVars().symbolPattern.MatchString(s)
}

// ParsePackage parses the Debian package file
// https://manpages.debian.org/bullseye/dpkg-dev/deb.5.en.html
func ParsePackage(r io.Reader) (*Package, error) {
	arr := ar.NewReader(r)

	for {
		hd, err := arr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if strings.HasPrefix(hd.Name, controlTar) {
			var inner io.Reader
			// https://man7.org/linux/man-pages/man5/deb-split.5.html#FORMAT
			// The file names might contain a trailing slash (since dpkg 1.15.6).
			switch strings.TrimSuffix(hd.Name[len(controlTar):], "/") {
			case "":
				inner = arr
			case ".gz":
				gzr, err := gzip.NewReader(arr)
				if err != nil {
					return nil, err
				}
				defer gzr.Close()

				inner = gzr
			case ".xz":
				xzr, err := xz.NewReader(arr)
				if err != nil {
					return nil, err
				}

				inner = xzr
			case ".zst":
				zr, err := zstd.NewReader(arr)
				if err != nil {
					return nil, err
				}
				defer zr.Close()

				inner = zr
			default:
				return nil, GlobalVars().ErrUnsupportedCompression
			}

			// bound the decompressed control archive: it holds only the small control file
			// and maintainer scripts, so a much larger stream is a decompression bomb
			const maxControlTarSize = 32 * 1024 * 1024
			tr := tar.NewReader(io.LimitReader(inner, maxControlTarSize))
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

				if hd.FileInfo().Name() == "control" {
					return ParseControlFile(tr)
				}
			}
		}
	}

	return nil, GlobalVars().ErrMissingControlFile
}

// ParseControlFile parses a Debian control file to retrieve the metadata
func ParseControlFile(r io.Reader) (*Package, error) {
	p := &Package{
		Metadata: &Metadata{},
	}

	key := ""
	var depends strings.Builder
	var control strings.Builder
	var description strings.Builder

	// https://www.debian.org/doc/debian-policy/ch-controlfields.html#syntax-of-control-files
	s := bufio.NewScanner(r)
	for s.Scan() {
		line := s.Text()

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			// A binary package control file holds exactly one stanza. Stop at the
			// blank line that terminates it, otherwise a crafted control file could
			// smuggle additional stanzas (with attacker-chosen Filename/Package
			// fields) into the generated repository "Packages" index.
			if control.Len() == 0 {
				continue
			}
			break
		}

		control.WriteString(line)
		control.WriteByte('\n')

		if line[0] == ' ' || line[0] == '\t' {
			switch key {
			case "Description":
				// use a Builder instead of string += to avoid O(n^2) growth on crafted input
				description.WriteString(line)
			case "Depends":
				depends.WriteString(trimmed)
			}
		} else {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) < 2 {
				continue
			}

			key = parts[0]
			value := strings.TrimSpace(parts[1])
			switch key {
			case "Package":
				p.Name = value
			case "Version":
				p.Version = value
			case "Architecture":
				p.Architecture = value
			case "Maintainer":
				a, err := mail.ParseAddress(value)
				if err != nil || a.Name == "" {
					p.Metadata.Maintainer = value
				} else {
					p.Metadata.Maintainer = a.Name
				}
			case "Description":
				description.Reset()
				description.WriteString(value)
			case "Depends":
				depends.WriteString(value)
			case "Homepage":
				if validation.IsValidURL(value) {
					p.Metadata.ProjectURL = value
				}
			}
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}

	if !GlobalVars().namePattern.MatchString(p.Name) {
		return nil, GlobalVars().ErrInvalidName
	}
	if !GlobalVars().versionPattern.MatchString(p.Version) {
		return nil, GlobalVars().ErrInvalidVersion
	}
	if p.Architecture == "" {
		return nil, GlobalVars().ErrInvalidArchitecture
	}

	p.Metadata.Description = description.String()

	dependencies := strings.Split(depends.String(), ",")
	for i := range dependencies {
		dependencies[i] = strings.TrimSpace(dependencies[i])
	}
	p.Metadata.Dependencies = dependencies

	p.Control = strings.TrimSpace(control.String())

	return p, nil
}
