// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	pull_model "gitea.dev/models/pull"
	"gitea.dev/modules/charset"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
)

// DiffLineType represents the type of DiffLine.
type DiffLineType uint8

// DiffLineType possible values.
const (
	DiffLinePlain DiffLineType = iota + 1
	DiffLineAdd
	DiffLineDel
	DiffLineSection
)

// DiffFileType represents the type of DiffFile.
type DiffFileType uint8

// DiffFileType possible values.
const (
	DiffFileAdd DiffFileType = iota + 1
	DiffFileChange
	DiffFileDel
	DiffFileRename
	DiffFileCopy
)

// DiffLine represents a line difference in a DiffSection.
type DiffLine struct {
	LeftIdx     int // line number, 1-based
	RightIdx    int // line number, 1-based
	Match       int // the diff matched index. -1: no match. 0: plain and no need to match. >0: for add/del, "Lines" slice index of the other side
	Type        DiffLineType
	Content     string
	IsTruncated bool
}

const (
	DiffStyleSplit   = "split"
	DiffStyleUnified = "unified"
)

// DiffSection represents a section of a DiffFile.
type DiffSection struct {
	FileName string
	Lines    []*DiffLine
}

// DiffFile represents a file diff.
type DiffFile struct {
	// only used internally to parse Ambiguous filenames
	isAmbiguous bool

	// basic fields (parsed from diff result)
	Name         string
	NameHash     string
	OldName      string
	Addition     int
	Deletion     int
	Type         DiffFileType
	EntryMode    string
	OldEntryMode string
	IsCreated    bool
	IsDeleted    bool
	IsBin        bool
	IsRenamed    bool
	// basic fields but for render purpose only
	Sections     []*DiffSection
	IsIncomplete bool // file is too large
}

// GetDiffFileName returns the name of the diff file, or its old name in case it was deleted
func (diffFile *DiffFile) GetDiffFileName() string {
	if diffFile.Name == "" {
		return diffFile.OldName
	}
	return diffFile.Name
}

// Diff represents a difference between two git trees.
type Diff struct {
	Start, End   string
	Files        []*DiffFile
	IsIncomplete bool
}

const cmdDiffHead = "diff --git "

// to correctly parse a diff line, the input buffer size should be large enough to read a full line for git diff headers
var defaultDiffLineBufferSize = 8 * 1024

// ParsePatch builds a Diff object by parsing git diff output
func ParsePatch(ctx context.Context, maxLines, maxLineCharacters, maxFiles int, reader io.Reader) (_ *Diff, retErr error) {
	log.Debug("ParsePatch(%d, %d, %d, ...)", maxLines, maxLineCharacters, maxFiles)

	diff := &Diff{Files: make([]*DiffFile, 0)}
	readerSize := max(maxLineCharacters, defaultDiffLineBufferSize)

	input := bufio.NewReaderSize(reader, readerSize)
	line, err := input.ReadString('\n')
	if err != nil {
		return diff, util.Iif(err == io.EOF, nil, err)
	}

	for {
		nextLine, err := diff.parseOneDiffFile(ctx, maxLines, maxLineCharacters, maxFiles, input, line)
		if nextLine == "" || err == io.EOF {
			break
		} else if err != nil {
			return diff, err
		}
		line = nextLine
	}

	diff.postProcessFiles()
	return diff, nil
}

