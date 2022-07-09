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
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	pull_model "code.gitea.io/gitea/models/pull"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/analyze"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/sergi/go-diff/diffmatchpatch"
	stdcharset "golang.org/x/net/html/charset"
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
	Match       int
	Type        DiffLineType
	Content     string
	Comments    []*issues_model.Comment
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

// BlobExcerptChunkSize represent max lines of excerpt
const BlobExcerptChunkSize = 20

// GetType returns the type of DiffLine.
func (d *DiffLine) GetType() int {
	return int(d.Type)
}

// CanComment returns whether a line can get commented
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
	} else if d.SectionInfo.RightIdx-d.SectionInfo.LastRightIdx > BlobExcerptChunkSize && d.SectionInfo.RightHunkSize > 0 {
		return DiffLineExpandUpDown
	} else if d.SectionInfo.LeftHunkSize <= 0 && d.SectionInfo.RightHunkSize <= 0 {
		return DiffLineExpandDown
	}
	return DiffLineExpandSingle
}

func getDiffLineSectionInfo(treePath, line string, lastLeftIdx, lastRightIdx int) *DiffLineSectionInfo {
	leftLine, leftHunk, rightLine, righHunk := git.ParseDiffHunkString(line)

	return &DiffLineSectionInfo{
		Path:          treePath,
		LastLeftIdx:   lastLeftIdx,
		LastRightIdx:  lastRightIdx,
		LeftIdx:       leftLine,
		RightIdx:      rightLine,
		LeftHunkSize:  leftHunk,
		RightHunkSize: righHunk,
	}
}

// escape a line's content or return <br> needed for copy/paste purposes
func getLineContent(content string) DiffInline {
	if len(content) > 0 {
		return DiffInlineWithUnicodeEscape(template.HTML(html.EscapeString(content)))
	}
	return DiffInline{Content: "<br>"}
}

// DiffSection represents a section of a DiffFile.
type DiffSection struct {
	file     *DiffFile
	FileName string
	Name     string
	Lines    []*DiffLine
}

var (
	addedCodePrefix   = []byte(`<span class="added-code">`)
	removedCodePrefix = []byte(`<span class="removed-code">`)
	codeTagSuffix     = []byte(`</span>`)
)

