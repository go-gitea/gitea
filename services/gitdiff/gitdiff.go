// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitdiff

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"html"
	"html/template"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/unknwon/com"
	stdcharset "golang.org/x/net/html/charset"
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

// DiffLineExpandDirection represents the DiffLineSection expand direction
type DiffLineExpandDirection uint8

// DiffLineExpandDirection possible values.
const (
	DiffLineExpandNone DiffLineExpandDirection = iota + 1
	DiffLineExpandSingle
	DiffLineExpandUpDown
	DiffLineExpandUp
	DiffLineExpandDown
)

// DiffLine represents a line difference in a DiffSection.
type DiffLine struct {
	LeftIdx     int
	RightIdx    int
	Type        DiffLineType
	Content     string
	Comments    []*models.Comment
	SectionInfo *DiffLineSectionInfo
}

// DiffLineSectionInfo represents diff line section meta data
type DiffLineSectionInfo struct {
	Path          string
	LastLeftIdx   int
	LastRightIdx  int
	LeftIdx       int
	RightIdx      int
	LeftHunkSize  int
	RightHunkSize int
}

// BlobExceprtChunkSize represent max lines of excerpt
const BlobExceprtChunkSize = 20

// GetType returns the type of a DiffLine.
func (d *DiffLine) GetType() int {
	return int(d.Type)
}

// CanComment returns whether or not a line can get commented
func (d *DiffLine) CanComment() bool {
	return len(d.Comments) == 0 && d.Type != DiffLineSection
}

// GetCommentSide returns the comment side of the first comment, if not set returns empty string
func (d *DiffLine) GetCommentSide() string {
	if len(d.Comments) == 0 {
		return ""
	}
	return d.Comments[0].DiffSide()
}

// GetLineTypeMarker returns the line type marker
func (d *DiffLine) GetLineTypeMarker() string {
	if strings.IndexByte(" +-", d.Content[0]) > -1 {
		return d.Content[0:1]
	}
	return ""
}

// GetBlobExcerptQuery builds query string to get blob excerpt
func (d *DiffLine) GetBlobExcerptQuery() string {
	query := fmt.Sprintf(
		"last_left=%d&last_right=%d&"+
			"left=%d&right=%d&"+
			"left_hunk_size=%d&right_hunk_size=%d&"+
			"path=%s",
		d.SectionInfo.LastLeftIdx, d.SectionInfo.LastRightIdx,
		d.SectionInfo.LeftIdx, d.SectionInfo.RightIdx,
		d.SectionInfo.LeftHunkSize, d.SectionInfo.RightHunkSize,
		url.QueryEscape(d.SectionInfo.Path))
	return query
}

// GetExpandDirection gets DiffLineExpandDirection
func (d *DiffLine) GetExpandDirection() DiffLineExpandDirection {
	if d.Type != DiffLineSection || d.SectionInfo == nil || d.SectionInfo.RightIdx-d.SectionInfo.LastRightIdx <= 1 {
		return DiffLineExpandNone
	}
	if d.SectionInfo.LastLeftIdx <= 0 && d.SectionInfo.LastRightIdx <= 0 {
		return DiffLineExpandUp
	} else if d.SectionInfo.RightIdx-d.SectionInfo.LastRightIdx > BlobExceprtChunkSize && d.SectionInfo.RightHunkSize > 0 {
		return DiffLineExpandUpDown
	} else if d.SectionInfo.LeftHunkSize <= 0 && d.SectionInfo.RightHunkSize <= 0 {
		return DiffLineExpandDown
	}
	return DiffLineExpandSingle
}