func (diff *Diff) parseOneDiffFile(ctx context.Context, maxLines, maxLineCharacters, maxFiles int, input *bufio.Reader, startLine string) (nextLine string, err error) {
	line := startLine

	extractGitDiffHead := func(s, p string) string {
		return strings.TrimSpace(strings.TrimPrefix(s, p))
	}

	{
		// 1. A patch file always begins with `diff --git ` + `a/path b/path` (possibly quoted)
		// if it does not we have bad input!
		if !strings.HasPrefix(line, cmdDiffHead) {
			return "", fmt.Errorf("invalid first file line: %s", line)
		}

		if maxFiles > -1 && len(diff.Files) >= maxFiles {
			lastFile := createDiffFile(line)
			diff.End = lastFile.Name
			diff.IsIncomplete = true
			return "", nil
		}

		curFile := createDiffFile(line)
		diff.Files = append(diff.Files, curFile)

		// 2. It is followed by one or more extended header lines:
		//
		//     old mode <mode>
		//     new mode <mode>
		//     deleted file mode <mode>
		//     new file mode <mode>
		//     copy from <path>
		//     copy to <path>
		//     rename from <path>
		//     rename to <path>
		//     similarity index <number>
		//     dissimilarity index <number>
		//     index <hash>..<hash> <mode>
		//
		// * <mode> 6-digit octal numbers including the file type and file permission bits.
		// * <path> does not include the a/ and b/ prefixes
		// * <number> percentage of unchanged lines for similarity, percentage of changed
		//   lines dissimilarity as integer rounded down with terminal %. 100% => equal files.
		// * The index line includes the blob object names before and after the change.
		//   The <mode> is included if the file mode does not change; otherwise, separate
		//   lines indicate the old and the new mode.
		// 3. Following this header the "standard unified" diff format header may be encountered: (but not for every case...)
		//
		//     --- a/<path>
		//     +++ b/<path>
		//
		// With multiple hunks
		//
		//     @@ <hunk descriptor> @@
		//     +added line
		//     -removed line
		//      unchanged line
		//
		// 4. Binary files get:
		//
		//     Binary files a/<path> and b/<path> differ
		//
		// but one of a/<path> and b/<path> could be /dev/null.
		for {
			line, err = input.ReadString('\n')
			if err != nil {
				return line, err
			}

			switch {
			case strings.HasPrefix(line, cmdDiffHead):
				return line, nil
			case strings.HasPrefix(line, "old mode ") ||
				strings.HasPrefix(line, "new mode "):

				if strings.HasPrefix(line, "old mode ") {
					curFile.OldEntryMode = extractGitDiffHead(line, "old mode ")
				}
				if strings.HasPrefix(line, "new mode ") {
					curFile.EntryMode = extractGitDiffHead(line, "new mode ")
				}
			case strings.HasPrefix(line, "rename from "):
				curFile.IsRenamed = true
				curFile.Type = DiffFileRename
				if curFile.isAmbiguous {
					curFile.OldName = extractGitDiffHead(line, "rename from ")
				}
			case strings.HasPrefix(line, "rename to "):
				curFile.IsRenamed = true
				curFile.Type = DiffFileRename
				if curFile.isAmbiguous {
					curFile.Name = extractGitDiffHead(line, "rename to ")
					curFile.isAmbiguous = false
				}
			case strings.HasPrefix(line, "copy from "):
				curFile.IsRenamed = true
				curFile.Type = DiffFileCopy
				if curFile.isAmbiguous {
					curFile.OldName = extractGitDiffHead(line, "copy from ")
				}
			case strings.HasPrefix(line, "copy to "):
				curFile.IsRenamed = true
				curFile.Type = DiffFileCopy
				if curFile.isAmbiguous {
					curFile.Name = extractGitDiffHead(line, "copy to ")
					curFile.isAmbiguous = false
				}
			case strings.HasPrefix(line, "new file"):
				curFile.Type = DiffFileAdd
				curFile.IsCreated = true
				if strings.HasPrefix(line, "new file mode ") {
					curFile.EntryMode = extractGitDiffHead(line, "new file mode ")
				}
			case strings.HasPrefix(line, "deleted"):
				curFile.Type = DiffFileDel
				curFile.IsDeleted = true
			case strings.HasPrefix(line, "similarity index 100%"):
				curFile.Type = DiffFileRename
			case strings.HasPrefix(line, "Binary"):
				curFile.IsBin = true
			case strings.HasPrefix(line, "--- "):
				// Handle ambiguous filenames
				if curFile.isAmbiguous {
					// The shortest string that can end up here is:
					// "--- a\t\n" without the quotes.
					// This line has a len() of 7 but doesn't contain a oldName.
					// So the amount that the line need is at least 8 or more.
					// The code will otherwise panic for a out-of-bounds.
					if len(line) > 7 && line[4] == 'a' {
						curFile.OldName = line[6 : len(line)-1]
						if line[len(line)-2] == '\t' {
							curFile.OldName = curFile.OldName[:len(curFile.OldName)-1]
						}
					} else {
						curFile.OldName = ""
					}
				}
				// Otherwise do nothing with this line
			case strings.HasPrefix(line, "+++ "):
				// Handle ambiguous filenames
				if curFile.isAmbiguous {
					if len(line) > 6 && line[4] == 'b' {
						curFile.Name = line[6 : len(line)-1]
						if line[len(line)-2] == '\t' {
							curFile.Name = curFile.Name[:len(curFile.Name)-1]
						}
						if curFile.OldName == "" {
							curFile.OldName = curFile.Name
						}
					} else {
						curFile.Name = curFile.OldName
					}
					curFile.isAmbiguous = false
				}
				// Otherwise do nothing with this line, but now switch to parsing hunks
				nextLine, err := parseHunks(ctx, curFile, maxLines, maxLineCharacters, input)
				return string(nextLine), err
			default:
				// ignore other extended header lines
			}
		}
	}
}

