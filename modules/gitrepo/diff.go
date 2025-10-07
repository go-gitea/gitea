// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// GetDiffShortStatByCmdArgs counts number of changed files, number of additions and deletions
// TODO: it can be merged with another "GetDiffShortStat" in the future
func GetDiffShortStatByCmdArgs(ctx context.Context, repo Repository, trustedArgs gitcmd.TrustedCmdArgs, dynamicArgs ...string) (numFiles, totalAdditions, totalDeletions int, err error) {
	// Now if we call:
	// $ git diff --shortstat 1ebb35b98889ff77299f24d82da426b434b0cca0...788b8b1440462d477f45b0088875
	// we get:
	// " 9902 files changed, 2034198 insertions(+), 298800 deletions(-)\n"
	cmd := gitcmd.NewCommand("diff", "--shortstat").AddArguments(trustedArgs...).AddDynamicArguments(dynamicArgs...)
	stdout, err := RunCmdString(ctx, repo, cmd)
	if err != nil {
		return 0, 0, 0, err
	}

	return parseDiffStat(stdout)
}

var shortStatFormat = regexp.MustCompile(
	`\s*(\d+) files? changed(?:, (\d+) insertions?\(\+\))?(?:, (\d+) deletions?\(-\))?`)

func parseDiffStat(stdout string) (numFiles, totalAdditions, totalDeletions int, err error) {
	if len(stdout) == 0 || stdout == "\n" {
		return 0, 0, 0, nil
	}
	groups := shortStatFormat.FindStringSubmatch(stdout)
	if len(groups) != 4 {
		return 0, 0, 0, fmt.Errorf("unable to parse shortstat: %s groups: %s", stdout, groups)
	}

	numFiles, err = strconv.Atoi(groups[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("unable to parse shortstat: %s. Error parsing NumFiles %w", stdout, err)
	}

	if len(groups[2]) != 0 {
		totalAdditions, err = strconv.Atoi(groups[2])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("unable to parse shortstat: %s. Error parsing NumAdditions %w", stdout, err)
		}
	}

	if len(groups[3]) != 0 {
		totalDeletions, err = strconv.Atoi(groups[3])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("unable to parse shortstat: %s. Error parsing NumDeletions %w", stdout, err)
		}
	}
	return numFiles, totalAdditions, totalDeletions, err
}
