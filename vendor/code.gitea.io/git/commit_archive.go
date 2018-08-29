// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ArchiveType archive types
type ArchiveType int

const (
	// ZIP zip archive type
	ZIP ArchiveType = iota + 1
	// TARGZ tar gz archive type
	TARGZ
)

// CreateArchive create archive content to the target path
func (c *Commit) CreateArchive(target string, archiveType ArchiveType) error {
	var format string
	switch archiveType {
	case ZIP:
		format = "zip"
	case TARGZ:
		format = "tar.gz"
	default:
		return fmt.Errorf("unknown format: %v", archiveType)
	}

	_, err := NewCommand("archive", "--prefix="+filepath.Base(strings.TrimSuffix(c.repo.Path, ".git"))+"/", "--format="+format, "-o", target, c.ID.String()).RunInDir(c.repo.Path)
	return err
}