func getDiffLineSectionInfo(curFile *DiffFile, line string, lastLeftIdx, lastRightIdx int) *DiffLineSectionInfo {
	var (
		leftLine  int
		leftHunk  int
		rightLine int
		righHunk  int
	)
	ss := strings.Split(line, "@@")
	ranges := strings.Split(ss[1][1:], " ")
	leftRange := strings.Split(ranges[0], ",")
	leftLine, _ = com.StrTo(leftRange[0][1:]).Int()
	if len(leftRange) > 1 {
		leftHunk, _ = com.StrTo(leftRange[1]).Int()
	}
	if len(ranges) > 1 {
		rightRange := strings.Split(ranges[1], ",")
		rightLine, _ = com.StrTo(rightRange[0]).Int()
		if len(rightRange) > 1 {
			righHunk, _ = com.StrTo(rightRange[1]).Int()
		}
	} else {
		log.Warn("Parse line number failed: %v", line)
		rightLine = leftLine
		righHunk = leftHunk
	}
	return &DiffLineSectionInfo{
		Path:          curFile.Name,
		LastLeftIdx:   lastLeftIdx,
		LastRightIdx:  lastRightIdx,
		LeftIdx:       leftLine,
		RightIdx:      rightLine,
		LeftHunkSize:  leftHunk,
		RightHunkSize: righHunk,
	}
}

// escape a line's content or return <br> needed for copy/paste purposes
func getLineContent(content string) string {
	if len(content) > 0 {
		return html.EscapeString(content)
	}
	return "<br>"
}

// DiffSection represents a section of a DiffFile.
type DiffSection struct {
	Name  string
	Lines []*DiffLine
}

var (
	addedCodePrefix   = []byte(`<span class="added-code">`)
	removedCodePrefix = []byte(`<span class="removed-code">`)
	codeTagSuffix     = []byte(`</span>`)
)

