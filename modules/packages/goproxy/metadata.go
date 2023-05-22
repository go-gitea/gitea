// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package goproxy

import (
	"archive/zip"
	"fmt"
	"io"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

const (
	PropertyGoMod = "go.mod"

	maxGoModFileSize = 16 * 1024 * 1024 // https://go.dev/ref/mod#zip-path-size-constraints
)

var (
	ErrInvalidStructure  = util.NewInvalidArgumentErrorf("package has invalid structure")
	ErrGoModFileTooLarge = util.NewInvalidArgumentErrorf("go.mod file is too large")
)

type Package struct {
	Name    string
	Version string
	GoMod   string
}

// ParsePackage parses the Go package file
// https://go.dev/ref/mod#zip-files
func ParsePackage(r io.ReaderAt, size int64) (*Package, error) {
	archive, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}

	var p *Package

	for _, file := range archive.File {
		nameAndVersion := path.Dir(file.Name)

		parts := strings.SplitN(nameAndVersion, "@", 2)
		if len(parts) != 2 {
			continue
		}

		versionParts := strings.SplitN(parts[1], "/", 2)

		if p == nil {
			p = &Package{
				Name:    strings.TrimSuffix(nameAndVersion, "@"+parts[1]),
				Version: versionParts[0],
			}
		}

		if len(versionParts) > 1 {
			// files are expected in the "root" folder
			continue
		}

		if path.Base(file.Name) == "go.mod" {
			if file.UncompressedSize64 > maxGoModFileSize {
				return nil, ErrGoModFileTooLarge
			}

			f, err := archive.Open(file.Name)
			if err != nil {
				return nil, err
			}
			defer f.Close()

			bytes, err := io.ReadAll(&io.LimitedReader{R: f, N: maxGoModFileSize})
			if err != nil {
				return nil, err
			}

			p.GoMod = string(bytes)

			return p, nil
		}
	}

	if p == nil {
		return nil, ErrInvalidStructure
	}

	p.GoMod = fmt.Sprintf("module %s", p.Name)

	return p, nil
}
