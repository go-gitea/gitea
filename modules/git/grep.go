// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"context"
	"fmt"
	"io"
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
	ContextLineNumber int
	IsFuzzy           bool
}

func GrepSearch(ctx context.Context, repo *Repository, search string, opts GrepOptions) ([]*GrepResult, error) {
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("unable to creata os pipe to grep: %w", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("unable to creata os pipe to grep: %w", err)
	}
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		_ = stderrReader.Close()
		_ = stderrWriter.Close()
	}()

	/*
	 The output is like this ( "^@" means \x00):

	 HEAD:.air.toml
	 6^@bin = "gitea"

	 HEAD:.changelog.yml
	 2^@repo: go-gitea/gitea
	*/
	var stderr []byte
	var results []*GrepResult
	cmd := NewCommand(ctx, "grep", "--null", "--break", "--heading", "--fixed-strings", "--line-number", "--ignore-case", "--full-name")
	cmd.AddOptionValues("--context", fmt.Sprint(opts.ContextLineNumber))
	if opts.IsFuzzy {
		words := strings.Fields(search)
		for _, word := range words {
			cmd.AddOptionValues("-e", word)
		}
	} else {
		cmd.AddOptionValues("-e", search)
	}
	cmd.AddDynamicArguments(util.IfZero(opts.RefName, "HEAD"))
	err = cmd.Run(&RunOpts{
		Dir:    repo.Path,
		Stdout: stdoutWriter,
		PipelineFunc: func(ctx context.Context, cancel context.CancelFunc) error {
			_ = stdoutWriter.Close()
			_ = stderrWriter.Close()
			defer stdoutReader.Close()
			defer stderrReader.Close()

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
					if len(results) >= 50 {
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
			stderr, _ = io.ReadAll(stderrReader)
			return scanner.Err()
		},
	})
	if err != nil && len(stderr) != 0 {
		return nil, fmt.Errorf("unable to run grep: %w, stderr: %s", err, string(stderr))
	}
	return results, nil
}