// detectDecoder returns the decoder of the detected encoding of the content, nil for UTF-8 or an unknown encoding
func detectDecoder(content []byte) *encoding.Decoder {
	charsetLabel, _ := charset.DetectEncoding(content)
	if charsetLabel == "UTF-8" {
		return nil
	}
	if charsetEncoding, _ := charset.Lookup(charsetLabel); charsetEncoding != nil {
		return charsetEncoding.NewDecoder()
	}
	return nil
}

func (diff *Diff) postProcessFiles() {
	// TODO: There are numerous issues with this:
	// - we might want to consider detecting encoding while parsing but...
	// - we're likely to fail to get the correct encoding here anyway as we won't have enough information
	diffLineTypeBuffers := make(map[DiffLineType]*bytes.Buffer, 3)
	diffLineTypeDecoders := make(map[DiffLineType]*encoding.Decoder, 3)
	diffLineTypeBuffers[DiffLinePlain] = new(bytes.Buffer)
	diffLineTypeBuffers[DiffLineAdd] = new(bytes.Buffer)
	diffLineTypeBuffers[DiffLineDel] = new(bytes.Buffer)
	for _, f := range diff.Files {
		f.NameHash = git.HashFilePathForWebUI(f.Name)

		for _, buffer := range diffLineTypeBuffers {
			buffer.Reset()
		}
		for _, sec := range f.Sections {
			for _, l := range sec.Lines {
				if l.Type == DiffLineSection {
					continue
				}
				diffLineTypeBuffers[l.Type].WriteString(l.Content[1:])
				diffLineTypeBuffers[l.Type].WriteString("\n")
			}
		}
		for lineType, buffer := range diffLineTypeBuffers {
			diffLineTypeDecoders[lineType] = nil
			if buffer.Len() != 0 {
				diffLineTypeDecoders[lineType] = detectDecoder(buffer.Bytes())
			}
		}
		for _, sec := range f.Sections {
			for _, l := range sec.Lines {
				decoder := diffLineTypeDecoders[l.Type]
				if decoder != nil {
					if c, _, err := transform.String(decoder, l.Content[1:]); err == nil {
						l.Content = l.Content[0:1] + c
					}
				}
			}
		}
	}
}

func readGitDiffLineWithDiscard(r *bufio.Reader) (_ []byte, truncated bool, _ error) {
	// HINT: GIT-DIFF-PARSE-LONG-LINE: it can't use Scanner which has a default limit and will cause errors if a line is very long
	line, isPrefix, err := r.ReadLine()
	if !isPrefix {
		return line, false, err
	}

	if bytes.HasPrefix(line, []byte(cmdDiffHead)) {
		// "diff head" is special, even if it is very long, we still want to fully read it
		line = slices.Clone(line)
		lineRemaining, err := r.ReadBytes('\n')
		lineRemaining = bytes.TrimRight(lineRemaining, "\r\n")
		return append(line, lineRemaining...), false, err
	}

	// discard remaining bytes, only return the prefix
	line = slices.Clone(line)
	for isPrefix && err == nil {
		_, isPrefix, err = r.ReadLine()
	}
	return line, true, err
}

