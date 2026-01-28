// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/util"
)

type GrepResult struct {
	Filename    string
	LineNumbers []int
	LineCodes   []string
}

type GrepModeType string

const (
	GrepModeExact  GrepModeType = "exact"
	GrepModeWords  GrepModeType = "words"
	GrepModeRegexp GrepModeType = "regexp"
)

type GrepOptions struct {
	RefName           string
	MaxResultLimit    int
	ContextLineNumber int
	GrepMode          GrepModeType
	MaxLineLength     int // the maximum length of a line to parse, exceeding chars will be truncated
	PathspecList      []string
}

func GrepSearch(ctx context.Context, repo *Repository, search string, opts GrepOptions) ([]*GrepResult, error) {
	/*
	 The output is like this ( "^@" means \x00):

	 HEAD:.air.toml
	 6^@bin = "gitea"

	 HEAD:.changelog.yml
	 2^@repo: go-gitea/gitea
	*/
	var results []*GrepResult
	cmd := gitcmd.NewCommand("grep", "--null", "--break", "--heading", "--line-number", "--full-name")
	cmd.AddOptionValues("--context", strconv.Itoa(opts.ContextLineNumber))
	switch opts.GrepMode {
	case GrepModeExact:
		cmd.AddArguments("--fixed-strings")
		cmd.AddOptionValues("-e", strings.TrimLeft(search, "-"))
	case GrepModeRegexp:
		cmd.AddArguments("--perl-regexp")
		cmd.AddOptionValues("-e", strings.TrimLeft(search, "-"))
	default: /* words */
		words := strings.Fields(search)
		cmd.AddArguments("--fixed-strings", "--ignore-case")
		for i, word := range words {
			cmd.AddOptionValues("-e", strings.TrimLeft(word, "-"))
			if i < len(words)-1 {
				cmd.AddOptionValues("--and")
			}
		}
	}
	cmd.AddDynamicArguments(util.IfZero(opts.RefName, "HEAD"))
	cmd.AddDashesAndList(opts.PathspecList...)
	opts.MaxResultLimit = util.IfZero(opts.MaxResultLimit, 50)

	stdoutReader, stdoutReaderClose := cmd.MakeStdoutPipe()
	defer stdoutReaderClose()
	err := cmd.WithDir(repo.Path).
		WithPipelineFunc(func(ctx gitcmd.Context) error {
			isInBlock := false
			rd := bufio.NewReaderSize(stdoutReader, util.IfZero(opts.MaxLineLength, 16*1024))
			var res *GrepResult
			for {
				lineBytes, isPrefix, err := rd.ReadLine()
				if isPrefix {
					lineBytes = slices.Clone(lineBytes)
					for isPrefix && err == nil {
						_, isPrefix, err = rd.ReadLine()
					}
				}
				if len(lineBytes) == 0 && err != nil {
					break
				}
				line := string(lineBytes) // the memory of lineBytes is mutable
				if !isInBlock {
					if _ /* ref */, filename, ok := strings.Cut(line, ":"); ok {
						isInBlock = true
						res = &GrepResult{Filename: filename}
						results = append(results, res)
					}
					continue
				}
				if line == "" {
					if len(results) >= opts.MaxResultLimit {
						return ctx.CancelPipeline(nil)
					}
					isInBlock = false
					continue
				}
				if line == "--" {
					continue
				}
				if lineNum, lineCode, ok := strings.Cut(line, "\x00"); ok {
					lineNumInt, _ := strconv.Atoi(lineNum)
					res.LineNumbers = append(res.LineNumbers, lineNumInt)
					res.LineCodes = append(res.LineCodes, lineCode)
				}
			}
			return nil
		}).
		RunWithStderr(ctx)
	// git grep exits by cancel (killed), usually it is caused by the limit of results
	if gitcmd.IsErrorExitCode(err, -1) && err.Stderr() == "" {
		return results, nil
	}
	// git grep exits with 1 if no results are found
	if gitcmd.IsErrorExitCode(err, 1) && err.Stderr() == "" {
		return nil, nil
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		return nil, fmt.Errorf("unable to run git grep: %w", err)
	}
	return results, nil
}
