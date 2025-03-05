// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

type DiffTree struct {
	Files []*DiffTreeRecord
}

type DiffTreeRecord struct {
	// Status is one of 'added', 'deleted', 'modified', 'renamed', 'copied', 'typechanged', 'unmerged', 'unknown'
	Status string

	// For renames and copies, the percentage of similarity between the source and target of the move/rename.
	Score uint8

	HeadPath   string
	BasePath   string
	HeadMode   git.EntryMode
	BaseMode   git.EntryMode
	HeadBlobID string
	BaseBlobID string
}

// GetDiffTree returns the list of path of the files that have changed between the two commits.
// If useMergeBase is true, the diff will be calculated using the merge base of the two commits.
// This is the same behavior as using a three-dot diff in git diff.
func GetDiffTree(ctx context.Context, gitRepo *git.Repository, useMergeBase bool, baseSha, headSha string) (*DiffTree, error) {
	gitDiffTreeRecords, err := runGitDiffTree(ctx, gitRepo, useMergeBase, baseSha, headSha)
	if err != nil {
		return nil, err
	}

	return &DiffTree{
		Files: gitDiffTreeRecords,
	}, nil
}

func runGitDiffTree(ctx context.Context, gitRepo *git.Repository, useMergeBase bool, baseSha, headSha string) ([]*DiffTreeRecord, error) {
	useMergeBase, baseCommitID, headCommitID, err := validateGitDiffTreeArguments(gitRepo, useMergeBase, baseSha, headSha)
	if err != nil {
		return nil, err
	}

	cmd := git.NewCommand("diff-tree", "--raw", "-r", "--find-renames", "--root")
	if useMergeBase {
		cmd.AddArguments("--merge-base")
	}
	cmd.AddDynamicArguments(baseCommitID, headCommitID)
	stdout, _, runErr := cmd.RunStdString(ctx, &git.RunOpts{Dir: gitRepo.Path})
	if runErr != nil {
		log.Warn("git diff-tree: %v", runErr)
		return nil, runErr
	}

	return parseGitDiffTree(strings.NewReader(stdout))
}

func validateGitDiffTreeArguments(gitRepo *git.Repository, useMergeBase bool, baseSha, headSha string) (shouldUseMergeBase bool, resolvedBaseSha, resolvedHeadSha string, err error) {
	// if the head is empty its an error
	if headSha == "" {
		return false, "", "", fmt.Errorf("headSha is empty")
	}

	// if the head commit doesn't exist its and error
	headCommit, err := gitRepo.GetCommit(headSha)
	if err != nil {
		return false, "", "", fmt.Errorf("failed to get commit headSha: %v", err)
	}
	headCommitID := headCommit.ID.String()

	// if the base is empty we should use the parent of the head commit
	if baseSha == "" {
		// if the headCommit has no parent we should use an empty commit
		// this can happen when we are generating a diff against an orphaned commit
		if headCommit.ParentCount() == 0 {
			objectFormat, err := gitRepo.GetObjectFormat()
			if err != nil {
				return false, "", "", err
			}

			// We set use merge base to false because we have no base commit
			return false, objectFormat.EmptyTree().String(), headCommitID, nil
		}

		baseCommit, err := headCommit.Parent(0)
		if err != nil {
			return false, "", "", fmt.Errorf("baseSha is '', attempted to use parent of commit %s, got error: %v", headCommit.ID.String(), err)
		}
		return useMergeBase, baseCommit.ID.String(), headCommitID, nil
	}

	// try and get the base commit
	baseCommit, err := gitRepo.GetCommit(baseSha)
	// propagate the error if we couldn't get the base commit
	if err != nil {
		return useMergeBase, "", "", fmt.Errorf("failed to get base commit %s: %v", baseSha, err)
	}

	return useMergeBase, baseCommit.ID.String(), headCommit.ID.String(), nil
}