// tryFixTruncatedString tries to fix the truncated diff line by removing the last corrupted rune
func tryFixTruncatedString(s string) string {
	b := util.UnsafeStringToBytes(s)
	var idx int
	for idx = 0; idx < len(s); {
		r, l := utf8.DecodeRune(b[idx:])
		if r == utf8.RuneError && l == 1 {
			break
		}
		idx += l
	}
	// for valid utf8 diff line, remove the last truncated rune
	remainingLen := len(s) - idx
	if idx > 0 && remainingLen < utf8.UTFMax {
		return s[:idx]
	}

	// for non-utf8 diff line, try to find an ASCII char at the ending, remove the potentially truncated chars after that
	for i := 1; i <= utf8.UTFMax; i++ {
		idx = len(s) - i
		if 0 < idx && idx+1 < len(s) && s[idx] < 127 {
			return s[:idx+1]
		}
	}

	// otherwise, just return the input as is
	return s
}

func parseHunks(ctx context.Context, curFile *DiffFile, maxLines, maxLineCharacters int, input *bufio.Reader) (nextLine []byte, err error) {
	var curSection *DiffSection
	curFileLFSPrefix := false

	lastLeftIdx := -1
	leftLine, rightLine := 1, 1

	curFileLinesCount := 0
	curFileLineReachesLimit := func() bool {
		return maxLines > -1 && curFileLinesCount >= maxLines
	}

	for {
		lineBytes, truncated, err := readGitDiffLineWithDiscard(input)
		if err != nil {
			return lineBytes, err
		}
		if bytes.HasPrefix(lineBytes, []byte(cmdDiffHead)) {
			return lineBytes, err
		}

		switch lineBytes[0] {
		case '@':
			if curFileLineReachesLimit() {
				curFile.IsIncomplete = true
				continue
			}
			line := string(lineBytes)

			// Create a new section to represent this hunk
			curSection = &DiffSection{}
			lastLeftIdx = -1
			curFile.Sections = append(curFile.Sections, curSection)

			curSection.Lines = append(curSection.Lines, &DiffLine{Type: DiffLineSection, Content: line})
			curSection.FileName = curFile.Name
			leftLine, _, rightLine, _ = git.ParseDiffHunkString(line)
			continue
		case '\\':
			// This is used only to indicate that the current file does not have a terminal newline
			if !bytes.Equal(lineBytes, []byte("\\ No newline at end of file")) {
				return nil, fmt.Errorf("unexpected line in hunk: %s", string(lineBytes))
			}
			// Technically this should be the end the file!
			// FIXME: we should be putting a marker at the end of the file if there is no terminal new line
			continue
		case '+':
			curFile.Addition++
			if curFileLineReachesLimit() {
				curFile.IsIncomplete = true
				continue
			}
			curFileLinesCount++

			diffLine := &DiffLine{Type: DiffLineAdd, RightIdx: rightLine, Match: -1}
			rightLine++
			if curSection == nil {
				// Create a new section to represent this hunk
				curSection = &DiffSection{}
				curFile.Sections = append(curFile.Sections, curSection)
				lastLeftIdx = -1
			}
			if lastLeftIdx > -1 {
				diffLine.Match = lastLeftIdx
				curSection.Lines[lastLeftIdx].Match = len(curSection.Lines)
				lastLeftIdx++
				if lastLeftIdx >= len(curSection.Lines) || curSection.Lines[lastLeftIdx].Type != DiffLineDel {
					lastLeftIdx = -1
				}
			}
			curSection.Lines = append(curSection.Lines, diffLine)
		case '-':
			curFile.Deletion++
			if curFileLineReachesLimit() {
				curFile.IsIncomplete = true
				continue
			}
			curFileLinesCount++

			diffLine := &DiffLine{Type: DiffLineDel, LeftIdx: leftLine, Match: -1}
			if leftLine > 0 {
				leftLine++
			}
			if curSection == nil {
				// Create a new section to represent this hunk
				curSection = &DiffSection{}
				curFile.Sections = append(curFile.Sections, curSection)
				lastLeftIdx = -1
			}
			if len(curSection.Lines) == 0 || curSection.Lines[len(curSection.Lines)-1].Type != DiffLineDel {
				lastLeftIdx = len(curSection.Lines)
			}
			curSection.Lines = append(curSection.Lines, diffLine)
		case ' ':
			if curFileLineReachesLimit() {
				curFile.IsIncomplete = true
				continue
			}
			curFileLinesCount++
			diffLine := &DiffLine{Type: DiffLinePlain, LeftIdx: leftLine, RightIdx: rightLine}
			leftLine++
			rightLine++
			lastLeftIdx = -1
			if curSection == nil {
				// Create a new section to represent this hunk
				curSection = &DiffSection{}
				curFile.Sections = append(curFile.Sections, curSection)
			}
			curSection.Lines = append(curSection.Lines, diffLine)
		default:
			// This is unexpected
			return nil, fmt.Errorf("unexpected line in hunk: %s", string(lineBytes))
		}

		curLine := curSection.Lines[len(curSection.Lines)-1]
		curLine.IsTruncated = truncated

		line := string(lineBytes)
		if len(line) > maxLineCharacters {
			line = line[:maxLineCharacters]
			curLine.IsTruncated = true
		}
		curLine.Content = line
		if curLine.IsTruncated {
			curLine.Content = tryFixTruncatedString(curLine.Content)
		}

		// handle LFS
		if line[1:] == lfs.MetaFileIdentifier {
			curFileLFSPrefix = true
		} else if curFileLFSPrefix && strings.HasPrefix(line[1:], lfs.MetaFileOidPrefix) {
			oid := strings.TrimPrefix(line[1:], lfs.MetaFileOidPrefix)
			if len(oid) == 64 {
				m := &git_model.LFSMetaObject{Pointer: lfs.Pointer{Oid: oid}}
				count, err := db.CountByBean(ctx, m)

				if err == nil && count > 0 {
					curFile.IsBin = true
					curSection.Lines = nil
					lastLeftIdx = -1
				}
			}
		}
	}
}

