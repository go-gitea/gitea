// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

// RawDiffType output format: diff or patch
type RawDiffType string

const (
	RawDiffNormal RawDiffType = "diff"
	RawDiffPatch  RawDiffType = "patch"
)

// GetRawDiff dumps diff results of repository in given commit ID to io.Writer.
func GetRawDiff(ctx context.Context, repo *Repository, commitID string, diffType RawDiffType, writer io.Writer) (retErr error) {
	cmd, err := getRepoRawDiffForFileCmd(ctx, repo, "", commitID, diffType, "")
	if err != nil {
		return fmt.Errorf("getRepoRawDiffForFileCmd: %w", err)
	}
	return cmd.WithStdoutCopy(writer).RunWithStderr(ctx)
}

// GetFileDiffCutAroundLine cuts the old or new part of the diff of a file around a specific line number
func GetFileDiffCutAroundLine(
	ctx context.Context, repo *Repository, startCommit, endCommit, treePath string,
	line int64, old bool, numbersOfLine int,
) (string, error) {
	return getFileDiffCut(ctx, repo, startCommit, endCommit, treePath, line, old, numbersOfLine, 0)
}

func GetFileDiffCutAroundRange(ctx context.Context, repo *Repository, startCommit, endCommit, treePath string, startLine, line int64) (string, error) {
	return getFileDiffCut(ctx, repo, startCommit, endCommit, treePath, max(line, -line), line < 0, 0, startLine)
}

func getFileDiffCut(ctx context.Context, repo *Repository, startCommit, endCommit, treePath string, line int64, old bool, numbersOfLine int, startLine int64) (string, error) {
	cmd, err := getRepoRawDiffForFileCmd(ctx, repo, startCommit, endCommit, RawDiffNormal, treePath)
	if err != nil {
		return "", fmt.Errorf("getRepoRawDiffForFileCmd: %w", err)
	}
	stdoutReader, stdoutClose := cmd.MakeStdoutPipe()
	defer stdoutClose()
	var patch string
	cmd.WithPipelineFunc(func(_ctx gitcmd.Context) error {
		patch, err = cutDiffAroundLine(stdoutReader, line, old, numbersOfLine, startLine)
		return err
	})
	err = cmd.RunWithStderr(ctx)
	return patch, err
}

type FileDiffHunk struct {
	OldStart, OldCount int64
	NewStart, NewCount int64
}

func parseFileDiffHunk(header string, advanceEmptySide bool) FileDiffHunk {
	fields := globalVars().hunkRegex.FindStringSubmatch(header)
	oldStart, _ := strconv.ParseInt(fields[1], 10, 64)
	oldCount, _ := strconv.ParseInt(cmp.Or(fields[3], "1"), 10, 64)
	newStart, _ := strconv.ParseInt(fields[4], 10, 64)
	newCount, _ := strconv.ParseInt(cmp.Or(fields[6], "1"), 10, 64)
	oldStart += util.Iif[int64](advanceEmptySide && oldCount == 0, 1, 0)
	newStart += util.Iif[int64](advanceEmptySide && newCount == 0, 1, 0)
	return FileDiffHunk{OldStart: oldStart, OldCount: oldCount, NewStart: newStart, NewCount: newCount}
}

// GetFileDiffHunks returns changes with 1-based starts, pointing to the next line on an empty side.
func GetFileDiffHunks(ctx context.Context, repo *Repository, startCommit, endCommit, path string) ([]FileDiffHunk, error) {
	cmd := gitcmd.NewCommand("diff", "--unified=0", "--inter-hunk-context=0", "--text", "--no-textconv", "--no-ext-diff", "--no-color").
		WithRepo(repo).AddOptionFormat("--find-renames=%s", setting.Git.DiffRenameSimilarityThreshold).
		AddDynamicArguments(startCommit, endCommit).AddDashesAndList(":(literal)" + path)
	stdout, closeStdout := cmd.MakeStdoutPipe()
	defer closeStdout()
	var hunks []FileDiffHunk
	cmd.WithPipelineFunc(func(_ctx gitcmd.Context) error {
		reader := bufio.NewReader(stdout)
		for continued := false; ; {
			line, more, err := reader.ReadLine()
			switch {
			case err == io.EOF:
				return nil
			case err != nil:
				return err
			case !continued && bytes.HasPrefix(line, []byte("@@ ")):
				hunks = append(hunks, parseFileDiffHunk(string(line), true))
			}
			continued = more
		}
	})
	err := cmd.RunWithStderr(ctx)
	return hunks, err
}

