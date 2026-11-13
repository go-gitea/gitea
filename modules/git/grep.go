// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/util"
)

func GrepSearch(ctx context.Context, repo *Repository, patterns []string, refName string, visit func(string) (bool, error), paths ...string) (incomplete bool, err error) {
	cmd := gitcmd.NewCommand("grep", "-l", "-z", "-I", "--perl-regexp", "--ignore-case", "--full-name", "--all-match")
	for _, pattern := range patterns {
		cmd.AddOptionGrepExpr(pattern)
	}
	cmd.AddDynamicArguments(refName)
	cmd.AddDashesAndList(paths...)
	stdoutReader, stdoutReaderClose := cmd.MakeStdoutPipe()
	defer stdoutReaderClose()
	runErr := cmd.WithRepo(repo).
		WithPipelineFunc(func(pipelineCtx gitcmd.Context) error {
			scanner := bufio.NewScanner(stdoutReader)
			scanner.Split(util.BufioScannerSplit(0))
			for scanner.Scan() {
				_, filename, _ := strings.Cut(scanner.Text(), ":")
				keepGoing, err := visit(filename)
				if err != nil || !keepGoing {
					incomplete = !keepGoing
					return pipelineCtx.CancelPipeline(err)
				}
			}
			return scanner.Err()
		}).RunWithStderr(ctx)
	if err := ctx.Err(); err != nil {
		return incomplete, err
	}
	if gitcmd.IsErrorExitCode(runErr, 1) && runErr.Stderr() == "" {
		return incomplete, nil
	}
	if runErr != nil {
		return false, fmt.Errorf("GrepSearch: %w", runErr)
	}
	return incomplete, nil
}