func createDiffFile(line string) *DiffFile {
	// The a/ and b/ filenames are the same unless rename/copy is involved.
	// Especially, even for a creation or a deletion, /dev/null is not used
	// in place of the a/ or b/ filenames.
	//
	// When rename/copy is involved, file1 and file2 show the name of the
	// source file of the rename/copy and the name of the file that rename/copy
	// produces, respectively.
	//
	// Path names are quoted if necessary.
	//
	// This means that you should always be able to determine the file name even when
	// there is potential ambiguity...
	//
	// but we can be simpler with our heuristics by just forcing git to prefix things nicely
	curFile := &DiffFile{
		Type:     DiffFileChange,
		Sections: make([]*DiffSection, 0, 10),
	}

	rd := strings.NewReader(line[len(cmdDiffHead):] + " ")
	curFile.Type = DiffFileChange
	var oldNameAmbiguity, newNameAmbiguity bool

	curFile.OldName, oldNameAmbiguity = readFileName(rd)
	curFile.Name, newNameAmbiguity = readFileName(rd)
	if oldNameAmbiguity && newNameAmbiguity {
		curFile.isAmbiguous = true
		// OK we should bet that the oldName and the newName are the same if they can be made to be same
		// So we need to start again ...
		if (len(line)-len(cmdDiffHead)-1)%2 == 0 {
			// diff --git a/b b/b b/b b/b b/b b/b
			//
			midpoint := (len(line) + len(cmdDiffHead) - 1) / 2
			newPart, oldPart := line[len(cmdDiffHead):midpoint], line[midpoint+1:]
			if len(newPart) > 2 && len(oldPart) > 2 && newPart[2:] == oldPart[2:] {
				curFile.OldName = oldPart[2:]
				curFile.Name = oldPart[2:]
			}
		}
	}

	curFile.IsRenamed = curFile.Name != curFile.OldName
	return curFile
}

func readFileName(rd *strings.Reader) (string, bool) {
	ambiguity := false
	var name string
	char, _ := rd.ReadByte()
	_ = rd.UnreadByte()
	if char == '"' {
		_, _ = fmt.Fscanf(rd, "%q ", &name)
		if len(name) == 0 {
			log.Error("Reader has no file name: reader=%+v", rd)
			return "", true
		}

		if name[0] == '\\' {
			name = name[1:]
		}
	} else {
		// This technique is potentially ambiguous it may not be possible to uniquely identify the filenames from the diff line alone
		ambiguity = true
		_, _ = fmt.Fscanf(rd, "%s ", &name)
		char, _ := rd.ReadByte()
		_ = rd.UnreadByte()
		for !(char == 0 || char == '"' || char == 'b') {
			var suffix string
			_, _ = fmt.Fscanf(rd, "%s ", &suffix)
			name += " " + suffix
			char, _ = rd.ReadByte()
			_ = rd.UnreadByte()
		}
	}
	if len(name) < 2 {
		log.Error("Unable to determine name from reader: reader=%+v", rd)
		return "", true
	}
	return name[2:], ambiguity
}