func diffToHTML(diffs []diffmatchpatch.Diff, lineType DiffLineType) template.HTML {
	buf := bytes.NewBuffer(nil)

	for i := range diffs {
		switch {
		case diffs[i].Type == diffmatchpatch.DiffInsert && lineType == DiffLineAdd:
			buf.Write(addedCodePrefix)
			buf.WriteString(getLineContent(diffs[i].Text))
			buf.Write(codeTagSuffix)
		case diffs[i].Type == diffmatchpatch.DiffDelete && lineType == DiffLineDel:
			buf.Write(removedCodePrefix)
			buf.WriteString(getLineContent(diffs[i].Text))
			buf.Write(codeTagSuffix)
		case diffs[i].Type == diffmatchpatch.DiffEqual:
			buf.WriteString(getLineContent(diffs[i].Text))
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
		return template.HTML(getLineContent(diffLine.Content[1:]))
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
			return template.HTML(getLineContent(diffLine.Content[1:]))
		}
		diff1 = compareDiffLine.Content
		diff2 = diffLine.Content
	case DiffLineDel:
		compareDiffLine = diffSection.GetLine(DiffLineAdd, diffLine.LeftIdx)
		if compareDiffLine == nil {
			return template.HTML(getLineContent(diffLine.Content[1:]))
		}
		diff1 = diffLine.Content
		diff2 = compareDiffLine.Content
	default:
		if strings.IndexByte(" +-", diffLine.Content[0]) > -1 {
			return template.HTML(getLineContent(diffLine.Content[1:]))
		}
		return template.HTML(getLineContent(diffLine.Content))
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

// GetTailSection creates a fake DiffLineSection if the last section is not the end of the file
func (diffFile *DiffFile) GetTailSection(gitRepo *git.Repository, leftCommitID, rightCommitID string) *DiffSection {
	if len(diffFile.Sections) == 0 || diffFile.Type != DiffFileChange || diffFile.IsBin || diffFile.IsLFSFile {
		return nil
	}
	leftCommit, err := gitRepo.GetCommit(leftCommitID)
	if err != nil {
		return nil
	}
	rightCommit, err := gitRepo.GetCommit(rightCommitID)
	if err != nil {
		return nil
	}
	lastSection := diffFile.Sections[len(diffFile.Sections)-1]
	lastLine := lastSection.Lines[len(lastSection.Lines)-1]
	leftLineCount := getCommitFileLineCount(leftCommit, diffFile.Name)
	rightLineCount := getCommitFileLineCount(rightCommit, diffFile.Name)
	if leftLineCount <= lastLine.LeftIdx || rightLineCount <= lastLine.RightIdx {
		return nil
	}
	tailDiffLine := &DiffLine{
		Type:    DiffLineSection,
		Content: " ",
		SectionInfo: &DiffLineSectionInfo{
			Path:         diffFile.Name,
			LastLeftIdx:  lastLine.LeftIdx,
			LastRightIdx: lastLine.RightIdx,
			LeftIdx:      leftLineCount,
			RightIdx:     rightLineCount,
		}}
	tailSection := &DiffSection{Lines: []*DiffLine{tailDiffLine}}
	return tailSection

}

func getCommitFileLineCount(commit *git.Commit, filePath string) int {
	blob, err := commit.GetBlobByPath(filePath)
	if err != nil {
		return 0
	}
	lineCount, err := blob.GetBlobLineCount()
	if err != nil {
		return 0
	}
	return lineCount
}

// Diff represents a difference between two git trees.
type Diff struct {
	TotalAddition, TotalDeletion int
	Files                        []*DiffFile
	IsIncomplete                 bool
}

// LoadComments loads comments into each line
func (diff *Diff) LoadComments(issue *models.Issue, currentUser *models.User) error {
	allComments, err := models.FetchCodeComments(issue, currentUser)
	if err != nil {
		return err
	}
	for _, file := range diff.Files {
		if lineCommits, ok := allComments[file.Name]; ok {
			for _, section := range file.Sections {
				for _, line := range section.Lines {
					if comments, ok := lineCommits[int64(line.LeftIdx*-1)]; ok {
						line.Comments = append(line.Comments, comments...)
					}
					if comments, ok := lineCommits[int64(line.RightIdx)]; ok {
						line.Comments = append(line.Comments, comments...)
					}
					sort.SliceStable(line.Comments, func(i, j int) bool {
						return line.Comments[i].CreatedUnix < line.Comments[j].CreatedUnix
					})
				}
			}
		}
	}
	return nil
}

// NumFiles returns number of files changes in a diff.
func (diff *Diff) NumFiles() int {
	return len(diff.Files)
}

// Example: @@ -1,8 +1,9 @@ => [..., 1, 8, 1, 9]
var hunkRegex = regexp.MustCompile(`^@@ -(?P<beginOld>[0-9]+)(,(?P<endOld>[0-9]+))? \+(?P<beginNew>[0-9]+)(,(?P<endNew>[0-9]+))? @@`)

func isHeader(lof string) bool {
	return strings.HasPrefix(lof, cmdDiffHead) || strings.HasPrefix(lof, "---") || strings.HasPrefix(lof, "+++")
}

// CutDiffAroundLine cuts a diff of a file in way that only the given line + numberOfLine above it will be shown
// it also recalculates hunks and adds the appropriate headers to the new diff.
// Warning: Only one-file diffs are allowed.
func CutDiffAroundLine(originalDiff io.Reader, line int64, old bool, numbersOfLine int) string {
	if line == 0 || numbersOfLine == 0 {
		// no line or num of lines => no diff
		return ""
	}
	scanner := bufio.NewScanner(originalDiff)
	hunk := make([]string, 0)
	// begin is the start of the hunk containing searched line
	// end is the end of the hunk ...
	// currentLine is the line number on the side of the searched line (differentiated by old)
	// otherLine is the line number on the opposite side of the searched line (differentiated by old)
	var begin, end, currentLine, otherLine int64
	var headerLines int
	for scanner.Scan() {
		lof := scanner.Text()
		// Add header to enable parsing
		if isHeader(lof) {
			hunk = append(hunk, lof)
			headerLines++
		}
		if currentLine > line {
			break
		}
		// Detect "hunk" with contains commented lof
		if strings.HasPrefix(lof, "@@") {
			// Already got our hunk. End of hunk detected!
			if len(hunk) > headerLines {
				break
			}
			// A map with named groups of our regex to recognize them later more easily
			submatches := hunkRegex.FindStringSubmatch(lof)
			groups := make(map[string]string)
			for i, name := range hunkRegex.SubexpNames() {
				if i != 0 && name != "" {
					groups[name] = submatches[i]
				}
			}
			if old {
				begin = com.StrTo(groups["beginOld"]).MustInt64()
				end = com.StrTo(groups["endOld"]).MustInt64()
				// init otherLine with begin of opposite side
				otherLine = com.StrTo(groups["beginNew"]).MustInt64()
			} else {
				begin = com.StrTo(groups["beginNew"]).MustInt64()
				if groups["endNew"] != "" {
					end = com.StrTo(groups["endNew"]).MustInt64()
				} else {
					end = 0
				}
				// init otherLine with begin of opposite side
				otherLine = com.StrTo(groups["beginOld"]).MustInt64()
			}
			end += begin // end is for real only the number of lines in hunk
			// lof is between begin and end
			if begin <= line && end >= line {
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
			default:
				currentLine++
				otherLine++
			}
		}
	}

	// No hunk found
	if currentLine == 0 {
		return ""
	}
	// headerLines + hunkLine (1) = totalNonCodeLines
	if len(hunk)-headerLines-1 <= numbersOfLine {
		// No need to cut the hunk => return existing hunk
		return strings.Join(hunk, "\n")
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
	newHunk := make([]string, headerLines)
	// transfer existing headers
	copy(newHunk, hunk[:headerLines])
	// transfer last n lines
	newHunk = append(newHunk, hunk[len(hunk)-numbersOfLine-1:]...)
	// calculate newBegin, ... by counting lines
	for i := len(hunk) - 1; i >= len(hunk)-numbersOfLine; i-- {
		switch hunk[i][0] {
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
	}
	// construct the new hunk header
	newHunk[headerLines] = fmt.Sprintf("@@ -%d,%d +%d,%d @@",
		oldBegin, oldNumOfLines, newBegin, newNumOfLines)
	return strings.Join(newHunk, "\n")
}

const cmdDiffHead = "diff --git "

// ParsePatch builds a Diff object from a io.Reader and some
// parameters.
// TODO: move this function to gogits/git-module
func ParsePatch(maxLines, maxLineCharacters, maxFiles int, reader io.Reader) (*Diff, error) {
	var (
		diff = &Diff{Files: make([]*DiffFile, 0)}

		curFile    = &DiffFile{}
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
		var linebuf bytes.Buffer
		for {
			b, err := input.ReadByte()
			if err != nil {
				if err == io.EOF {
					isEOF = true
					break
				} else {
					return nil, fmt.Errorf("ReadByte: %v", err)
				}
			}
			if b == '\n' {
				break
			}
			if linebuf.Len() < maxLineCharacters {
				linebuf.WriteByte(b)
			} else if linebuf.Len() == maxLineCharacters {
				curFile.IsIncomplete = true
			}
		}
		line := linebuf.String()

		if strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- ") || len(line) == 0 {
			continue
		}

		trimLine := strings.Trim(line, "+- ")

		if trimLine == models.LFSMetaFileIdentifier {
			curFileLFSPrefix = true
		}

		if curFileLFSPrefix && strings.HasPrefix(trimLine, models.LFSMetaFileOidPrefix) {
			oid := strings.TrimPrefix(trimLine, models.LFSMetaFileOidPrefix)

			if len(oid) == 64 {
				m := &models.LFSMetaObject{Oid: oid}
				count, err := models.Count(m)

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
		if curFileLinesCount >= maxLines {
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
			lineSectionInfo := getDiffLineSectionInfo(curFile, line, leftLine-1, rightLine-1)
			diffLine := &DiffLine{
				Type:        DiffLineSection,
				Content:     line,
				SectionInfo: lineSectionInfo,
			}
			curSection.Lines = append(curSection.Lines, diffLine)
			// update line number.
			leftLine = lineSectionInfo.LeftIdx
			rightLine = lineSectionInfo.RightIdx
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
			if len(diff.Files) >= maxFiles {
				diff.IsIncomplete = true
				_, err := io.Copy(ioutil.Discard, reader)
				if err != nil {
					return nil, fmt.Errorf("Copy: %v", err)
				}
				break
			}

			var middle int

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
				// Keep the entire string in double quotes for now
				a = line[beg:middle]
				b = line[middle+1:]

				var err error
				a, err = strconv.Unquote(a)
				if err != nil {
					return nil, fmt.Errorf("Unquote: %v", err)
				}
				b, err = strconv.Unquote(b)
				if err != nil {
					return nil, fmt.Errorf("Unquote: %v", err)
				}
				// Now remove the /a /b
				a = a[2:]
				b = b[2:]

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
			curFileLinesCount = 0
			leftLine = 1
			rightLine = 1
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
		charsetLabel, err := charset.DetectEncoding(buf.Bytes())
		if charsetLabel != "UTF-8" && err == nil {
			encoding, _ := stdcharset.Lookup(charsetLabel)
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
	return GetDiffRangeWithWhitespaceBehavior(repoPath, beforeCommitID, afterCommitID, maxLines, maxLineCharacters, maxFiles, "")
}

// GetDiffRangeWithWhitespaceBehavior builds a Diff between two commits of a repository.
// Passing the empty string as beforeCommitID returns a diff from the parent commit.
// The whitespaceBehavior is either an empty string or a git flag
func GetDiffRangeWithWhitespaceBehavior(repoPath, beforeCommitID, afterCommitID string, maxLines, maxLineCharacters, maxFiles int, whitespaceBehavior string) (*Diff, error) {
	gitRepo, err := git.OpenRepository(repoPath)
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	commit, err := gitRepo.GetCommit(afterCommitID)
	if err != nil {
		return nil, err
	}

	// FIXME: graceful: These commands should likely have a timeout
	ctx, cancel := context.WithCancel(git.DefaultContext)
	defer cancel()
	var cmd *exec.Cmd
	if len(beforeCommitID) == 0 && commit.ParentCount() == 0 {
		cmd = exec.CommandContext(ctx, git.GitExecutable, "show", afterCommitID)
	} else {
		actualBeforeCommitID := beforeCommitID
		if len(actualBeforeCommitID) == 0 {
			parentCommit, _ := commit.Parent(0)
			actualBeforeCommitID = parentCommit.ID.String()
		}
		diffArgs := []string{"diff", "-M"}
		if len(whitespaceBehavior) != 0 {
			diffArgs = append(diffArgs, whitespaceBehavior)
		}
		diffArgs = append(diffArgs, actualBeforeCommitID)
		diffArgs = append(diffArgs, afterCommitID)
		cmd = exec.CommandContext(ctx, git.GitExecutable, diffArgs...)
		beforeCommitID = actualBeforeCommitID
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

	pid := process.GetManager().Add(fmt.Sprintf("GetDiffRange [repo_path: %s]", repoPath), cancel)
	defer process.GetManager().Remove(pid)

	diff, err := ParsePatch(maxLines, maxLineCharacters, maxFiles, stdout)
	if err != nil {
		return nil, fmt.Errorf("ParsePatch: %v", err)
	}
	for _, diffFile := range diff.Files {
		tailSection := diffFile.GetTailSection(gitRepo, beforeCommitID, afterCommitID)
		if tailSection != nil {
			diffFile.Sections = append(diffFile.Sections, tailSection)
		}
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
	return GetRawDiffForFile(repoPath, "", commitID, diffType, "", writer)
}

// GetRawDiffForFile dumps diff results of file in given commit ID to io.Writer.
// TODO: move this function to gogits/git-module
func GetRawDiffForFile(repoPath, startCommit, endCommit string, diffType RawDiffType, file string, writer io.Writer) error {
	repo, err := git.OpenRepository(repoPath)
	if err != nil {
		return fmt.Errorf("OpenRepository: %v", err)
	}
	defer repo.Close()

	commit, err := repo.GetCommit(endCommit)
	if err != nil {
		return fmt.Errorf("GetCommit: %v", err)
	}
	fileArgs := make([]string, 0)
	if len(file) > 0 {
		fileArgs = append(fileArgs, "--", file)
	}
	// FIXME: graceful: These commands should have a timeout
	ctx, cancel := context.WithCancel(git.DefaultContext)
	defer cancel()

	var cmd *exec.Cmd
	switch diffType {
	case RawDiffNormal:
		if len(startCommit) != 0 {
			cmd = exec.CommandContext(ctx, git.GitExecutable, append([]string{"diff", "-M", startCommit, endCommit}, fileArgs...)...)
		} else if commit.ParentCount() == 0 {
			cmd = exec.CommandContext(ctx, git.GitExecutable, append([]string{"show", endCommit}, fileArgs...)...)
		} else {
			c, _ := commit.Parent(0)
			cmd = exec.CommandContext(ctx, git.GitExecutable, append([]string{"diff", "-M", c.ID.String(), endCommit}, fileArgs...)...)
		}
	case RawDiffPatch:
		if len(startCommit) != 0 {
			query := fmt.Sprintf("%s...%s", endCommit, startCommit)
			cmd = exec.CommandContext(ctx, git.GitExecutable, append([]string{"format-patch", "--no-signature", "--stdout", "--root", query}, fileArgs...)...)
		} else if commit.ParentCount() == 0 {
			cmd = exec.CommandContext(ctx, git.GitExecutable, append([]string{"format-patch", "--no-signature", "--stdout", "--root", endCommit}, fileArgs...)...)
		} else {
			c, _ := commit.Parent(0)
			query := fmt.Sprintf("%s...%s", endCommit, c.ID.String())
			cmd = exec.CommandContext(ctx, git.GitExecutable, append([]string{"format-patch", "--no-signature", "--stdout", query}, fileArgs...)...)
		}
	default:
		return fmt.Errorf("invalid diffType: %s", diffType)
	}

	stderr := new(bytes.Buffer)

	cmd.Dir = repoPath
	cmd.Stdout = writer
	cmd.Stderr = stderr
	pid := process.GetManager().Add(fmt.Sprintf("GetRawDiffForFile: [repo_path: %s]", repoPath), cancel)
	defer process.GetManager().Remove(pid)

	if err = cmd.Run(); err != nil {
		return fmt.Errorf("Run: %v - %s", err, stderr)
	}
	return nil
}

// GetDiffCommit builds a Diff representing the given commitID.
func GetDiffCommit(repoPath, commitID string, maxLines, maxLineCharacters, maxFiles int) (*Diff, error) {
	return GetDiffRange(repoPath, "", commitID, maxLines, maxLineCharacters, maxFiles)
}

// CommentAsDiff returns c.Patch as *Diff
func CommentAsDiff(c *models.Comment) (*Diff, error) {
	diff, err := ParsePatch(setting.Git.MaxGitDiffLines,
		setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(c.Patch))
	if err != nil {
		return nil, err
	}
	if len(diff.Files) == 0 {
		return nil, fmt.Errorf("no file found for comment ID: %d", c.ID)
	}
	secs := diff.Files[0].Sections
	if len(secs) == 0 {
		return nil, fmt.Errorf("no sections found for comment ID: %d", c.ID)
	}
	return diff, nil
}

// CommentMustAsDiff executes AsDiff and logs the error instead of returning
func CommentMustAsDiff(c *models.Comment) *Diff {
	diff, err := CommentAsDiff(c)
	if err != nil {
		log.Warn("CommentMustAsDiff: %v", err)
	}
	return diff
}
