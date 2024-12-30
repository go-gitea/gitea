// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitgraph

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
)

// GetCommitGraph return a list of commit (GraphItems) from all branches
func GetCommitGraph(r *git.Repository, page, maxAllowedColors int, hidePRRefs bool, branches, files []string) (*Graph, error) {
	format := "DATA:%D|%H|%ad|%h|%s"

	if page == 0 {
		page = 1
	}

	graphCmd := git.NewCommand(r.Ctx, "log", "--graph", "--date-order", "--decorate=full")

	if hidePRRefs {
		graphCmd.AddArguments("--exclude=" + git.PullPrefix + "*")
	}

	if len(branches) == 0 {
		graphCmd.AddArguments("--all")
	}

	graphCmd.AddArguments("-C", "-M", "--date=iso-strict").
		AddOptionFormat("-n %d", setting.UI.GraphMaxCommitNum*page).
		AddOptionFormat("--pretty=format:%s", format)

	if len(branches) > 0 {
		graphCmd.AddDynamicArguments(branches...)
	}
	if len(files) > 0 {
		graphCmd.AddDashesAndList(files...)
	}
	graph := NewGraph()

	stderr := new(strings.Builder)
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	commitsToSkip := setting.UI.GraphMaxCommitNum * (page - 1)

	scanner := bufio.NewScanner(stdoutReader)

	if err := graphCmd.Run(&git.RunOpts{
		Dir:    r.Path,
		Stdout: stdoutWriter,
		Stderr: stderr,
		PipelineFunc: func(ctx context.Context, cancel context.CancelFunc) error {
			_ = stdoutWriter.Close()
			defer stdoutReader.Close()
			parser := &Parser{}
			parser.firstInUse = -1
			parser.maxAllowedColors = maxAllowedColors
			if maxAllowedColors > 0 {
				parser.availableColors = make([]int, maxAllowedColors)
				for i := range parser.availableColors {
					parser.availableColors[i] = i + 1
				}
			} else {
				parser.availableColors = []int{1, 2}
			}
			for commitsToSkip > 0 && scanner.Scan() {
				line := scanner.Bytes()
				dataIdx := bytes.Index(line, []byte("DATA:"))
				if dataIdx < 0 {
					dataIdx = len(line)
				}
				starIdx := bytes.IndexByte(line, '*')
				if starIdx >= 0 && starIdx < dataIdx {
					commitsToSkip--
				}
				parser.ParseGlyphs(line[:dataIdx])
			}

			row := 0

			// Skip initial non-commit lines
			for scanner.Scan() {
				line := scanner.Bytes()
				if bytes.IndexByte(line, '*') >= 0 {
					if err := parser.AddLineToGraph(graph, row, line); err != nil {
						cancel()
						return err
					}
					break
				}
				parser.ParseGlyphs(line)
			}

			for scanner.Scan() {
				row++
				line := scanner.Bytes()
				if err := parser.AddLineToGraph(graph, row, line); err != nil {
					cancel()
					return err
				}
			}
			return scanner.Err()
		},
	}); err != nil {
		return graph, err
	}
	return graph, nil
}