type DiffCommonOptions struct {
	BeforeCommitID     string
	AfterCommitID      string
	WhitespaceBehavior gitcmd.TrustedCmdArgs
}

type DiffOptions struct {
	DiffCommonOptions
	SkipTo            string
	MaxLines          int
	MaxLineCharacters int
	MaxFiles          int
}

// prepareDiffCommits prepares the before and after commits for a diff operation based on the provided options.
// The "before commit" can be nil (the empty tree ID is used) if there is no "before commit" can be determined.
func prepareDiffCommits(ctx context.Context, gitRepo *git.Repository, opts *DiffCommonOptions) (actualBeforeCommit *git.Commit, actualBeforeCommitID git.ObjectID, afterCommit *git.Commit, err error) {
	afterCommit, err = gitRepo.GetCommit(ctx, opts.AfterCommitID)
	if err != nil {
		return nil, nil, nil, err
	}

	commitObjectFormat := afterCommit.ID.Type()
	isBeforeCommitIDEmpty := opts.BeforeCommitID == "" || opts.BeforeCommitID == commitObjectFormat.EmptyObjectID().String()
	if isBeforeCommitIDEmpty && afterCommit.ParentCount() == 0 {
		// "git diff 4b825dc642cb6eb9a060e54bf8d69288fbee4904 after-commit" can work with tree ID as before commit ID
		actualBeforeCommitID = commitObjectFormat.EmptyTree()
	} else {
		if isBeforeCommitIDEmpty {
			actualBeforeCommit, err = afterCommit.Parent(ctx, gitRepo, 0)
		} else {
			actualBeforeCommit, err = gitRepo.GetCommit(ctx, opts.BeforeCommitID)
		}
		if err != nil {
			return nil, nil, nil, err
		}
		actualBeforeCommitID = actualBeforeCommit.ID
	}
	return actualBeforeCommit, actualBeforeCommitID, afterCommit, nil
}

// GetDiffForAPI builds a Diff between two commits of a repository.
// Passing the empty string as beforeCommitID returns a diff from the parent commit.
// The whitespaceBehavior is either an empty string or a git flag
func GetDiffForAPI(ctx context.Context, gitRepo *git.Repository, opts *DiffOptions, files ...string) (*Diff, error) {
	_, beforeCommitID, _, err := prepareDiffCommits(ctx, gitRepo, &opts.DiffCommonOptions)
	if err != nil {
		return nil, err
	}

	// HINT: GIT-DIFF-HIGHLIGHT-LINE-NUMBER: git doesn't treat CR(\r) as EOL, CR is just a plain char which can appear anywhere in the diff output
	// Since we have to do full-file-highlighting for the diff result, we need to make sure the highlighted lines exactly match the git's diff output.
	cmdDiff := gitcmd.NewCommand().
		AddArguments("diff", "--src-prefix=\\a/", "--dst-prefix=\\b/").
		AddArguments(opts.WhitespaceBehavior...).
		AddOptionFormat("--find-renames=%s", setting.Git.DiffRenameSimilarityThreshold)

	if opts.SkipTo != "" {
		cmdDiff.AddOptionFormat("--skip-to=%s", opts.SkipTo)
	}

	cmdDiff.AddDynamicArguments(beforeCommitID.String(), opts.AfterCommitID)
	cmdDiff.AddDashesAndList(files...)

	cmdCtx, cmdCancel := context.WithCancel(ctx)
	defer cmdCancel()

	reader, readerClose := cmdDiff.MakeStdoutPipe()
	defer readerClose()
	go func() {
		if err := cmdDiff.
			WithRepo(gitRepo).
			RunWithStderr(cmdCtx); err != nil && !gitcmd.IsErrorCanceledOrKilled(err) {
			log.Error("error during GetDiff(git diff dir: %s): %v", gitRepo.LogString(), err)
		}
	}()

	diff, err := ParsePatch(cmdCtx, opts.MaxLines, opts.MaxLineCharacters, opts.MaxFiles, reader)
	// Ensure the git process is killed if it didn't exit already
	cmdCancel()
	if err != nil {
		return nil, fmt.Errorf("unable to ParsePatch: %w", err)
	}
	diff.Start = opts.SkipTo
	return diff, nil
}

