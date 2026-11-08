// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
)

// ArchiveType archive types
type ArchiveType int

const (
	ArchiveUnknown ArchiveType = iota
	ArchiveZip                 // 1
	ArchiveTarGz               // 2
	ArchiveBundle              // 3
)

// String converts an ArchiveType to string: the extension of the archive file without prefix dot
func (a ArchiveType) String() string {
	switch a {
	case ArchiveZip:
		return "zip"
	case ArchiveTarGz:
		return "tar.gz"
	case ArchiveBundle:
		return "bundle"
	}
	return "unknown"
}

func SplitArchiveNameType(s string) (string, ArchiveType) {
	switch {
	case strings.HasSuffix(s, ".zip"):
		return strings.TrimSuffix(s, ".zip"), ArchiveZip
	case strings.HasSuffix(s, ".tar.gz"):
		return strings.TrimSuffix(s, ".tar.gz"), ArchiveTarGz
	case strings.HasSuffix(s, ".bundle"):
		return strings.TrimSuffix(s, ".bundle"), ArchiveBundle
	}
	return s, ArchiveUnknown
}

// SetArchiveRepoState sets if a repo is archived
func SetArchiveRepoState(ctx context.Context, repo *Repository, isArchived bool) (err error) {
	repo.IsArchived = isArchived

	if isArchived {
		repo.ArchivedUnix = timeutil.TimeStampNow()
	} else {
		repo.ArchivedUnix = timeutil.TimeStamp(0)
	}

	_, err = db.GetEngine(ctx).ID(repo.ID).Cols("is_archived", "archived_unix").NoAutoTime().Update(repo)
	return err
}