// getRepoRawDiffForFile returns an io.Reader for the diff results of file in given commit ID
// and a "finish" function to wait for the git command and clean up resources after reading is done.
func getRepoRawDiffForFileCmd(ctx context.Context, repo *Repository, startCommit, endCommit string, diffType RawDiffType, file string) (*gitcmd.Command, error) {
	commit, err := repo.GetCommit(ctx, endCommit)
	if err != nil {
		return nil, err
	}
	var files []string
	if len(file) > 0 {
		files = append(files, ":(literal)"+file)
	}

	cmd := gitcmd.NewCommand().WithRepo(repo)
	switch diffType {
	case RawDiffNormal:
		if len(startCommit) != 0 {
			cmd.AddArguments("diff").
				AddOptionFormat("--find-renames=%s", setting.Git.DiffRenameSimilarityThreshold).
				AddDynamicArguments(startCommit, endCommit).AddDashesAndList(files...)
		} else if commit.ParentCount() == 0 {
			cmd.AddArguments("show").AddDynamicArguments(endCommit).AddDashesAndList(files...)
		} else {
			c, err := commit.Parent(ctx, repo, 0)
			if err != nil {
				return nil, err
			}
			cmd.AddArguments("diff").
				AddOptionFormat("--find-renames=%s", setting.Git.DiffRenameSimilarityThreshold).
				AddDynamicArguments(c.ID.String(), endCommit).AddDashesAndList(files...)
		}
	case RawDiffPatch:
		if len(startCommit) != 0 {
			query := fmt.Sprintf("%s...%s", endCommit, startCommit)
			cmd.AddArguments("format-patch", "--no-signature", "--stdout", "--root").AddDynamicArguments(query).AddDashesAndList(files...)
		} else if commit.ParentCount() == 0 {
			cmd.AddArguments("format-patch", "--no-signature", "--stdout", "--root").AddDynamicArguments(endCommit).AddDashesAndList(files...)
		} else {
			c, err := commit.Parent(ctx, repo, 0)
			if err != nil {
				return nil, err
			}
			query := fmt.Sprintf("%s...%s", endCommit, c.ID.String())
			cmd.AddArguments("format-patch", "--no-signature", "--stdout").AddDynamicArguments(query).AddDashesAndList(files...)
		}
	default:
		return nil, util.NewInvalidArgumentErrorf("invalid diff type: %s", diffType)
	}
	return cmd, nil
}

// ParseDiffHunkString parse the diff hunk content and return
func ParseDiffHunkString(diffHunk string) (leftLine, leftHunk, rightLine, rightHunk int) {
	ss := strings.Split(diffHunk, "@@")
	ranges := strings.Split(ss[1][1:], " ")
	leftRange := strings.Split(ranges[0], ",")
	leftLine, _ = strconv.Atoi(leftRange[0][1:])
	if len(leftRange) > 1 {
		leftHunk, _ = strconv.Atoi(leftRange[1])
	}
	if len(ranges) > 1 {
		rightRange := strings.Split(ranges[1], ",")
		rightLine, _ = strconv.Atoi(rightRange[0])
		if len(rightRange) > 1 {
			rightHunk, _ = strconv.Atoi(rightRange[1])
		}
	} else {
		log.Debug("Parse line number failed: %v", diffHunk)
		rightLine = leftLine
		rightHunk = leftHunk
	}
	if rightLine == 0 {
		// FIXME: GIT-DIFF-CUT-BUG search this tag to see details
		// this is only a hacky patch, the rightLine&rightHunk might still be incorrect in some cases.
		rightLine++
	}
	return leftLine, leftHunk, rightLine, rightHunk
}

const cmdDiffHead = "diff --git "

func isHeader(lof string, inHunk bool) bool {
	return strings.HasPrefix(lof, cmdDiffHead) || (!inHunk && (strings.HasPrefix(lof, "---") || strings.HasPrefix(lof, "+++")))
}

func NewGitDiffScanner(r io.Reader) *bufio.Scanner {
	// TODO: GIT-DIFF-PARSE-LONG-LINE: ideally it shouldn't use bufio.Scanner which has a limit.
	// It will cause errors if a line is very long.
	scanner := bufio.NewScanner(r)
	scanner.Buffer(nil, max(512*1024, int(setting.UI.MaxDisplayFileSize/16)))
	return scanner
}

// CutDiffAroundLine cuts a diff of a file in way that only the given line + numberOfLine above it will be shown
// it also recalculates hunks and adds the appropriate headers to the new diff.
// Warning: Only one-file diffs are allowed.
func CutDiffAroundLine(originalDiff io.Reader, line int64, old bool, numbersOfLine int) (string, error) {
	return cutDiffAroundLine(originalDiff, line, old, numbersOfLine, 0)
}

