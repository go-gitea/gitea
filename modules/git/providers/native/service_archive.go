// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"
)

var _ (service.ArchiveService) = ArchiveService{}

// ArchiveService represents a very basic implementation of an archive
type ArchiveService struct{}

// CreateArchive create archive content to the target path
func (service ArchiveService) CreateArchive(ctx context.Context, r service.Repository, treeishID, filename string, opts service.CreateArchiveOpts) error {
	if opts.Format.String() == "unknown" {
		return fmt.Errorf("unknown format: %v", opts.Format)
	}

	args := []string{
		"archive",
	}
	if opts.Prefix {
		args = append(args, "--prefix="+filepath.Base(strings.TrimSuffix(r.Path(), ".git"))+"/")
	}

	args = append(args,
		"--format="+opts.Format.String(),
		"-o",
		filename,
		treeishID,
	)

	_, err := git.NewCommandContext(ctx, args...).RunInDir(r.Path())
	return err
}
