// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"bufio"
	"bytes"
	"fmt"
	"html"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	"github.com/Unknwon/com"
	"github.com/sergi/go-diff/diffmatchpatch"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

// DiffLineType represents the type of a DiffLine.
type DiffLineType uint8

// DiffLineType possible values.
const (
	DiffLinePlain DiffLineType = iota + 1
	DiffLineAdd
	DiffLineDel
	DiffLineSection
)

// DiffFileType represents the type of a DiffFile.
type DiffFileType uint8

// DiffFileType possible values.
const (
	DiffFileAdd DiffFileType = iota + 1
	DiffFileChange
	DiffFileDel
	DiffFileRename
)

// DiffLine represents a line difference in a DiffSection.
type DiffLine struct {
	LeftIdx  int
	RightIdx int
	Type     DiffLineType
	Content  string
}

// GetType returns the type of a DiffLine.
func (d *DiffLine) GetType() int {
	return int(d.Type)
}

// DiffSection represents a section of a DiffFile.
type DiffSection struct {
	Name  string
	Lines []*DiffLine
}

var (
	addedCodePrefix   = []byte("<span class=\"added-code\">")
	removedCodePrefix = []byte("<span class=\"removed-code\">")
	codeTagSuffix     = []byte("</span>")
)

func diffToHTML(diffs []diffmatchpatch.Diff, lineType DiffLineType) template.HTML {
	buf := bytes.NewBuffer(nil)

	// Reproduce signs which are cut for inline diff before.
	switch lineType {
	case DiffLineAdd:
		buf.WriteByte('+')
	case DiffLineDel:
		buf.WriteByte('-')
	}

	for i := range diffs {
		switch {
		case diffs[i].Type == diffmatchpatch.DiffInsert && lineType == DiffLineAdd:
			buf.Write(addedCodePrefix)
			buf.WriteString(html.EscapeString(diffs[i].Text))
			buf.Write(codeTagSuffix)
		case diffs[i].Type == diffmatchpatch.DiffDelete && lineType == DiffLineDel:
			buf.Write(removedCodePrefix)
			buf.WriteString(html.EscapeString(diffs[i].Text))
			buf.Write(codeTagSuffix)
		case diffs[i].Type == diffmatchpatch.DiffEqual:
			buf.WriteString(html.EscapeString(diffs[i].Text))
		}
	}

	return template.HTML(buf.Bytes())
}

// GetLine gets a specific line by type (add or del) and file line number
func (diffSection *DiffSection) GetLine(lineType DiffLineType, idx int) *DiffLine {
	var (
		difference    = 0
		addCount      = 0
		delCount      = 0
		matchDiffLine *DiffLine
	)

LOOP:
	for _, diffLine := range diffSection.Lines {
		switch diffLine.Type {
		case DiffLineAdd:
			addCount++
		case DiffLineDel:
			delCount++
		default:
			if matchDiffLine != nil {
				break LOOP
			}
			difference = diffLine.RightIdx - diffLine.LeftIdx
			addCount = 0
			delCount = 0
		}

		switch lineType {
		case DiffLineDel:
			if diffLine.RightIdx == 0 && diffLine.LeftIdx == idx-difference {
				matchDiffLine = diffLine
			}
		case DiffLineAdd:
			if diffLine.LeftIdx == 0 && diffLine.RightIdx == idx+difference {
				matchDiffLine = diffLine
			}
		}
	}

	if addCount == delCount {
		return matchDiffLine
	}
	return nil
}

var diffMatchPatch = diffmatchpatch.New()

func init() {
	diffMatchPatch.DiffEditCost = 100
}

