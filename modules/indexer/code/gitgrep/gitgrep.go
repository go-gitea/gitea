// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitgrep

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"gitea.dev/modules/git"
	"gitea.dev/modules/indexer"
	code_indexer "gitea.dev/modules/indexer/code"
	"gitea.dev/modules/setting"
)

// indexSettingToGitGrepPathspecList returns the pathspecs of the files to search, case-insensitive like the lowercased patterns.
// Git matches any of the positive pathspecs, so with include patterns the directory is filtered from the results instead.
func indexSettingToGitGrepPathspecList(dir string) (list []string) {
	for _, expr := range setting.Indexer.IncludePatterns {
		list = append(list, ":(glob,icase)"+expr.PatternString())
	}
	if dir != "" && len(list) == 0 {
		list = append(list, ":(literal)"+dir+"/")
	}
	for _, expr := range setting.Indexer.ExcludePatterns {
		list = append(list, ":(glob,exclude,icase)"+expr.PatternString())
	}
	return list
}

// MaxResultLimit is the maximum number of files a search returns
const MaxResultLimit = 50

type SearchOptions struct {
	RepoID     int64
	Ref        git.RefName
	Keyword    string
	SearchMode indexer.SearchModeType
	Path       string // only search the files in this directory
	Page       int
	PageSize   int
}

func PerformSearch(ctx context.Context, gitRepo *git.Repository, opts *SearchOptions) (searchResults []*code_indexer.Result, total int64, isLimited bool, err error) {
	grepMode := git.GrepModeWords
	switch opts.SearchMode {
	case indexer.SearchModeExact:
		grepMode = git.GrepModeExact
	case indexer.SearchModeRegexp:
		grepMode = git.GrepModeRegexp
	}
	res, err := git.GrepSearch(ctx, gitRepo, opts.Keyword, git.GrepOptions{
		ContextLineNumber: 1,
		GrepMode:          grepMode,
		RefName:           opts.Ref.String(),
		MaxResultLimit:    MaxResultLimit + 1, // one more to know whether results were dropped
		PathspecList:      indexSettingToGitGrepPathspecList(opts.Path),
	})
	if err != nil {
		// TODO: if no branch exists, it reports: exit status 128, fatal: this operation must be run in a work tree.
		return nil, 0, false, fmt.Errorf("git.GrepSearch: %w", err)
	}
	commitID, err := gitRepo.GetRefCommitID(ctx, opts.Ref.String())
	if err != nil {
		return nil, 0, false, fmt.Errorf("gitRepo.GetRefCommitID: %w", err)
	}

	isLimited = len(res) > MaxResultLimit
	if opts.Path != "" {
		res = slices.DeleteFunc(res, func(r *git.GrepResult) bool { return !strings.HasPrefix(r.Filename, opts.Path+"/") })
	}
	res = res[:min(len(res), MaxResultLimit)]
	total = int64(len(res))
	pageStart := min((opts.Page-1)*opts.PageSize, len(res))
	pageEnd := min(opts.Page*opts.PageSize, len(res))
	res = res[pageStart:pageEnd]
	matchPattern, _ := grepMatchPattern(opts.Keyword, grepMode) // positions are optional, git grep's PCRE may not compile as Go regexp
	for _, r := range res {
		rawContent := strings.Join(r.LineCodes, "\n")
		searchResults = append(searchResults, &code_indexer.Result{
			RepoID:   opts.RepoID,
			Filename: r.Filename,
			CommitID: commitID,
			// UpdatedUnix: not supported yet
			// Language:    not supported yet
			// Color:       not supported yet
			Lines:          code_indexer.HighlightSearchResultCode(r.Filename, "", r.LineNumbers, rawContent),
			RawContent:     rawContent,
			ContentMatches: findMatchRanges(matchPattern, rawContent),
		})
	}
	return searchResults, total, isLimited, nil
}

// grepMatchPattern returns a Go regexp matching what git.GrepSearch matches in the given mode
func grepMatchPattern(keyword string, grepMode git.GrepModeType) (*regexp.Regexp, error) {
	switch grepMode {
	case git.GrepModeExact:
		return regexp.Compile(regexp.QuoteMeta(keyword))
	case git.GrepModeRegexp:
		return regexp.Compile(keyword)
	default:
		var words []string
		for word := range strings.FieldsSeq(keyword) {
			words = append(words, regexp.QuoteMeta(word))
		}
		return regexp.Compile("(?i)" + strings.Join(words, "|"))
	}
}

func findMatchRanges(pattern *regexp.Regexp, content string) (ranges []code_indexer.MatchRange) {
	if pattern == nil {
		return nil
	}
	for _, loc := range pattern.FindAllStringIndex(content, -1) {
		if loc[0] < loc[1] {
			ranges = append(ranges, code_indexer.MatchRange{Start: loc[0], End: loc[1]})
		}
	}
	return ranges
}