func diffToHTML(lineWrapperTags []string, diffs []diffmatchpatch.Diff, lineType DiffLineType) string {
	buf := bytes.NewBuffer(nil)
	// restore the line wrapper tags <span class="line"> and <span class="cl">, if necessary
	for _, tag := range lineWrapperTags {
		buf.WriteString(tag)
	}
	for _, diff := range diffs {
		switch {
		case diff.Type == diffmatchpatch.DiffEqual:
			buf.WriteString(diff.Text)
		case diff.Type == diffmatchpatch.DiffInsert && lineType == DiffLineAdd:
			buf.Write(addedCodePrefix)
			buf.WriteString(diff.Text)
			buf.Write(codeTagSuffix)
		case diff.Type == diffmatchpatch.DiffDelete && lineType == DiffLineDel:
			buf.Write(removedCodePrefix)
			buf.WriteString(diff.Text)
			buf.Write(codeTagSuffix)
		}
	}
	for range lineWrapperTags {
		buf.WriteString("</span>")
	}
	return buf.String()
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

// DiffInline is a struct that has a content and escape status
type DiffInline struct {
	EscapeStatus charset.EscapeStatus
	Content      template.HTML
}

// DiffInlineWithUnicodeEscape makes a DiffInline with hidden unicode characters escaped
func DiffInlineWithUnicodeEscape(s template.HTML) DiffInline {
	status, content := charset.EscapeControlString(string(s))
	return DiffInline{EscapeStatus: status, Content: template.HTML(content)}
}

// DiffInlineWithHighlightCode makes a DiffInline with code highlight and hidden unicode characters escaped
func DiffInlineWithHighlightCode(fileName, language, code string) DiffInline {
	status, content := charset.EscapeControlString(highlight.Code(fileName, language, code))
	return DiffInline{EscapeStatus: status, Content: template.HTML(content)}
}

// highlightCodeDiff is used to do diff with highlighted HTML code.
// The HTML tags will be replaced by Unicode placeholders: "<span>{TEXT}</span>" => "\uE000{TEXT}\uE001"
// These Unicode placeholders are friendly to the diff.
// Then after diff, the placeholders in diff result will be recovered to the HTML tags.
// It's guaranteed that the tags in final diff result are paired correctly.
type highlightCodeDiff struct {
	placeholderBegin    rune
	placeholderMaxCount int
	placeholderIndex    int
	placeholderTagMap   map[rune]string
	tagPlaceholderMap   map[string]rune

	placeholderOverflowCount int

	lineWrapperTags []string
}

func newHighlightCodeDiff() *highlightCodeDiff {
	return &highlightCodeDiff{
		placeholderBegin:    rune(0xE000), // Private Use Unicode: U+E000..U+F8FF, BMP(0), 6400
		placeholderMaxCount: 6400,
		placeholderTagMap:   map[rune]string{},
		tagPlaceholderMap:   map[string]rune{},
	}
}

// nextPlaceholder returns 0 if no more placeholder can be used
// the diff is done line by line, usually there are only a few (no more than 10) placeholders in one line
// so the placeholderMaxCount is impossible to be exhausted in real cases.
func (hcd *highlightCodeDiff) nextPlaceholder() rune {
	for hcd.placeholderIndex < hcd.placeholderMaxCount {
		r := hcd.placeholderBegin + rune(hcd.placeholderIndex)
		hcd.placeholderIndex++
		// only use non-existing (not used by code) rune as placeholders
		if _, ok := hcd.placeholderTagMap[r]; !ok {
			return r
		}
	}
	return 0 // no more available placeholder
}

func (hcd *highlightCodeDiff) isInPlaceholderRange(r rune) bool {
	return hcd.placeholderBegin <= r && r < hcd.placeholderBegin+rune(hcd.placeholderMaxCount)
}

func (hcd *highlightCodeDiff) collectUsedRunes(code string) {
	for _, r := range code {
		if hcd.isInPlaceholderRange(r) {
			// put the existing rune (used by code) in map, then this rune won't be used a placeholder anymore.
			hcd.placeholderTagMap[r] = ""
		}
	}
}

func (hcd *highlightCodeDiff) diffWithHighlight(filename, language, codeA, codeB string) []diffmatchpatch.Diff {
	hcd.collectUsedRunes(codeA)
	hcd.collectUsedRunes(codeB)

	highlightCodeA := highlight.Code(filename, language, codeA)
	highlightCodeB := highlight.Code(filename, language, codeB)

	highlightCodeA = hcd.convertToPlaceholders(highlightCodeA)
	highlightCodeB = hcd.convertToPlaceholders(highlightCodeB)

	diffs := diffMatchPatch.DiffMain(highlightCodeA, highlightCodeB, true)
	diffs = diffMatchPatch.DiffCleanupEfficiency(diffs)

	for i := range diffs {
		hcd.recoverOneDiff(&diffs[i])
	}
	return diffs
}

func (hcd *highlightCodeDiff) convertToPlaceholders(htmlCode string) string {
	var tagStack []string
	res := strings.Builder{}

	firstRunForLineTags := hcd.lineWrapperTags == nil

	// the standard chroma highlight HTML is "<span class="line [hl]"><span class="cl"> ... </span></span>"
	for {
		// find the next HTML tag
		pos1 := strings.IndexByte(htmlCode, '<')
		pos2 := strings.IndexByte(htmlCode, '>')
		if pos1 == -1 || pos2 == -1 || pos2 < pos1 {
			break
		}
		tag := htmlCode[pos1 : pos2+1]

		// write the content before the tag into result string, and consume the tag in the string
		res.WriteString(htmlCode[:pos1])
		htmlCode = htmlCode[pos2+1:]

		// the line wrapper tags should be removed before diff
		if strings.HasPrefix(tag, `<span class="line`) || strings.HasPrefix(tag, `<span class="cl"`) {
			if firstRunForLineTags {
				// if this is the first run for converting, save the line wrapper tags for later use, they should be added back
				hcd.lineWrapperTags = append(hcd.lineWrapperTags, tag)
			}
			htmlCode = strings.TrimSuffix(htmlCode, "</span>")
			continue
		}

		var tagInMap string
		if tag[1] == '/' { // for closed tag
			if len(tagStack) == 0 {
				break // invalid diff result, no open tag but see close tag
			}
			// make sure the closed tag in map is related to the open tag, to make the diff algorithm can match the open/closed tags
			// the closed tag will be recorded in the map by key "</span><!-- <span the-open> -->" for "<span the-open>"
			tagInMap = tag + "<!-- " + tagStack[len(tagStack)-1] + "-->"
			tagStack = tagStack[:len(tagStack)-1]
		} else { // for open tag
			tagInMap = tag
			tagStack = append(tagStack, tag)
		}

		// remember the placeholder and tag in the map
		placeholder, ok := hcd.tagPlaceholderMap[tagInMap]
		if !ok {
			placeholder = hcd.nextPlaceholder()
			if placeholder != 0 {
				hcd.tagPlaceholderMap[tagInMap] = placeholder
				hcd.placeholderTagMap[placeholder] = tagInMap
			}
		}

		if placeholder != 0 {
			res.WriteRune(placeholder) // use the placeholder to replace the tag
		} else {
			// unfortunately, all private use runes has been exhausted, no more placeholder could be used, no more converting
			// usually, the exhausting won't occur in real cases, the magnitude of used placeholders is not larger than that of the CSS classes outputted by chroma.
			hcd.placeholderOverflowCount++
		}
	}
	// write the remaining string
	res.WriteString(htmlCode)
	return res.String()
}

func (hcd *highlightCodeDiff) recoverOneDiff(diff *diffmatchpatch.Diff) {
	sb := strings.Builder{}
	var tagStack []string

	for _, r := range diff.Text {
		tag, ok := hcd.placeholderTagMap[r]
		if !ok || tag == "" {
			sb.WriteRune(r) // if the rune is not a placeholder, write it as it is
			continue
		}
		var tagToRecover string
		if tag[1] == '/' { // Closing tag
			// only get the tag itself, ignore the trailing comment (for how the comment is generated, see the code in `convert` function)
			tagToRecover = tag[:strings.IndexByte(tag, '>')+1]
			if len(tagStack) == 0 {
				continue // if no open tag in stack yet, skip the closed tag
			}
			tagStack = tagStack[:len(tagStack)-1]
		} else {
			tagToRecover = tag
			tagStack = append(tagStack, tag)
		}
		sb.WriteString(tagToRecover)
	}

	if len(tagStack) > 0 {
		// close all open tags
		for i := len(tagStack) - 1; i >= 0; i-- {
			tagToClose := tagStack[i]
			// get the closed tag "</span>" from "<span class=...>" or "<span>"
			pos := strings.IndexAny(tagToClose, " >")
			if pos != -1 {
				sb.WriteString("</" + tagToClose[1:pos] + ">")
			} // else: impossible. every tag was pushed into the stack by the code above and is valid HTML open tag
		}
	}

	diff.Text = sb.String()
}

// GetComputedInlineDiffFor computes inline diff for the given line.
func (diffSection *DiffSection) GetComputedInlineDiffFor(diffLine *DiffLine) DiffInline {
	if setting.Git.DisableDiffHighlight {
		return getLineContent(diffLine.Content[1:])
	}

	var (
		compareDiffLine *DiffLine
		diff1           string
		diff2           string
	)

	language := ""
	if diffSection.file != nil {
		language = diffSection.file.Language
	}

	// try to find equivalent diff line. ignore, otherwise
	switch diffLine.Type {
	case DiffLineSection:
		return getLineContent(diffLine.Content[1:])
	case DiffLineAdd:
		compareDiffLine = diffSection.GetLine(DiffLineDel, diffLine.RightIdx)
		if compareDiffLine == nil {
			return DiffInlineWithHighlightCode(diffSection.FileName, language, diffLine.Content[1:])
		}
		diff1 = compareDiffLine.Content
		diff2 = diffLine.Content
	case DiffLineDel:
		compareDiffLine = diffSection.GetLine(DiffLineAdd, diffLine.LeftIdx)
		if compareDiffLine == nil {
			return DiffInlineWithHighlightCode(diffSection.FileName, language, diffLine.Content[1:])
		}
		diff1 = diffLine.Content
		diff2 = compareDiffLine.Content
	default:
		if strings.IndexByte(" +-", diffLine.Content[0]) > -1 {
			return DiffInlineWithHighlightCode(diffSection.FileName, language, diffLine.Content[1:])
		}
		return DiffInlineWithHighlightCode(diffSection.FileName, language, diffLine.Content)
	}

	hcd := newHighlightCodeDiff()
	diffRecord := hcd.diffWithHighlight(diffSection.FileName, language, diff1[1:], diff2[1:])
	// it seems that Gitea doesn't need the line wrapper of Chroma, so do not add them back
	// if the line wrappers are still needed in the future, it can be added back by "diffToHTML(hcd.lineWrapperTags. ...)"
	diffHTML := diffToHTML(nil, diffRecord, diffLine.Type)
	return DiffInlineWithUnicodeEscape(template.HTML(diffHTML))
}

// DiffFile represents a file diff.
type DiffFile struct {
	Name                      string
	NameHash                  string
	OldName                   string
	Index                     int
	Addition, Deletion        int
	Type                      DiffFileType
	IsCreated                 bool
	IsDeleted                 bool
	IsBin                     bool
	IsLFSFile                 bool
	IsRenamed                 bool
	IsAmbiguous               bool
	IsSubmodule               bool
	Sections                  []*DiffSection
	IsIncomplete              bool
	IsIncompleteLineTooLong   bool
	IsProtected               bool
	IsGenerated               bool
	IsVendored                bool
	IsViewed                  bool // User specific
	HasChangedSinceLastReview bool // User specific
	Language                  string
}

// GetType returns type of diff file.
func (diffFile *DiffFile) GetType() int {
	return int(diffFile.Type)
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
		},
	}
	tailSection := &DiffSection{FileName: diffFile.Name, Lines: []*DiffLine{tailDiffLine}}
	return tailSection
}

