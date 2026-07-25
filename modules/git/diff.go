// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"bytes"
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
) (ret string, retErr error) {
	cmd, err := getRepoRawDiffForFileCmd(ctx, repo, startCommit, endCommit, RawDiffNormal, treePath)
	if err != nil {
		return "", fmt.Errorf("getRepoRawDiffForFileCmd: %w", err)
	}
	stdoutReader, stdoutClose := cmd.MakeStdoutPipe()
	defer stdoutClose()
	cmd.WithPipelineFunc(func(ctx gitcmd.Context) error {
		ret, err = CutDiffAroundLine(stdoutReader, line, old, numbersOfLine)
		return err
	})
	return ret, cmd.RunWithStderr(ctx)
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
		files = append(files, file)
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

// Example: @@ -1,8 +1,9 @@ => [..., 1, 8, 1, 9]
// We no longer use regexp for parsing hunk headers for performance reasons.

const cmdDiffHead = "diff --git "

func isHeader(lof []byte, inHunk bool) bool {
	return bytes.HasPrefix(lof, []byte(cmdDiffHead)) || (!inHunk && (bytes.HasPrefix(lof, []byte("---")) || bytes.HasPrefix(lof, []byte("+++"))))
}

// CutDiffAroundLine cuts a diff of a file in way that only the given line + numberOfLine above it will be shown
// it also recalculates hunks and adds the appropriate headers to the new diff.
// Warning: Only one-file diffs are allowed.
func CutDiffAroundLine(originalDiff io.Reader, line int64, old bool, numbersOfLine int) (string, error) {
	if line == 0 || numbersOfLine == 0 {
		// no line or num of lines => no diff
		return "", nil
	}

	data, err := io.ReadAll(originalDiff)
	if err != nil {
		return "", fmt.Errorf("CutDiffAroundLine: read: %w", err)
	}

	// We'll store start/end offsets of the lines we want to include
	type offset struct {
		start, end int
	}

	headerOffsets := make([]offset, 0, 8)
	hunkOffsets := make([]offset, 0, 64)

	// begin is the start of the hunk containing searched line
	// end is the end of the hunk ...
	// currentLine is the line number on the side of the searched line (differentiated by old)
	// otherLine is the line number on the opposite side of the searched line (differentiated by old)
	var begin, end, currentLine, otherLine int64

	inHunk := false
	pos := 0

	nextLine := func() (int, int, bool) {
		if pos >= len(data) {
			return 0, 0, false
		}
		start := pos
		idx := bytes.IndexByte(data[pos:], '\n')
		if idx == -1 {
			pos = len(data)
			return start, len(data), true
		}
		pos += idx + 1
		return start, start + idx, true
	}

	for {
		lStart, lEnd, ok := nextLine()
		if !ok {
			break
		}
		lof := data[lStart:lEnd]

		if isHeader(lof, inHunk) {
			if bytes.HasPrefix(lof, []byte(cmdDiffHead)) {
				inHunk = false
			}
			headerOffsets = append(headerOffsets, offset{lStart, lEnd})
		}
		if currentLine > line {
			break
		}
		// Detect "hunk" with contains commented lof
		if bytes.HasPrefix(lof, []byte("@@")) {
			inHunk = true
			// Already got our hunk. End of hunk detected!
			if len(hunkOffsets) > 0 {
				break
			}

			beginOld, endOld, beginNew, endNew, ok := parseHunkHeaderBytes(lof)
			if !ok {
				continue
			}

			if old {
				begin = beginOld
				end = endOld
				// init otherLine with begin of opposite side
				otherLine = beginNew
			} else {
				begin = beginNew
				end = endNew
				// init otherLine with begin of opposite side
				otherLine = beginOld
			}

			end += begin // end is for real only the number of lines in hunk
			// lof is between begin and end
			if begin <= line && end >= line {
				hunkOffsets = append(hunkOffsets, offset{lStart, lEnd})
				currentLine = begin
				continue
			}
		} else if len(hunkOffsets) > 0 {
			hunkOffsets = append(hunkOffsets, offset{lStart, lEnd})
			// Count lines in context
			if len(lof) > 0 {
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
			} else {
				currentLine++
				otherLine++
			}
		}
	}

	// No hunk found
	if currentLine == 0 {
		return "", nil
	}

	headerLines := len(headerOffsets)

	// headerLines + hunkLine (1) = totalNonCodeLines
	if len(hunkOffsets)-1 <= numbersOfLine {
		// No need to cut the hunk => return existing hunk
		var sb strings.Builder
		totalSize := 0
		for _, off := range headerOffsets {
			totalSize += off.end - off.start + 1
		}
		for _, off := range hunkOffsets {
			totalSize += off.end - off.start + 1
		}
		sb.Grow(totalSize)

		for i, off := range headerOffsets {
			if i > 0 {
				sb.WriteByte('\n')
			}
			sb.Write(data[off.start:off.end])
		}
		for _, off := range hunkOffsets {
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			sb.Write(data[off.start:off.end])
		}
		return sb.String(), nil
	}

	var oldBegin, oldNumOfLines, newBegin, newNumOfLines int64
	if old {
		oldBegin = currentLine
		newBegin = otherLine
	} else {
		oldBegin = otherLine
		newBegin = currentLine
	}

	// headers + hunk header
	resOffsets := make([]offset, 0, headerLines+numbersOfLine+1)
	// transfer existing headers
	resOffsets = append(resOffsets, headerOffsets...)
	// placeholder for new hunk header
	resOffsets = append(resOffsets, offset{0, 0})
	// transfer last n lines
	resOffsets = append(resOffsets, hunkOffsets[len(hunkOffsets)-numbersOfLine:]...)

	// calculate newBegin, ... by counting lines
	for idx := len(hunkOffsets) - 1; idx >= len(hunkOffsets)-numbersOfLine; idx-- {
		off := hunkOffsets[idx]
		lof := data[off.start:off.end]
		if len(lof) > 0 {
			switch lof[0] {
			case '+':
				newBegin--
				newNumOfLines++
			case '-':
				oldBegin--
				oldNumOfLines++
			default:
				oldBegin--
				newBegin--
				newNumOfLines++
				oldNumOfLines++
			}
		} else {
			oldBegin--
			newBegin--
			newNumOfLines++
			oldNumOfLines++
		}
	}

	// "git diff" outputs "@@ -1 +1,3 @@" for "OLD" => "A\nB\nC"
	// FIXME: GIT-DIFF-CUT-BUG But there is a bug in CutDiffAroundLine, then the "Patch" stored in the comment model becomes "@@ -1,1 +0,4 @@"
	// It may generate incorrect results for difference cases, for example: delete 2 line add 1 line, delete 2 line add 2 line etc, need to double check.
	// For example: "L1\nL2" => "A\nB", then the patch shows "L2" as line 1 on the left (deleted part)

	// construct the new hunk header
	var sb strings.Builder
	for idx, off := range resOffsets {
		if idx > 0 {
			sb.WriteByte('\n')
		}
		if idx == headerLines {
			fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@", oldBegin, oldNumOfLines, newBegin, newNumOfLines)
		} else {
			sb.Write(data[off.start:off.end])
		}
	}
	return sb.String(), nil
}

func parseHunkHeaderBytes(lof []byte) (beginOld, endOld, beginNew, endNew int64, ok bool) {
	if !bytes.HasPrefix(lof, []byte("@@ -")) {
		return 0, 0, 0, 0, false
	}
	endIdx := bytes.Index(lof[4:], []byte(" @@"))
	if endIdx == -1 {
		return 0, 0, 0, 0, false
	}
	content := lof[4 : 4+endIdx]
	parts := bytes.SplitN(content, []byte(" +"), 2)
	if len(parts) != 2 {
		return 0, 0, 0, 0, false
	}

	oldPart := string(parts[0])
	newPart := string(parts[1])

	oldSubparts := strings.SplitN(oldPart, ",", 2)
	var err error
	beginOld, err = strconv.ParseInt(oldSubparts[0], 10, 64)
	if err != nil {
		return 0, 0, 0, 0, false
	}
	if len(oldSubparts) > 1 {
		endOld, err = strconv.ParseInt(oldSubparts[1], 10, 64)
		if err != nil {
			return 0, 0, 0, 0, false
		}
	}

	newSubparts := strings.SplitN(newPart, ",", 2)
	beginNew, err = strconv.ParseInt(newSubparts[0], 10, 64)
	if err != nil {
		return 0, 0, 0, 0, false
	}
	if len(newSubparts) > 1 {
		endNew, err = strconv.ParseInt(newSubparts[1], 10, 64)
		if err != nil {
			return 0, 0, 0, 0, false
		}
	}

	return beginOld, endOld, beginNew, endNew, true
}

// GetAffectedFiles returns the affected files between two commits
func GetAffectedFiles(ctx context.Context, repo *Repository, branchName, oldCommitID, newCommitID string, env []string) ([]string, error) {
	if oldCommitID == emptySha1ObjectID.String() || oldCommitID == emptySha256ObjectID.String() {
		startCommitID, err := repo.GetCommitBranchStart(ctx, env, branchName, newCommitID)
		if err != nil {
			return nil, err
		}
		if startCommitID == "" {
			return nil, fmt.Errorf("cannot find the start commit of %s", newCommitID)
		}
		oldCommitID = startCommitID
	}

	affectedFiles := make([]string, 0, 32)

	// Run `git diff --name-only` to get the names of the changed files
	cmd := gitcmd.NewCommand("diff", "--name-only").AddDynamicArguments(oldCommitID, newCommitID)
	stdoutReader, stdoutReaderClose := cmd.MakeStdoutPipe()
	defer stdoutReaderClose()
	err := cmd.WithEnv(env).WithRepo(repo).
		WithPipelineFunc(func(ctx gitcmd.Context) error {
			// Now scan the output from the command
			scanner := bufio.NewScanner(stdoutReader)
			for scanner.Scan() {
				path := strings.TrimSpace(scanner.Text())
				if len(path) == 0 {
					continue
				}
				affectedFiles = append(affectedFiles, path)
			}
			return scanner.Err()
		}).
		Run(ctx)
	if err != nil {
		log.Error("Unable to get affected files for commits from %s to %s in %s: %v", oldCommitID, newCommitID, repo.LogString(), err)
	}

	return affectedFiles, err
}