func parseGitDiffTree(gitOutput io.Reader) ([]*DiffTreeRecord, error) {
	/*
		The output of `git diff-tree --raw -r --find-renames` is of the form:

		:<old_mode> <new_mode> <old_sha> <new_sha> <status>\t<path>

		or for renames:

		:<old_mode> <new_mode> <old_sha> <new_sha> <status>\t<old_path>\t<new_path>

		See: <https://git-scm.com/docs/git-diff-tree#_raw_output_format> for more details
	*/
	results := make([]*DiffTreeRecord, 0)

	lines := bufio.NewScanner(gitOutput)
	for lines.Scan() {
		line := lines.Text()

		if len(line) == 0 {
			continue
		}

		record, err := parseGitDiffTreeLine(line)
		if err != nil {
			return nil, err
		}

		results = append(results, record)
	}

	if err := lines.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func parseGitDiffTreeLine(line string) (*DiffTreeRecord, error) {
	line = strings.TrimPrefix(line, ":")
	splitSections := strings.SplitN(line, "\t", 2)
	if len(splitSections) < 2 {
		return nil, fmt.Errorf("unparsable output for diff-tree --raw: `%s`)", line)
	}

	fields := strings.Fields(splitSections[0])
	if len(fields) < 5 {
		return nil, fmt.Errorf("unparsable output for diff-tree --raw: `%s`, expected 5 space delimited values got %d)", line, len(fields))
	}

	baseMode, err := git.ParseEntryMode(fields[0])
	if err != nil {
		return nil, err
	}

	headMode, err := git.ParseEntryMode(fields[1])
	if err != nil {
		return nil, err
	}

	baseBlobID := fields[2]
	headBlobID := fields[3]

	status, score, err := statusFromLetter(fields[4])
	if err != nil {
		return nil, fmt.Errorf("unparsable output for diff-tree --raw: %s, error: %s", line, err)
	}

	filePaths := strings.Split(splitSections[1], "\t")

	var headPath, basePath string
	if status == "renamed" {
		if len(filePaths) != 2 {
			return nil, fmt.Errorf("unparsable output for diff-tree --raw: `%s`, expected 2 paths found %d", line, len(filePaths))
		}
		basePath = filePaths[0]
		headPath = filePaths[1]
	} else {
		basePath = filePaths[0]
		headPath = filePaths[0]
	}

	return &DiffTreeRecord{
		Status:     status,
		Score:      score,
		BaseMode:   baseMode,
		HeadMode:   headMode,
		BaseBlobID: baseBlobID,
		HeadBlobID: headBlobID,
		BasePath:   basePath,
		HeadPath:   headPath,
	}, nil
}

func statusFromLetter(rawStatus string) (status string, score uint8, err error) {
	if len(rawStatus) < 1 {
		return "", 0, fmt.Errorf("empty status letter")
	}
	switch rawStatus[0] {
	case 'A':
		return "added", 0, nil
	case 'D':
		return "deleted", 0, nil
	case 'M':
		return "modified", 0, nil
	case 'R':
		score, err = tryParseStatusScore(rawStatus)
		return "renamed", score, err
	case 'C':
		score, err = tryParseStatusScore(rawStatus)
		return "copied", score, err
	case 'T':
		return "typechanged", 0, nil
	case 'U':
		return "unmerged", 0, nil
	case 'X':
		return "unknown", 0, nil
	default:
		return "", 0, fmt.Errorf("unknown status letter: '%s'", rawStatus)
	}
}

func tryParseStatusScore(rawStatus string) (uint8, error) {
	if len(rawStatus) < 2 {
		return 0, fmt.Errorf("status score missing")
	}

	score, err := strconv.ParseUint(rawStatus[1:], 10, 8)
	if err != nil {
		return 0, fmt.Errorf("failed to parse status score: %w", err)
	} else if score > 100 {
		return 0, fmt.Errorf("status score out of range: %d", score)
	}

	return uint8(score), nil
}
