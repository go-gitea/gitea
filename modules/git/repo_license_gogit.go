// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"io"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/licensecheck"
)

// GetLicenseStats calculates license stats for git repository at specified commit
func (repo *Repository) GetLicenseStats(commitID string) ([]string, error) {
	r, err := git.PlainOpen(repo.Path)
	if err != nil {
		return nil, err
	}

	rev, err := r.ResolveRevision(plumbing.Revision(commitID))
	if err != nil {
		return nil, err
	}

	commit, err := r.CommitObject(*rev)
	if err != nil {
		return nil, err
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	var licenses []string
	// TODO: fix unnecessary check
	err = tree.Files().ForEach(func(f *object.File) error {
		if f.Size == 0 {
			return nil
		}

		// TODO: support ext
		if f.Name == "LICENSE" {
			r, err := f.Reader()
			if err != nil {
				return err
			}
			defer r.Close()

			content, err := io.ReadAll(r)
			if err != nil {
				return err
			}

			cov := licensecheck.Scan(content)
			for _, m := range cov.Match {
				licenses = append(licenses, m.ID)
			}
		}
		return nil
	})

	return licenses, nil
}