// GetDiffFileName returns the name of the diff file, or its old name in case it was deleted
func (diffFile *DiffFile) GetDiffFileName() string {
	if diffFile.Name == "" {
		return diffFile.OldName
	}
	return diffFile.Name
}

func (diffFile *DiffFile) ShouldBeHidden() bool {
	return diffFile.IsGenerated || diffFile.IsViewed
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
	Start, End                   string
	NumFiles                     int
	TotalAddition, TotalDeletion int
	Files                        []*DiffFile
	IsIncomplete                 bool
	NumViewedFiles               int // user-specific
}

// LoadComments loads comments into each line
func (diff *Diff) LoadComments(ctx context.Context, issue *issues_model.Issue, currentUser *user_model.User) error {
	allComments, err := issues_model.FetchCodeComments(ctx, issue, currentUser)
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

const cmdDiffHead = "diff --git "

// ParsePatch builds a Diff object from a io.Reader and some parameters.
func ParsePatch(maxLines, maxLineCharacters, maxFiles int, reader io.Reader, skipToFile string) (*Diff, error) {
	log.Debug("ParsePatch(%d, %d, %d, ..., %s)", maxLines, maxLineCharacters, maxFiles, skipToFile)
	var curFile *DiffFile

	skipping := skipToFile != ""

	diff := &Diff{Files: make([]*DiffFile, 0)}

	sb := strings.Builder{}

	// OK let's set a reasonable buffer size.
	// This should be let's say at least the size of maxLineCharacters or 4096 whichever is larger.
	readerSize := maxLineCharacters
	if readerSize < 4096 {
		readerSize = 4096
	}

	input := bufio.NewReaderSize(reader, readerSize)
	line, err := input.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			return diff, nil
		}
		return diff, err
	}
