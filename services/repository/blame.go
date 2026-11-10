// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
)

type BlameResult struct {
	Parts                []*git.BlamePart
	UsesIgnoreRevs       bool
	FaultyIgnoreRevsFile bool
}

// GetBlame blames the file at the commit, honouring .git-blame-ignore-revs unless bypassed
// and falling back to a plain blame if git fails with it.
func GetBlame(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, commit *git.Commit, file string, bypassBlameIgnore bool) (*BlameResult, error) {
	objectFormat := git.ObjectFormatFromName(repo.ObjectFormatName)

	blameReader, err := git.CreateBlameReader(ctx, objectFormat, repo, gitRepo, commit, file, bypassBlameIgnore)
	if err != nil {
		return nil, err
	}

	r := &BlameResult{}
	if err := fillBlameResult(blameReader, r); err != nil {
		_ = blameReader.Close()
		return nil, err
	}

	err = blameReader.Close()
	if err != nil {
		if len(r.Parts) == 0 && r.UsesIgnoreRevs {
			// try again without ignored revs

			blameReader, err = git.CreateBlameReader(ctx, objectFormat, repo, gitRepo, commit, file, true)
			if err != nil {
				return nil, err
			}

			r := &BlameResult{
				FaultyIgnoreRevsFile: true,
			}
			if err := fillBlameResult(blameReader, r); err != nil {
				_ = blameReader.Close()
				return nil, err
			}

			return r, blameReader.Close()
		}
		return nil, err
	}
	return r, nil
}

func fillBlameResult(br *git.BlameReader, r *BlameResult) error {
	r.UsesIgnoreRevs = br.UsesIgnoreRevs()

	previousHelper := make(map[string]*git.BlamePart)

	r.Parts = make([]*git.BlamePart, 0, 5)
	for {
		blamePart, err := br.NextPart()
		if err != nil {
			return fmt.Errorf("BlameReader.NextPart failed: %w", err)
		}
		if blamePart == nil {
			break
		}

		if prev, ok := previousHelper[blamePart.Sha]; ok {
			if blamePart.PreviousSha == "" {
				blamePart.PreviousSha = prev.PreviousSha
				blamePart.PreviousPath = prev.PreviousPath
			}
		} else {
			previousHelper[blamePart.Sha] = blamePart
		}

		r.Parts = append(r.Parts, blamePart)
	}

	return nil
}
