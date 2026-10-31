// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitgrep

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"gitea.dev/modules/git"
	"gitea.dev/modules/indexer"
	code_indexer "gitea.dev/modules/indexer/code"
	"gitea.dev/modules/indexer/code/internal"
	"gitea.dev/modules/util"
)

const MaxResultLimit = 50

type SearchOptions struct {
	RepoID   int64
	Ref      git.RefName
	Terms    []indexer.SearchTerm
	Path     string
	Page     int
	PageSize int
}

func PerformSearch(ctx context.Context, gitRepo *git.Repository, opts *SearchOptions) (searchResults []*code_indexer.Result, total int64, incomplete bool, err error) {
	if opts == nil || len(opts.Terms) == 0 {
		return nil, 0, false, nil
	}
	matcher, err := internal.NewFileMatcher(opts.Terms)
	if err != nil {
		return nil, 0, false, err
	}
	searchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	defer func() {
		if ctx.Err() != nil {
			err = ctx.Err()
		} else if errors.Is(searchCtx.Err(), context.DeadlineExceeded) {
			err = nil
			incomplete = true
		}
	}()
	patterns := make([]string, 0, len(opts.Terms))
	for _, term := range opts.Terms {
		pattern := term.Text
		if term.Kind == indexer.SearchTermLiteral {
			pattern = strings.ReplaceAll(regexp.QuoteMeta(strings.ToLower(pattern)), "i", "[iİ]")
		}
		patterns = append(patterns, "(*UTF)"+pattern)
	}
	commit, err := gitRepo.GetCommit(searchCtx, util.IfZero(opts.Ref.String(), "HEAD"))
	if err != nil {
		if ctx.Err() != nil {
			return nil, 0, false, ctx.Err()
		}
		if empty, emptyErr := gitRepo.IsEmpty(searchCtx); emptyErr == nil && empty {
			return nil, 0, false, nil
		}
		return nil, 0, false, fmt.Errorf("PerformSearch: %w", err)
	}
	var paths []string
	if opts.Path != "" {
		paths = []string{":(literal)" + opts.Path + "/"}
	}
	files, err := code_indexer.ListFiles(searchCtx, gitRepo, commit.ID.String(), paths...)
	if err != nil {
		return nil, 0, false, err
	}
	eligible := make(map[string]internal.FileUpdate, len(files))
	var pathMatches []string
	for _, file := range files {
		eligible[file.Filename] = file
		if _, ok := matcher.MatchFile(file.Filename, ""); ok {
			pathMatches = append(pathMatches, file.Filename)
		}
	}
	if len(eligible) == 0 {
		return nil, 0, false, nil
	}
	slices.Sort(pathMatches)
	batch, err := git.NewBatch(searchCtx, gitRepo)
	if err != nil {
		return nil, 0, false, err
	}
	defer batch.Close()
	var results []*code_indexer.Result
	accept := func(filename string) (bool, error) {
		if err := searchCtx.Err(); err != nil {
			return false, err
		}
		file, ok := eligible[filename]
		if !ok {
			return true, nil
		}
		content, searchable, err := internal.ReadFile(batch, file)
		if err != nil {
			return false, err
		}
		if !searchable {
			return true, nil
		}
		result := &code_indexer.Result{RepoID: opts.RepoID, Filename: filename, CommitID: commit.ID.String(), Content: string(content)}
		if matcher.MatchResult(result) {
			if len(results) == MaxResultLimit {
				incomplete = true
				return false, nil
			}
			results = append(results, result)
		}
		return true, nil
	}
	grepIncomplete, err := git.GrepSearch(searchCtx, gitRepo, patterns, commit.ID.String(), func(filename string) (bool, error) {
		for len(pathMatches) > 0 && pathMatches[0] < filename {
			keepGoing, err := accept(pathMatches[0])
			pathMatches = pathMatches[1:]
			if err != nil || !keepGoing {
				return keepGoing, err
			}
		}
		if len(pathMatches) > 0 && pathMatches[0] == filename {
			pathMatches = pathMatches[1:]
		}
		return accept(filename)
	}, paths...)
	if err == nil && !grepIncomplete {
		for _, filename := range pathMatches {
			keepGoing, acceptErr := accept(filename)
			if acceptErr != nil || !keepGoing {
				err = acceptErr
				break
			}
		}
	}
	if err != nil && searchCtx.Err() == nil {
		return nil, 0, false, fmt.Errorf("PerformSearch: %w", err)
	}
	return util.PaginateSlice(results, max(opts.Page, 1), util.IfZero(opts.PageSize, MaxResultLimit)), int64(len(results)), incomplete || grepIncomplete, nil
}