type DiffShortStat struct {
	NumFiles, TotalAddition, TotalDeletion int // these fields are used in templates directly
}

func GetDiffShortStat(ctx context.Context, gitRepo *git.Repository, opts *DiffCommonOptions) (*DiffShortStat, error) {
	_, actualBeforeCommitID, afterCommit, err := prepareDiffCommits(ctx, gitRepo, opts)
	if err != nil {
		return nil, err
	}

	stat := &DiffShortStat{}
	cmd := gitcmd.NewCommand("diff", "--shortstat").
		AddArguments(opts.WhitespaceBehavior...).
		AddOptionFormat("--find-renames=%s", setting.Git.DiffRenameSimilarityThreshold).
		AddDynamicArguments(actualBeforeCommitID.String(), afterCommit.ID.String())
	// output: "  9902 files changed, 2034198 insertions(+), 298800 deletions(-)\n"
	stdout, _, err := cmd.WithRepo(gitRepo).RunStdString(ctx)
	if err != nil {
		return nil, err
	}
	stdout = strings.TrimSpace(stdout)
	for field := range strings.SplitSeq(stdout, ",") {
		field = strings.TrimSpace(field)
		num, suffix, ok := strings.Cut(field, " ")
		switch {
		case strings.Contains(suffix, "file") && strings.Contains(suffix, "change"):
			stat.NumFiles, _ = strconv.Atoi(num)
		case strings.Contains(suffix, "insertion"):
			stat.TotalAddition, _ = strconv.Atoi(num)
		case strings.Contains(suffix, "deletion"):
			stat.TotalDeletion, _ = strconv.Atoi(num)
		case ok:
			setting.PanicInDevOrTesting("unexpected diff shortstat output: %s", stdout)
		}
	}
	return stat, nil
}

// SyncUserSpecificDiff returns the user's review state of the given files, in which files changed since the last review are marked as HasChanged.
// The database is updated if files have changed since the last review.
func SyncUserSpecificDiff(ctx context.Context, userID int64, pull *issues_model.PullRequest, gitRepo *git.Repository, fileNames []string, afterCommitID string) (*pull_model.ReviewState, error) {
	review, err := pull_model.GetNewestReviewState(ctx, userID, pull.ID)
	if err != nil {
		return nil, err
	}
	if review == nil || len(review.UpdatedFiles) == 0 {
		return review, nil
	}

	latestCommit := afterCommitID
	if latestCommit == "" {
		latestCommit = pull.HeadBranch // afterCommitID is preferred because it handles PRs from forks correctly and the branch name doesn't
	}

	changedFiles, errIgnored := gitRepo.GetFilesChangedBetween(ctx, review.CommitSHA, latestCommit)
	// There are way too many possible errors.
	// Examples are various git errors such as the commit the review was based on was gc'ed and hence doesn't exist anymore as well as unrecoverable errors where we should serve a 500 response
	// Due to the current architecture and physical limitation of needing to compare explicit error messages, we can only choose one approach without the code getting ugly
	// For SOME of the errors such as the gc'ed commit, it would be best to mark all files as changed
	// But as that does not work for all potential errors, we simply mark all files as unchanged and drop the error which always works, even if not as good as possible
	if errIgnored != nil {
		log.Error("Could not get changed files between %s and %s for pull request %d in repo with path %s. Assuming no changes. Error: %w", review.CommitSHA, latestCommit, pull.Index, gitRepo.LogString(), err)
	}
	changedFilesSet := make(map[string]struct{}, len(changedFiles))
	for _, changedFile := range changedFiles {
		changedFilesSet[changedFile] = struct{}{}
	}

	filesChangedSinceLastDiff := make(map[string]pull_model.ViewedState)
	for _, filename := range fileNames {
		fileViewedState := review.UpdatedFiles[filename]
		if fileViewedState == pull_model.HasChanged { // Check whether it was previously detected that the file has changed since the last review
			delete(changedFilesSet, filename)
		} else if _, ok := changedFilesSet[filename]; ok { // Check explicitly whether the file has changed since the last review
			filesChangedSinceLastDiff[filename] = pull_model.HasChanged
			delete(changedFilesSet, filename)
		}
	}

	// All changed files still present at this point aren't part of the diff anymore, this occurs
	// when a file was modified in a previous commit of the diff and the modification got reverted afterwards.
	// Marking the files as unviewed to prevent errors where a non-existing file has a view state
	for changedFile := range changedFilesSet {
		if _, ok := review.UpdatedFiles[changedFile]; ok {
			filesChangedSinceLastDiff[changedFile] = pull_model.Unviewed
		}
	}

	if len(filesChangedSinceLastDiff) > 0 {
		// Explicitly store files that have changed in the database, if any is present at all.
		// This has the benefit that the "Has Changed" attribute will be present as long as the user does not explicitly mark this file as viewed, so it will even survive a page reload after marking another file as viewed.
		updatedReview, err := pull_model.UpdateReviewState(ctx, review.UserID, review.PullID, review.CommitSHA, filesChangedSinceLastDiff)
		if err != nil {
			log.Warn("Could not update review for user %d, pull %d, commit %s and the changed files %v: %v", review.UserID, review.PullID, review.CommitSHA, filesChangedSinceLastDiff, err)
			return nil, err
		}
		// Update the local review to reflect the changes immediately
		review = updatedReview
	}

	return review, nil
}