func cutDiffAroundLine(originalDiff io.Reader, line int64, old bool, numbersOfLine int, startLine int64) (string, error) {
	if line == 0 || (numbersOfLine == 0 && startLine == 0) {
		// no line or num of lines => no diff
		return "", nil
	}

	scanner := NewGitDiffScanner(originalDiff)
	hunk := make([]string, 0)

	// begin is the start of the hunk containing searched line
	// end is the end of the hunk ...
	// currentLine is the line number on the side of the searched line (differentiated by old)
	// otherLine is the line number on the opposite side of the searched line (differentiated by old)
	var begin, end, currentLine, otherLine int64
	var headerLines int

	inHunk := false

	for scanner.Scan() {
		lof := scanner.Text()
		// Add header to enable parsing

		if isHeader(lof, inHunk) {
			if strings.HasPrefix(lof, cmdDiffHead) {
				inHunk = false
			}
			hunk = append(hunk, lof)
			headerLines++
		}
		if currentLine > line {
			break
		}
		// Detect "hunk" with contains commented lof
		if strings.HasPrefix(lof, "@@") {
			inHunk = true
			// Already got our hunk. End of hunk detected!
			if len(hunk) > headerLines {
				break
			}
			section := parseFileDiffHunk(lof, startLine != 0)
			if old {
				begin, end, otherLine = section.OldStart, section.OldCount, section.NewStart
			} else {
				begin, end, otherLine = section.NewStart, section.NewCount, section.OldStart
			}
			if begin <= line && line < begin+end {
				hunk = append(hunk, lof)
				currentLine = begin
				continue
			}
		} else if len(hunk) > headerLines {
			hunk = append(hunk, lof)
			// Count lines in context
			switch lof[0] {
			case '+':
				if !old {
					currentLine++
				} else {
					otherLine++
				}
			case '-':
				if old {
					currentLine++
				} else {
					otherLine++
				}
			case '\\':
				// FIXME: handle `\ No newline at end of file`
			default:
				currentLine++
				otherLine++
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("cutDiffAroundLine: scan: %w", err)
	}

	// No hunk found
	if currentLine == 0 {
		return "", nil
	}
	// headerLines + hunkLine (1) = totalNonCodeLines
	if startLine == 0 && len(hunk)-headerLines-1 <= numbersOfLine {
		// No need to cut the hunk => return existing hunk
		return strings.Join(hunk, "\n"), nil
	}
	var oldBegin, oldNumOfLines, newBegin, newNumOfLines int64
	if old {
		oldBegin = currentLine
		newBegin = otherLine
	} else {
		oldBegin = otherLine
		newBegin = currentLine
	}
	// calculate newBegin, ... by counting lines
	start := len(hunk)
	for count := 0; start > headerLines+1 && (startLine != 0 || count < numbersOfLine); {
		start--
		marker := hunk[start][0]
		switch marker {
		case '+':
			newBegin--
			newNumOfLines++
		case '-':
			oldBegin--
			oldNumOfLines++
		case '\\':
			continue
		default:
			oldBegin--
			newBegin--
			newNumOfLines++
			oldNumOfLines++
		}
		if (startLine < 0 && marker != '+' && -oldBegin == startLine) || (startLine > 0 && marker != '-' && newBegin == startLine) {
			break
		}
		count++
	}
	if startLine != 0 {
		oldBegin -= util.Iif[int64](oldNumOfLines == 0, 1, 0)
		newBegin -= util.Iif[int64](newNumOfLines == 0, 1, 0)
	}

	// "git diff" outputs "@@ -1 +1,3 @@" for "OLD" => "A\nB\nC"
	// FIXME: GIT-DIFF-CUT-BUG But there is a bug in CutDiffAroundLine, then the "Patch" stored in the comment model becomes "@@ -1,1 +0,4 @@"
	// It may generate incorrect results for difference cases, for example: delete 2 line add 1 line, delete 2 line add 2 line etc, need to double check.
	// For example: "L1\nL2" => "A\nB", then the patch shows "L2" as line 1 on the left (deleted part)

	// construct the new hunk header
	hunk[headerLines] = fmt.Sprintf("@@ -%d,%d +%d,%d @@",
		oldBegin, oldNumOfLines, newBegin, newNumOfLines)
	return strings.Join(append(hunk[:headerLines+1], hunk[start:]...), "\n"), nil
}

// GetAffectedFiles returns the affected files between two commits
func GetAffectedFiles(ctx context.Context, repo *Repository, branchName, oldCommitID, newCommitID string, env []string) ([]string, error) {
	// env contains GIT_QUARANTINE_PATH/GIT_OBJECT_DIRECTORY/GIT_ALTERNATE_OBJECT_DIRECTORIES from git push pre-receive hook
	if IsEmptyObjectID(oldCommitID) {
		startCommitID, err := getCommitBranchStart(ctx, repo, branchName, newCommitID, env)
		if err != nil {
			return nil, err
		}
		if startCommitID == "" {
			return nil, fmt.Errorf("cannot find the start commit of %s", newCommitID)
		}
		oldCommitID = startCommitID
	}
	return getFilesChangedBetweenWithEnv(ctx, repo, oldCommitID, newCommitID, env)
}

// GetReverseRawDiff dumps the reverse diff results of repository in given commit ID to io.Writer.
func GetReverseRawDiff(ctx context.Context, repo RepositoryFacade, commitID string, writer io.Writer) error {
	return gitcmd.NewCommand("show", "--pretty=format:revert %H%n", "-R").
		AddDynamicArguments(commitID).
		WithStdoutCopy(writer).
		WithRepo(repo).RunWithStderr(ctx)
}
