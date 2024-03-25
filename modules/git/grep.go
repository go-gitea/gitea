// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

type GrepResult struct {
	Filename    string
	LineNumbers []int
	LineCodes   []string
}

type GrepOptions struct {
	RefName           string
	MaxResultLimit    int
	ContextLineNumber int
	IsFuzzy           bool
}

func GrepSearch(ctx context.Context, repo *Repository, search string, opts GrepOptions) ([]*GrepResult, error) {
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("unable to create os pipe to grep: %w", err)
	}
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	/*
	 The output is like this ( "^@" means \x00):

	 HEAD:.air.toml
	 6^@bin = "gitea"

	 HEAD:.changelog.yml
	 2^@repo: go-gitea/gitea
	*/
	var results []*GrepResult
	cmd := NewCommand(ctx, "grep", "--null", "--break", "--heading", "--fixed-strings", "--line-number", "--ignore-case", "--full-name")
	cmd.AddOptionValues("--context", fmt.Sprint(opts.ContextLineNumber))
	if opts.IsFuzzy {
		words := strings.Fields(search)
		for _, word := range words {
			cmd.AddOptionValues("-e", strings.TrimLeft(word, "-"))
		}
	} else {
		cmd.AddOptionValues("-e", strings.TrimLeft(search, "-"))
	}
	cmd.AddDynamicArguments(util.IfZero(opts.RefName, "HEAD"))
	opts.MaxResultLimit = util.IfZero(opts.MaxResultLimit, 50)
	stderr := bytes.Buffer{}
	err = cmd.Run(&RunOpts{
		Dir:    repo.Path,
		Stdout: stdoutWriter,
		Stderr: &stderr,
		PipelineFunc: func(ctx context.Context, cancel context.CancelFunc) error {
			_ = stdoutWriter.Close()
			defer stdoutReader.Close()

			isInBlock := false
			scanner := bufio.NewScanner(stdoutReader)
			var res *GrepResult
			for scanner.Scan() {
				line := scanner.Text()
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
						cancel()
						break
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
			return scanner.Err()
		},
	})
	// git grep exits by cancel (killed), usually it is caused by the limit of results
	if IsErrorExitCode(err, -1) && stderr.Len() == 0 {
		return results, nil
	}
	// git grep exits with 1 if no results are found
	if IsErrorExitCode(err, 1) && stderr.Len() == 0 {
		return nil, nil
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		return nil, fmt.Errorf("unable to run git grep: %w, stderr: %s", err, stderr.String())
	}
	return results, nil
}
