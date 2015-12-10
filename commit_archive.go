// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import "fmt"

type ArchiveType int

const (
	ZIP ArchiveType = iota + 1
	TARGZ
)

func (c *Commit) CreateArchive(path string, archiveType ArchiveType) error {
	var format string
	switch archiveType {
	case ZIP:
		format = "zip"
	case TARGZ:
		format = "tar.gz"
	default:
		return fmt.Errorf("unknown format: %v", archiveType)
	}

	_, err := NewCommand("archive", "--format="+format, "-o", path, c.ID.String()).RunInDir(c.repo.Path)
	return err
}