// GetComputedInlineDiffFor computes inline diff for the given line.
func (diffSection *DiffSection) GetComputedInlineDiffFor(diffLine *DiffLine) template.HTML {
	if setting.Git.DisableDiffHighlight {
		return template.HTML(html.EscapeString(diffLine.Content[1:]))
	}
	var (
		compareDiffLine *DiffLine
		diff1           string
		diff2           string
	)

	// try to find equivalent diff line. ignore, otherwise
	switch diffLine.Type {
	case DiffLineAdd:
		compareDiffLine = diffSection.GetLine(DiffLineDel, diffLine.RightIdx)
		if compareDiffLine == nil {
			return template.HTML(html.EscapeString(diffLine.Content))
		}
		diff1 = compareDiffLine.Content
		diff2 = diffLine.Content
	case DiffLineDel:
		compareDiffLine = diffSection.GetLine(DiffLineAdd, diffLine.LeftIdx)
		if compareDiffLine == nil {
			return template.HTML(html.EscapeString(diffLine.Content))
		}
		diff1 = diffLine.Content
		diff2 = compareDiffLine.Content
	default:
		return template.HTML(html.EscapeString(diffLine.Content))
	}

	diffRecord := diffMatchPatch.DiffMain(diff1[1:], diff2[1:], true)
	diffRecord = diffMatchPatch.DiffCleanupEfficiency(diffRecord)

	return diffToHTML(diffRecord, diffLine.Type)
}

// DiffFile represents a file diff.
type DiffFile struct {
	Name               string
	OldName            string
	Index              int
	Addition, Deletion int
	Type               DiffFileType
	IsCreated          bool
	IsDeleted          bool
	IsBin              bool
	IsLFSFile          bool
	IsRenamed          bool
	IsSubmodule        bool
	Sections           []*DiffSection
	IsIncomplete       bool
}

// GetType returns type of diff file.
func (diffFile *DiffFile) GetType() int {
	return int(diffFile.Type)
}

// GetHighlightClass returns highlight class for a filename.
func (diffFile *DiffFile) GetHighlightClass() string {
	return highlight.FileNameToHighlightClass(diffFile.Name)
}

// Diff represents a difference between two git trees.
type Diff struct {
	TotalAddition, TotalDeletion int
	Files                        []*DiffFile
	IsIncomplete                 bool
}

// NumFiles returns number of files changes in a diff.
func (diff *Diff) NumFiles() int {
	return len(diff.Files)
}

const cmdDiffHead = "diff --git "