// GeneratePatchForUnchangedLine creates a patch showing code context for an unchanged line
func GeneratePatchForUnchangedLine(ctx context.Context, gitRepo *git.Repository, commitID, treePath string, line int64, contextLines int) (string, error) {
	commit, err := gitRepo.GetCommit(ctx, commitID)
	if err != nil {
		return "", fmt.Errorf("GetCommit: %w", err)
	}

	entry, err := commit.GetTreeEntryByPath(ctx, gitRepo, treePath)
	if err != nil {
		return "", fmt.Errorf("GetTreeEntryByPath: %w", err)
	}

	blob := entry.Blob(gitRepo)
	dataRc, err := blob.DataAsync(ctx)
	if err != nil {
		return "", fmt.Errorf("DataAsync: %w", err)
	}
	defer dataRc.Close()

	return generatePatchForUnchangedLineFromReader(dataRc, treePath, line, contextLines)
}

// generatePatchForUnchangedLineFromReader is the testable core logic that generates a patch from a reader
func generatePatchForUnchangedLineFromReader(reader io.Reader, treePath string, line int64, contextLines int) (string, error) {
	// Calculate line range (commented line + lines above it)
	commentLine := int(line)
	if line < 0 {
		commentLine = int(-line)
	}
	startLine := max(commentLine-contextLines, 1)
	endLine := commentLine

	// Read only the needed lines efficiently
	scanner := bufio.NewScanner(reader)
	currentLine := 0
	var lines []string
	for scanner.Scan() {
		currentLine++
		if currentLine >= startLine && currentLine <= endLine {
			lines = append(lines, scanner.Text())
		}
		if currentLine > endLine {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scanner error: %w", err)
	}

	if len(lines) == 0 {
		return "", fmt.Errorf("no lines found in range %d-%d", startLine, endLine)
	}

	// Generate synthetic patch
	var patchBuilder strings.Builder
	fmt.Fprintf(&patchBuilder, "diff --git a/%s b/%s\n", treePath, treePath)
	fmt.Fprintf(&patchBuilder, "--- a/%s\n", treePath)
	fmt.Fprintf(&patchBuilder, "+++ b/%s\n", treePath)
	fmt.Fprintf(&patchBuilder, "@@ -%d,%d +%d,%d @@\n", startLine, len(lines), startLine, len(lines))

	for _, lineContent := range lines {
		patchBuilder.WriteString(" ")
		patchBuilder.WriteString(lineContent)
		patchBuilder.WriteString("\n")
	}

	return patchBuilder.String(), nil
}

// GetWhitespaceFlag returns git diff flag for treating whitespaces
func GetWhitespaceFlag(whitespaceBehavior string) gitcmd.TrustedCmdArgs {
	whitespaceFlags := map[string]gitcmd.TrustedCmdArgs{
		"ignore-all":    {"-w"},
		"ignore-change": {"-b"},
		"ignore-eol":    {"--ignore-space-at-eol"},
		"show-all":      nil,
	}
	if flag, ok := whitespaceFlags[whitespaceBehavior]; ok {
		return flag
	}
	return nil
}