parsingLoop:
	for {
		// 1. A patch file always begins with `diff --git ` + `a/path b/path` (possibly quoted)
		// if it does not we have bad input!
		if !strings.HasPrefix(line, cmdDiffHead) {
			return diff, fmt.Errorf("invalid first file line: %s", line)
		}

		if maxFiles > -1 && len(diff.Files) >= maxFiles {
			lastFile := createDiffFile(diff, line)
			diff.End = lastFile.Name
			diff.IsIncomplete = true
			_, err := io.Copy(io.Discard, reader)
			if err != nil {
				// By the definition of io.Copy this never returns io.EOF
				return diff, fmt.Errorf("error during io.Copy: %w", err)
			}
			break parsingLoop
		}

		curFile = createDiffFile(diff, line)
		if skipping {
			if curFile.Name != skipToFile {
				line, err = skipToNextDiffHead(input)
				if err != nil {
					if err == io.EOF {
						return diff, nil
					}
					return diff, err
				}
				continue
			}
			skipping = false
		}

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
	curFileLoop:
		for {
			line, err = input.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					return diff, err
				}
				break parsingLoop
			}
			switch {
			case strings.HasPrefix(line, cmdDiffHead):
				break curFileLoop
			case strings.HasPrefix(line, "old mode ") ||
				strings.HasPrefix(line, "new mode "):
				if strings.HasSuffix(line, " 160000\n") {
					curFile.IsSubmodule = true
				}
			case strings.HasPrefix(line, "rename from "):
				curFile.IsRenamed = true
				curFile.Type = DiffFileRename
				if curFile.IsAmbiguous {
					curFile.OldName = line[len("rename from ") : len(line)-1]
				}
			case strings.HasPrefix(line, "rename to "):
				curFile.IsRenamed = true
				curFile.Type = DiffFileRename
				if curFile.IsAmbiguous {
					curFile.Name = line[len("rename to ") : len(line)-1]
					curFile.IsAmbiguous = false
				}
			case strings.HasPrefix(line, "copy from "):
				curFile.IsRenamed = true
				curFile.Type = DiffFileCopy
				if curFile.IsAmbiguous {
					curFile.OldName = line[len("copy from ") : len(line)-1]
				}
			case strings.HasPrefix(line, "copy to "):
				curFile.IsRenamed = true
				curFile.Type = DiffFileCopy
				if curFile.IsAmbiguous {
					curFile.Name = line[len("copy to ") : len(line)-1]
					curFile.IsAmbiguous = false
				}
			case strings.HasPrefix(line, "new file"):
				curFile.Type = DiffFileAdd
				curFile.IsCreated = true
				if strings.HasSuffix(line, " 160000\n") {
					curFile.IsSubmodule = true
				}
			case strings.HasPrefix(line, "deleted"):
				curFile.Type = DiffFileDel
				curFile.IsDeleted = true
				if strings.HasSuffix(line, " 160000\n") {
					curFile.IsSubmodule = true
				}
			case strings.HasPrefix(line, "index"):
				if strings.HasSuffix(line, " 160000\n") {
					curFile.IsSubmodule = true
				}
			case strings.HasPrefix(line, "similarity index 100%"):
				curFile.Type = DiffFileRename
			case strings.HasPrefix(line, "Binary"):
				curFile.IsBin = true
			case strings.HasPrefix(line, "--- "):
				// Handle ambiguous filenames
				if curFile.IsAmbiguous {
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
				if curFile.IsAmbiguous {
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
					curFile.IsAmbiguous = false
				}
				// Otherwise do nothing with this line, but now switch to parsing hunks
				lineBytes, isFragment, err := parseHunks(curFile, maxLines, maxLineCharacters, input)
				diff.TotalAddition += curFile.Addition
				diff.TotalDeletion += curFile.Deletion
				if err != nil {
					if err != io.EOF {
						return diff, err
					}
					break parsingLoop
				}
				sb.Reset()
				_, _ = sb.Write(lineBytes)
				for isFragment {
					lineBytes, isFragment, err = input.ReadLine()
					if err != nil {
						// Now by the definition of ReadLine this cannot be io.EOF
						return diff, fmt.Errorf("unable to ReadLine: %w", err)
					}
					_, _ = sb.Write(lineBytes)
				}
				line = sb.String()
				sb.Reset()

				break curFileLoop
			}
		}
	}

	// TODO: There are numerous issues with this:
	// - we might want to consider detecting encoding while parsing but...
	// - we're likely to fail to get the correct encoding here anyway as we won't have enough information
	diffLineTypeBuffers := make(map[DiffLineType]*bytes.Buffer, 3)
	diffLineTypeDecoders := make(map[DiffLineType]*encoding.Decoder, 3)
	diffLineTypeBuffers[DiffLinePlain] = new(bytes.Buffer)
	diffLineTypeBuffers[DiffLineAdd] = new(bytes.Buffer)
	diffLineTypeBuffers[DiffLineDel] = new(bytes.Buffer)
	for _, f := range diff.Files {
		f.NameHash = base.EncodeSha1(f.Name)

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
			if buffer.Len() == 0 {
				continue
			}
			charsetLabel, err := charset.DetectEncoding(buffer.Bytes())
			if charsetLabel != "UTF-8" && err == nil {
				encoding, _ := stdcharset.Lookup(charsetLabel)
				if encoding != nil {
					diffLineTypeDecoders[lineType] = encoding.NewDecoder()
				}
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

	diff.NumFiles = len(diff.Files)
	return diff, nil
}

func skipToNextDiffHead(input *bufio.Reader) (line string, err error) {
	// need to skip until the next cmdDiffHead
	var isFragment, wasFragment bool
	var lineBytes []byte
	for {
		lineBytes, isFragment, err = input.ReadLine()
		if err != nil {
			return
		}
		if wasFragment {
			wasFragment = isFragment
			continue
		}
		if bytes.HasPrefix(lineBytes, []byte(cmdDiffHead)) {
			break
		}
		wasFragment = isFragment
	}
	line = string(lineBytes)
	if isFragment {
		var tail string
		tail, err = input.ReadString('\n')
		if err != nil {
			return
		}
		line += tail
	}
	return line, err
}

func parseHunks(curFile *DiffFile, maxLines, maxLineCharacters int, input *bufio.Reader) (lineBytes []byte, isFragment bool, err error) {
	sb := strings.Builder{}

	var (
		curSection        *DiffSection
		curFileLinesCount int
		curFileLFSPrefix  bool
	)

	lastLeftIdx := -1
	leftLine, rightLine := 1, 1

	for {
		for isFragment {
			curFile.IsIncomplete = true
			curFile.IsIncompleteLineTooLong = true
			_, isFragment, err = input.ReadLine()
			if err != nil {
				// Now by the definition of ReadLine this cannot be io.EOF
				err = fmt.Errorf("unable to ReadLine: %w", err)
				return
			}
		}
		sb.Reset()
		lineBytes, isFragment, err = input.ReadLine()
		if err != nil {
			if err == io.EOF {
				return
			}
			err = fmt.Errorf("unable to ReadLine: %w", err)
			return
		}
		if lineBytes[0] == 'd' {
			// End of hunks
			return
		}

		switch lineBytes[0] {
		case '@':
			if maxLines > -1 && curFileLinesCount >= maxLines {
				curFile.IsIncomplete = true
				continue
			}

			_, _ = sb.Write(lineBytes)
			for isFragment {
				// This is very odd indeed - we're in a section header and the line is too long
				// This really shouldn't happen...
				lineBytes, isFragment, err = input.ReadLine()
				if err != nil {
					// Now by the definition of ReadLine this cannot be io.EOF
					err = fmt.Errorf("unable to ReadLine: %w", err)
					return
				}
				_, _ = sb.Write(lineBytes)
			}
			line := sb.String()

			// Create a new section to represent this hunk
			curSection = &DiffSection{file: curFile}
			lastLeftIdx = -1
			curFile.Sections = append(curFile.Sections, curSection)

			lineSectionInfo := getDiffLineSectionInfo(curFile.Name, line, leftLine-1, rightLine-1)
			diffLine := &DiffLine{
				Type:        DiffLineSection,
				Content:     line,
				SectionInfo: lineSectionInfo,
			}
			curSection.Lines = append(curSection.Lines, diffLine)
			curSection.FileName = curFile.Name
			// update line number.
			leftLine = lineSectionInfo.LeftIdx
			rightLine = lineSectionInfo.RightIdx
			continue
		case '\\':
			if maxLines > -1 && curFileLinesCount >= maxLines {
				curFile.IsIncomplete = true
				continue
			}
			// This is used only to indicate that the current file does not have a terminal newline
			if !bytes.Equal(lineBytes, []byte("\\ No newline at end of file")) {
				err = fmt.Errorf("unexpected line in hunk: %s", string(lineBytes))
				return
			}
			// Technically this should be the end the file!
			// FIXME: we should be putting a marker at the end of the file if there is no terminal new line
			continue
		case '+':
			curFileLinesCount++
			curFile.Addition++
			if maxLines > -1 && curFileLinesCount >= maxLines {
				curFile.IsIncomplete = true
				continue
			}
			diffLine := &DiffLine{Type: DiffLineAdd, RightIdx: rightLine, Match: -1}
			rightLine++
			if curSection == nil {
				// Create a new section to represent this hunk
				curSection = &DiffSection{file: curFile}
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
			curFileLinesCount++
			curFile.Deletion++
			if maxLines > -1 && curFileLinesCount >= maxLines {
				curFile.IsIncomplete = true
				continue
			}
			diffLine := &DiffLine{Type: DiffLineDel, LeftIdx: leftLine, Match: -1}
			if leftLine > 0 {
				leftLine++
			}
			if curSection == nil {
				// Create a new section to represent this hunk
				curSection = &DiffSection{file: curFile}
				curFile.Sections = append(curFile.Sections, curSection)
				lastLeftIdx = -1
			}
			if len(curSection.Lines) == 0 || curSection.Lines[len(curSection.Lines)-1].Type != DiffLineDel {
				lastLeftIdx = len(curSection.Lines)
			}
			curSection.Lines = append(curSection.Lines, diffLine)
		case ' ':
			curFileLinesCount++
			if maxLines > -1 && curFileLinesCount >= maxLines {
				curFile.IsIncomplete = true
				continue
			}
			diffLine := &DiffLine{Type: DiffLinePlain, LeftIdx: leftLine, RightIdx: rightLine}
			leftLine++
			rightLine++
			lastLeftIdx = -1
			if curSection == nil {
				// Create a new section to represent this hunk
				curSection = &DiffSection{file: curFile}
				curFile.Sections = append(curFile.Sections, curSection)
			}
			curSection.Lines = append(curSection.Lines, diffLine)
		default:
			// This is unexpected
			err = fmt.Errorf("unexpected line in hunk: %s", string(lineBytes))
			return
		}

		line := string(lineBytes)
		if isFragment {
			curFile.IsIncomplete = true
			curFile.IsIncompleteLineTooLong = true
			for isFragment {
				lineBytes, isFragment, err = input.ReadLine()
				if err != nil {
					// Now by the definition of ReadLine this cannot be io.EOF
					err = fmt.Errorf("unable to ReadLine: %w", err)
					return
				}
			}
		}
		if len(line) > maxLineCharacters {
			curFile.IsIncomplete = true
			curFile.IsIncompleteLineTooLong = true
			line = line[:maxLineCharacters]
		}
		curSection.Lines[len(curSection.Lines)-1].Content = line

		// handle LFS
		if line[1:] == lfs.MetaFileIdentifier {
			curFileLFSPrefix = true
		} else if curFileLFSPrefix && strings.HasPrefix(line[1:], lfs.MetaFileOidPrefix) {
			oid := strings.TrimPrefix(line[1:], lfs.MetaFileOidPrefix)
			if len(oid) == 64 {
				m := &git_model.LFSMetaObject{Pointer: lfs.Pointer{Oid: oid}}
				count, err := db.CountByBean(db.DefaultContext, m)

				if err == nil && count > 0 {
					curFile.IsBin = true
					curFile.IsLFSFile = true
					curSection.Lines = nil
					lastLeftIdx = -1
				}
			}
		}
	}
}

func createDiffFile(diff *Diff, line string) *DiffFile {
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
	// This means that you should always be able to determine the file name even when there
	// there is potential ambiguity...
	//
	// but we can be simpler with our heuristics by just forcing git to prefix things nicely
	curFile := &DiffFile{
		Index:    len(diff.Files) + 1,
		Type:     DiffFileChange,
		Sections: make([]*DiffSection, 0, 10),
	}

	rd := strings.NewReader(line[len(cmdDiffHead):] + " ")
	curFile.Type = DiffFileChange
	var oldNameAmbiguity, newNameAmbiguity bool

	curFile.OldName, oldNameAmbiguity = readFileName(rd)
	curFile.Name, newNameAmbiguity = readFileName(rd)
	if oldNameAmbiguity && newNameAmbiguity {
		curFile.IsAmbiguous = true
		// OK we should bet that the oldName and the newName are the same if they can be made to be same
		// So we need to start again ...
		if (len(line)-len(cmdDiffHead)-1)%2 == 0 {
			// diff --git a/b b/b b/b b/b b/b b/b
			//
			midpoint := (len(line) + len(cmdDiffHead) - 1) / 2
			new, old := line[len(cmdDiffHead):midpoint], line[midpoint+1:]
			if len(new) > 2 && len(old) > 2 && new[2:] == old[2:] {
				curFile.OldName = old[2:]
				curFile.Name = old[2:]
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
		fmt.Fscanf(rd, "%q ", &name)
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
		fmt.Fscanf(rd, "%s ", &name)
		char, _ := rd.ReadByte()
		_ = rd.UnreadByte()
		for !(char == 0 || char == '"' || char == 'b') {
			var suffix string
			fmt.Fscanf(rd, "%s ", &suffix)
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

// DiffOptions represents the options for a DiffRange
type DiffOptions struct {
	BeforeCommitID     string
	AfterCommitID      string
	SkipTo             string
	MaxLines           int
	MaxLineCharacters  int
	MaxFiles           int
	WhitespaceBehavior string
	DirectComparison   bool
}

// GetDiff builds a Diff between two commits of a repository.
// Passing the empty string as beforeCommitID returns a diff from the parent commit.
// The whitespaceBehavior is either an empty string or a git flag
func GetDiff(gitRepo *git.Repository, opts *DiffOptions, files ...string) (*Diff, error) {
	repoPath := gitRepo.Path

	commit, err := gitRepo.GetCommit(opts.AfterCommitID)
	if err != nil {
		return nil, err
	}

	argsLength := 6
	if len(opts.WhitespaceBehavior) > 0 {
		argsLength++
	}
	if len(opts.SkipTo) > 0 {
		argsLength++
	}
	if len(files) > 0 {
		argsLength += len(files) + 1
	}

	diffArgs := make([]string, 0, argsLength)
	if (len(opts.BeforeCommitID) == 0 || opts.BeforeCommitID == git.EmptySHA) && commit.ParentCount() == 0 {
		diffArgs = append(diffArgs, "diff", "--src-prefix=\\a/", "--dst-prefix=\\b/", "-M")
		if len(opts.WhitespaceBehavior) != 0 {
			diffArgs = append(diffArgs, opts.WhitespaceBehavior)
		}
		// append empty tree ref
		diffArgs = append(diffArgs, "4b825dc642cb6eb9a060e54bf8d69288fbee4904")
		diffArgs = append(diffArgs, opts.AfterCommitID)
	} else {
		actualBeforeCommitID := opts.BeforeCommitID
		if len(actualBeforeCommitID) == 0 {
			parentCommit, _ := commit.Parent(0)
			actualBeforeCommitID = parentCommit.ID.String()
		}
		diffArgs = append(diffArgs, "diff", "--src-prefix=\\a/", "--dst-prefix=\\b/", "-M")
		if len(opts.WhitespaceBehavior) != 0 {
			diffArgs = append(diffArgs, opts.WhitespaceBehavior)
		}
		diffArgs = append(diffArgs, actualBeforeCommitID)
		diffArgs = append(diffArgs, opts.AfterCommitID)
		opts.BeforeCommitID = actualBeforeCommitID
	}

	// In git 2.31, git diff learned --skip-to which we can use to shortcut skip to file
	// so if we are using at least this version of git we don't have to tell ParsePatch to do
	// the skipping for us
	parsePatchSkipToFile := opts.SkipTo
	if opts.SkipTo != "" && git.CheckGitVersionAtLeast("2.31") == nil {
		diffArgs = append(diffArgs, "--skip-to="+opts.SkipTo)
		parsePatchSkipToFile = ""
	}

	if len(files) > 0 {
		diffArgs = append(diffArgs, "--")
		diffArgs = append(diffArgs, files...)
	}

	reader, writer := io.Pipe()
	defer func() {
		_ = reader.Close()
		_ = writer.Close()
	}()

	go func(ctx context.Context, diffArgs []string, repoPath string, writer *io.PipeWriter) {
		cmd := git.NewCommand(ctx, diffArgs...)
		cmd.SetDescription(fmt.Sprintf("GetDiffRange [repo_path: %s]", repoPath))
		if err := cmd.Run(&git.RunOpts{
			Timeout: time.Duration(setting.Git.Timeout.Default) * time.Second,
			Dir:     repoPath,
			Stderr:  os.Stderr,
			Stdout:  writer,
		}); err != nil {
			log.Error("error during RunWithContext: %w", err)
		}

		_ = writer.Close()
	}(gitRepo.Ctx, diffArgs, repoPath, writer)

	diff, err := ParsePatch(opts.MaxLines, opts.MaxLineCharacters, opts.MaxFiles, reader, parsePatchSkipToFile)
	if err != nil {
		return nil, fmt.Errorf("unable to ParsePatch: %w", err)
	}
	diff.Start = opts.SkipTo

	checker, deferable := gitRepo.CheckAttributeReader(opts.AfterCommitID)
	defer deferable()

	for _, diffFile := range diff.Files {

		gotVendor := false
		gotGenerated := false
		if checker != nil {
			attrs, err := checker.CheckPath(diffFile.Name)
			if err == nil {
				if vendored, has := attrs["linguist-vendored"]; has {
					if vendored == "set" || vendored == "true" {
						diffFile.IsVendored = true
						gotVendor = true
					} else {
						gotVendor = vendored == "false"
					}
				}
				if generated, has := attrs["linguist-generated"]; has {
					if generated == "set" || generated == "true" {
						diffFile.IsGenerated = true
						gotGenerated = true
					} else {
						gotGenerated = generated == "false"
					}
				}
				if language, has := attrs["linguist-language"]; has && language != "unspecified" && language != "" {
					diffFile.Language = language
				} else if language, has := attrs["gitlab-language"]; has && language != "unspecified" && language != "" {
					diffFile.Language = language
				}
			} else {
				log.Error("Unexpected error: %v", err)
			}
		}

		if !gotVendor {
			diffFile.IsVendored = analyze.IsVendor(diffFile.Name)
		}
		if !gotGenerated {
			diffFile.IsGenerated = analyze.IsGenerated(diffFile.Name)
		}

		tailSection := diffFile.GetTailSection(gitRepo, opts.BeforeCommitID, opts.AfterCommitID)
		if tailSection != nil {
			diffFile.Sections = append(diffFile.Sections, tailSection)
		}
	}

	separator := "..."
	if opts.DirectComparison {
		separator = ".."
	}

	shortstatArgs := []string{opts.BeforeCommitID + separator + opts.AfterCommitID}
	if len(opts.BeforeCommitID) == 0 || opts.BeforeCommitID == git.EmptySHA {
		shortstatArgs = []string{git.EmptyTreeSHA, opts.AfterCommitID}
	}
	diff.NumFiles, diff.TotalAddition, diff.TotalDeletion, err = git.GetDiffShortStat(gitRepo.Ctx, repoPath, shortstatArgs...)
	if err != nil && strings.Contains(err.Error(), "no merge base") {
		// git >= 2.28 now returns an error if base and head have become unrelated.
		// previously it would return the results of git diff --shortstat base head so let's try that...
		shortstatArgs = []string{opts.BeforeCommitID, opts.AfterCommitID}
		diff.NumFiles, diff.TotalAddition, diff.TotalDeletion, err = git.GetDiffShortStat(gitRepo.Ctx, repoPath, shortstatArgs...)
	}
	if err != nil {
		return nil, err
	}

	return diff, nil
}

// SyncAndGetUserSpecificDiff is like GetDiff, except that user specific data such as which files the given user has already viewed on the given PR will also be set
// Additionally, the database asynchronously is updated if files have changed since the last review
func SyncAndGetUserSpecificDiff(ctx context.Context, userID int64, pull *issues_model.PullRequest, gitRepo *git.Repository, opts *DiffOptions, files ...string) (*Diff, error) {
	diff, err := GetDiff(gitRepo, opts, files...)
	if err != nil {
		return nil, err
	}
	review, err := pull_model.GetNewestReviewState(ctx, userID, pull.ID)
	if err != nil || review == nil || review.UpdatedFiles == nil {
		return diff, err
	}

	latestCommit := opts.AfterCommitID
	if latestCommit == "" {
		latestCommit = pull.HeadBranch // opts.AfterCommitID is preferred because it handles PRs from forks correctly and the branch name doesn't
	}

	changedFiles, err := gitRepo.GetFilesChangedBetween(review.CommitSHA, latestCommit)
	if err != nil {
		return diff, err
	}

	filesChangedSinceLastDiff := make(map[string]pull_model.ViewedState)
outer:
	for _, diffFile := range diff.Files {
		fileViewedState := review.UpdatedFiles[diffFile.GetDiffFileName()]

		// Check whether it was previously detected that the file has changed since the last review
		if fileViewedState == pull_model.HasChanged {
			diffFile.HasChangedSinceLastReview = true
			continue
		}

		filename := diffFile.GetDiffFileName()

		// Check explicitly whether the file has changed since the last review
		for _, changedFile := range changedFiles {
			diffFile.HasChangedSinceLastReview = filename == changedFile
			if diffFile.HasChangedSinceLastReview {
				filesChangedSinceLastDiff[filename] = pull_model.HasChanged
				continue outer // We don't want to check if the file is viewed here as that would fold the file, which is in this case unwanted
			}
		}
		// Check whether the file has already been viewed
		if fileViewedState == pull_model.Viewed {
			diffFile.IsViewed = true
			diff.NumViewedFiles++
		}
	}

	// Explicitly store files that have changed in the database, if any is present at all.
	// This has the benefit that the "Has Changed" attribute will be present as long as the user does not explicitly mark this file as viewed, so it will even survive a page reload after marking another file as viewed.
	// On the other hand, this means that even if a commit reverting an unseen change is committed, the file will still be seen as changed.
	if len(filesChangedSinceLastDiff) > 0 {
		err := pull_model.UpdateReviewState(ctx, review.UserID, review.PullID, review.CommitSHA, filesChangedSinceLastDiff)
		if err != nil {
			log.Warn("Could not update review for user %d, pull %d, commit %s and the changed files %v: %v", review.UserID, review.PullID, review.CommitSHA, filesChangedSinceLastDiff, err)
			return nil, err
		}
	}

	return diff, err
}

// CommentAsDiff returns c.Patch as *Diff
func CommentAsDiff(c *issues_model.Comment) (*Diff, error) {
	diff, err := ParsePatch(setting.Git.MaxGitDiffLines,
		setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(c.Patch), "")
	if err != nil {
		log.Error("Unable to parse patch: %v", err)
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
func CommentMustAsDiff(c *issues_model.Comment) *Diff {
	if c == nil {
		return nil
	}
	defer func() {
		if err := recover(); err != nil {
			log.Error("PANIC whilst retrieving diff for comment[%d] Error: %v\nStack: %s", c.ID, err, log.Stack(2))
		}
	}()
	diff, err := CommentAsDiff(c)
	if err != nil {
		log.Warn("CommentMustAsDiff: %v", err)
	}
	return diff
}

// GetWhitespaceFlag returns git diff flag for treating whitespaces
func GetWhitespaceFlag(whitespaceBehavior string) string {
	whitespaceFlags := map[string]string{
		"ignore-all":    "-w",
		"ignore-change": "-b",
		"ignore-eol":    "--ignore-space-at-eol",
		"show-all":      "",
	}

	if flag, ok := whitespaceFlags[whitespaceBehavior]; ok {
		return flag
	}
	log.Warn("unknown whitespace behavior: %q, default to 'show-all'", whitespaceBehavior)
	return ""
}