// ParsePatch builds a Diff object from a io.Reader and some
// parameters.
// TODO: move this function to gogits/git-module
func ParsePatch(maxLines, maxLineCharacters, maxFiles int, reader io.Reader) (*Diff, error) {
	var (
		diff = &Diff{Files: make([]*DiffFile, 0)}

		curFile    *DiffFile
		curSection = &DiffSection{
			Lines: make([]*DiffLine, 0, 10),
		}

		leftLine, rightLine int
		lineCount           int
		curFileLinesCount   int
		curFileLFSPrefix    bool
	)

	input := bufio.NewReader(reader)
	isEOF := false
	for !isEOF {
		line, err := input.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				isEOF = true
			} else {
				return nil, fmt.Errorf("ReadString: %v", err)
			}
		}

		if len(line) > 0 && line[len(line)-1] == '\n' {
			// Remove line break.
			line = line[:len(line)-1]
		}

		if strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- ") || len(line) == 0 {
			continue
		}

		trimLine := strings.Trim(line, "+- ")

		if trimLine == LFSMetaFileIdentifier {
			curFileLFSPrefix = true
		}

		if curFileLFSPrefix && strings.HasPrefix(trimLine, LFSMetaFileOidPrefix) {
			oid := strings.TrimPrefix(trimLine, LFSMetaFileOidPrefix)

			if len(oid) == 64 {
				m := &LFSMetaObject{Oid: oid}
				count, err := x.Count(m)

				if err == nil && count > 0 {
					curFile.IsBin = true
					curFile.IsLFSFile = true
					curSection.Lines = nil
				}
			}
		}

		curFileLinesCount++
		lineCount++

		// Diff data too large, we only show the first about maxLines lines
		if curFileLinesCount >= maxLines || len(line) >= maxLineCharacters {
			curFile.IsIncomplete = true
		}

		switch {
		case line[0] == ' ':
			diffLine := &DiffLine{Type: DiffLinePlain, Content: line, LeftIdx: leftLine, RightIdx: rightLine}
			leftLine++
			rightLine++
			curSection.Lines = append(curSection.Lines, diffLine)
			continue
		case line[0] == '@':
			curSection = &DiffSection{}
			curFile.Sections = append(curFile.Sections, curSection)
			ss := strings.Split(line, "@@")
			diffLine := &DiffLine{Type: DiffLineSection, Content: line}
			curSection.Lines = append(curSection.Lines, diffLine)

			// Parse line number.
			ranges := strings.Split(ss[1][1:], " ")
			leftLine, _ = com.StrTo(strings.Split(ranges[0], ",")[0][1:]).Int()
			if len(ranges) > 1 {
				rightLine, _ = com.StrTo(strings.Split(ranges[1], ",")[0]).Int()
			} else {
				log.Warn("Parse line number failed: %v", line)
				rightLine = leftLine
			}
			continue
		case line[0] == '+':
			curFile.Addition++
			diff.TotalAddition++
			diffLine := &DiffLine{Type: DiffLineAdd, Content: line, RightIdx: rightLine}
			rightLine++
			curSection.Lines = append(curSection.Lines, diffLine)
			continue
		case line[0] == '-':
			curFile.Deletion++
			diff.TotalDeletion++
			diffLine := &DiffLine{Type: DiffLineDel, Content: line, LeftIdx: leftLine}
			if leftLine > 0 {
				leftLine++
			}
			curSection.Lines = append(curSection.Lines, diffLine)
		case strings.HasPrefix(line, "Binary"):
			curFile.IsBin = true
			continue
		}

		// Get new file.
		if strings.HasPrefix(line, cmdDiffHead) {
			middle := -1

			// Note: In case file name is surrounded by double quotes (it happens only in git-shell).
			// e.g. diff --git "a/xxx" "b/xxx"
			hasQuote := line[len(cmdDiffHead)] == '"'
			if hasQuote {
				middle = strings.Index(line, ` "b/`)
			} else {
				middle = strings.Index(line, " b/")
			}

			beg := len(cmdDiffHead)
			a := line[beg+2 : middle]
			b := line[middle+3:]
			if hasQuote {
				a = string(git.UnescapeChars([]byte(a[1 : len(a)-1])))
				b = string(git.UnescapeChars([]byte(b[1 : len(b)-1])))
			}

			curFile = &DiffFile{
				Name:      b,
				OldName:   a,
				Index:     len(diff.Files) + 1,
				Type:      DiffFileChange,
				Sections:  make([]*DiffSection, 0, 10),
				IsRenamed: a != b,
			}
			diff.Files = append(diff.Files, curFile)
			if len(diff.Files) >= maxFiles {
				diff.IsIncomplete = true
				io.Copy(ioutil.Discard, reader)
				break
			}
			curFileLinesCount = 0
			curFileLFSPrefix = false

			// Check file diff type and is submodule.
			for {
				line, err := input.ReadString('\n')
				if err != nil {
					if err == io.EOF {
						isEOF = true
					} else {
						return nil, fmt.Errorf("ReadString: %v", err)
					}
				}

				switch {
				case strings.HasPrefix(line, "new file"):
					curFile.Type = DiffFileAdd
					curFile.IsCreated = true
				case strings.HasPrefix(line, "deleted"):
					curFile.Type = DiffFileDel
					curFile.IsDeleted = true
				case strings.HasPrefix(line, "index"):
					curFile.Type = DiffFileChange
				case strings.HasPrefix(line, "similarity index 100%"):
					curFile.Type = DiffFileRename
				}
				if curFile.Type > 0 {
					if strings.HasSuffix(line, " 160000\n") {
						curFile.IsSubmodule = true
					}
					break
				}
			}
		}
	}

	// FIXME: detect encoding while parsing.
	var buf bytes.Buffer
	for _, f := range diff.Files {
		buf.Reset()
		for _, sec := range f.Sections {
			for _, l := range sec.Lines {
				buf.WriteString(l.Content)
				buf.WriteString("\n")
			}
		}
		charsetLabel, err := base.DetectEncoding(buf.Bytes())
		if charsetLabel != "UTF-8" && err == nil {
			encoding, _ := charset.Lookup(charsetLabel)
			if encoding != nil {
				d := encoding.NewDecoder()
				for _, sec := range f.Sections {
					for _, l := range sec.Lines {
						if c, _, err := transform.String(d, l.Content); err == nil {
							l.Content = c
						}
					}
				}
			}
		}
	}
	return diff, nil
}

