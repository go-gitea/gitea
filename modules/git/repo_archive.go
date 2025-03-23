// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
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

// CreateArchive create archive content to the target path
func (repo *Repository) CreateArchive(ctx context.Context, format ArchiveType, target io.Writer, usePrefix bool, commitID string) error {
	if format.String() == "unknown" {
		return fmt.Errorf("unknown format: %v", format)
	}

	cmd := NewCommand("archive")
	if usePrefix {
		cmd.AddOptionFormat("--prefix=%s", filepath.Base(strings.TrimSuffix(repo.Path, ".git"))+"/")
	}
	cmd.AddOptionFormat("--format=%s", format.String())
	cmd.AddDynamicArguments(commitID)

	var stderr strings.Builder
	err := cmd.Run(ctx, &RunOpts{
		Dir:    repo.Path,
		Stdout: target,
		Stderr: &stderr,
	})
	if err != nil {
		return ConcatenateError(err, stderr.String())
	}
	return nil
}