// GetDiffRange builds a Diff between two commits of a repository.
// passing the empty string as beforeCommitID returns a diff from the
// parent commit.
func GetDiffRange(repoPath, beforeCommitID, afterCommitID string, maxLines, maxLineCharacters, maxFiles int) (*Diff, error) {
	gitRepo, err := git.OpenRepository(repoPath)
	if err != nil {
		return nil, err
	}

	commit, err := gitRepo.GetCommit(afterCommitID)
	if err != nil {
		return nil, err
	}

	var cmd *exec.Cmd
	// if "after" commit given
	if len(beforeCommitID) == 0 {
		// First commit of repository.
		if commit.ParentCount() == 0 {
			cmd = exec.Command("git", "show", afterCommitID)
		} else {
			c, _ := commit.Parent(0)
			cmd = exec.Command("git", "diff", "-M", c.ID.String(), afterCommitID)
		}
	} else {
		cmd = exec.Command("git", "diff", "-M", beforeCommitID, afterCommitID)
	}
	cmd.Dir = repoPath
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("StdoutPipe: %v", err)
	}

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("Start: %v", err)
	}

	pid := process.GetManager().Add(fmt.Sprintf("GetDiffRange [repo_path: %s]", repoPath), cmd)
	defer process.GetManager().Remove(pid)

	diff, err := ParsePatch(maxLines, maxLineCharacters, maxFiles, stdout)
	if err != nil {
		return nil, fmt.Errorf("ParsePatch: %v", err)
	}

	if err = cmd.Wait(); err != nil {
		return nil, fmt.Errorf("Wait: %v", err)
	}

	return diff, nil
}

// RawDiffType type of a raw diff.
type RawDiffType string

// RawDiffType possible values.
const (
	RawDiffNormal RawDiffType = "diff"
	RawDiffPatch  RawDiffType = "patch"
)

// GetRawDiff dumps diff results of repository in given commit ID to io.Writer.
// TODO: move this function to gogits/git-module
func GetRawDiff(repoPath, commitID string, diffType RawDiffType, writer io.Writer) error {
	repo, err := git.OpenRepository(repoPath)
	if err != nil {
		return fmt.Errorf("OpenRepository: %v", err)
	}

	commit, err := repo.GetCommit(commitID)
	if err != nil {
		return fmt.Errorf("GetCommit: %v", err)
	}

	var cmd *exec.Cmd
	switch diffType {
	case RawDiffNormal:
		if commit.ParentCount() == 0 {
			cmd = exec.Command("git", "show", commitID)
		} else {
			c, _ := commit.Parent(0)
			cmd = exec.Command("git", "diff", "-M", c.ID.String(), commitID)
		}
	case RawDiffPatch:
		if commit.ParentCount() == 0 {
			cmd = exec.Command("git", "format-patch", "--no-signature", "--stdout", "--root", commitID)
		} else {
			c, _ := commit.Parent(0)
			query := fmt.Sprintf("%s...%s", commitID, c.ID.String())
			cmd = exec.Command("git", "format-patch", "--no-signature", "--stdout", query)
		}
	default:
		return fmt.Errorf("invalid diffType: %s", diffType)
	}

	stderr := new(bytes.Buffer)

	cmd.Dir = repoPath
	cmd.Stdout = writer
	cmd.Stderr = stderr

	if err = cmd.Run(); err != nil {
		return fmt.Errorf("Run: %v - %s", err, stderr)
	}
	return nil
}

// GetDiffCommit builds a Diff representing the given commitID.
func GetDiffCommit(repoPath, commitID string, maxLines, maxLineCharacters, maxFiles int) (*Diff, error) {
	return GetDiffRange(repoPath, "", commitID, maxLines, maxLineCharacters, maxFiles)
}
