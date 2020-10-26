// Copyright (c) 2012-2016 The go-diff Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// This file is a based on "sergi/go-diff"'s patch.go
// with the following fixes:
//  * That nonsensical query escaping is removed
//  * The line numbers in the generated patch are based on the number of lines (duh!) instead of the number of characters
//  * The generated contextual line (before and after the changes to show context) are now full lines instead of just partial lines.

var diffMatchPatchForPatches = diffmatchpatch.New()

func init() {
	diffMatchPatchForPatches.DiffEditCost = 1
}

// Patch represents one patch operation.
type patch struct {
	diffs       []diffmatchpatch.Diff
	Start1      int
	StartLine1  int
	Start2      int
	StartLine2  int
	Length1     int
	LengthLine1 int
	Length2     int
	LengthLine2 int
}

func lineCount(s string) int {
	return len(lines(s))
}

func lines(s string) []string {
	lines := strings.Split(s, "\n")
	l := len(lines)
	if l > 0 && lines[l-1] == "" {
		lines = lines[:l-1]
	}
	return lines
}

// String emulates GNU diff's format.
// Header: @@ -382,8 +481,9 @@
// Indices are printed as 1-based, not 0-based.
func (p *patch) String() string {
	var coords1, coords2 string

	if p.LengthLine1 == 0 {
		coords1 = strconv.Itoa(p.StartLine1) + ",0"
	} else if p.LengthLine1 == 1 {
		coords1 = strconv.Itoa(p.StartLine1 + 1)
	} else {
		coords1 = strconv.Itoa(p.StartLine1+1) + "," + strconv.Itoa(p.LengthLine1)
	}

	if p.LengthLine2 == 0 {
		coords2 = strconv.Itoa(p.StartLine2) + ",0"
	} else if p.LengthLine2 == 1 {
		coords2 = strconv.Itoa(p.StartLine2 + 1)
	} else {
		coords2 = strconv.Itoa(p.StartLine2+1) + "," + strconv.Itoa(p.LengthLine2)
	}

	var text bytes.Buffer
	_, _ = text.WriteString("@@ -" + coords1 + " +" + coords2 + " @@\n")

	for _, aDiff := range p.diffs {
		noNewLineEOF := false
		if !strings.HasSuffix(aDiff.Text, "\n") {
			noNewLineEOF = true
		}
		lines := lines(aDiff.Text)

		for _, line := range lines {
			switch aDiff.Type {
			case diffmatchpatch.DiffInsert:
				_, _ = text.WriteString("+")
			case diffmatchpatch.DiffDelete:
				_, _ = text.WriteString("-")
			case diffmatchpatch.DiffEqual:
				_, _ = text.WriteString(" ")
			}

			_, _ = text.WriteString(line)
			_, _ = text.WriteString("\n")
		}
		if noNewLineEOF {
			_, _ = text.WriteString("\\ No newline at end of file\n")
		}
	}

	return text.String()
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func patchAddContext(dmp *diffmatchpatch.DiffMatchPatch, patch patch, text string) patch {
	if len(text) == 0 {
		return patch
	}

	lines := strings.SplitAfter(text, "\n")

	pattern := strings.Join(lines[patch.StartLine2:patch.StartLine2+patch.LengthLine1], "")
	padding := 0

	// Look for the first and last matches of pattern in text.  If two different matches are found, increase the pattern length.
	for strings.Index(text, pattern) != strings.LastIndex(text, pattern) &&
		lineCount(pattern) < dmp.MatchMaxBits-2*dmp.PatchMargin {
		padding += dmp.PatchMargin
		maxStart := max(0, patch.StartLine2-padding)
		minEnd := min(lineCount(text), patch.StartLine2+patch.LengthLine1+padding)
		pattern = strings.Join(lines[maxStart:minEnd], "")
	}
	// Add one chunk for good luck.
	padding += dmp.PatchMargin

	// Add the prefix.
	prefix := strings.Join(lines[max(0, patch.StartLine2-padding):patch.StartLine2], "")
	if len(prefix) != 0 {
		patch.diffs = append([]diffmatchpatch.Diff{{Type: diffmatchpatch.DiffEqual, Text: prefix}}, patch.diffs...)
	}
	// Add the suffix.
	suffix := strings.Join(lines[patch.StartLine2+patch.LengthLine1:min(len(lines), patch.StartLine2+patch.LengthLine1+padding)], "")
	if len(suffix) != 0 {
		patch.diffs = append(patch.diffs, diffmatchpatch.Diff{Type: diffmatchpatch.DiffEqual, Text: suffix})
	}

	// Roll back the start points.
	patch.Start1 -= len(prefix)
	patch.StartLine1 -= lineCount(prefix)
	patch.Start2 -= len(prefix)
	patch.StartLine2 -= lineCount(prefix)
	// Extend the lengths.
	patch.Length1 += len(prefix) + len(suffix)
	patch.Length2 += len(prefix) + len(suffix)
	patch.LengthLine1 += lineCount(prefix) + lineCount(suffix)
	patch.LengthLine2 += lineCount(prefix) + lineCount(suffix)

	return patch
}

func patchMake2(dmp *diffmatchpatch.DiffMatchPatch, text1 string, diffs []diffmatchpatch.Diff) []patch {
	// Check for null inputs not needed since null can't be passed in C#.
	patches := []patch{}
	if len(diffs) == 0 {
		return patches // Get rid of the null case.
	}

	p := patch{}
	charCount1 := 0 // Number of characters into the text1 string.
	charCount2 := 0 // Number of characters into the text2 string.
	lineCount1 := 0 // Number of lines into the text1 string.
	lineCount2 := 0 // Number of lines into the text2 string.
	// Start with text1 (prepatchText) and apply the diffs until we arrive at text2 (postpatchText). We recreate the patches one by one to determine context info.
	prepatchText := text1
	postpatchText := text1

	for i, aDiff := range diffs {
		if len(p.diffs) == 0 && aDiff.Type != diffmatchpatch.DiffEqual {
			// A new patch starts here.
			p.Start1 = charCount1
			p.Start2 = charCount2
			p.StartLine1 = lineCount1
			p.StartLine2 = lineCount2
		}

		switch aDiff.Type {
		case diffmatchpatch.DiffInsert:
			p.diffs = append(p.diffs, aDiff)
			p.Length2 += len(aDiff.Text)
			p.LengthLine2 += lineCount(aDiff.Text)
			postpatchText = postpatchText[:charCount2] +
				aDiff.Text + postpatchText[charCount2:]
		case diffmatchpatch.DiffDelete:
			p.Length1 += len(aDiff.Text)
			p.LengthLine1 += lineCount(aDiff.Text)
			p.diffs = append(p.diffs, aDiff)
			postpatchText = postpatchText[:charCount2] + postpatchText[charCount2+len(aDiff.Text):]
		case diffmatchpatch.DiffEqual:
			if lineCount(aDiff.Text) <= 2*dmp.PatchMargin &&
				len(p.diffs) != 0 && i != len(diffs)-1 {
				// Small equality inside a patch.
				p.diffs = append(p.diffs, aDiff)
				p.Length1 += len(aDiff.Text)
				p.Length2 += len(aDiff.Text)
				p.LengthLine1 += lineCount(aDiff.Text)
				p.LengthLine2 += lineCount(aDiff.Text)
			}
			if lineCount(aDiff.Text) >= 2*dmp.PatchMargin {
				// Time for a new patch.
				if len(p.diffs) != 0 {
					p = patchAddContext(dmp, p, prepatchText)
					patches = append(patches, p)
					p = patch{}
					// Unlike Unidiff, our patch lists have a rolling context. http://code.google.com/p/google-diff-match-patch/wiki/Unidiff Update prepatch text & pos to reflect the application of the just completed patch.
					prepatchText = postpatchText
					charCount1 = charCount2
					lineCount1 = lineCount2
				}
			}
		}

		// Update the current character count.
		if aDiff.Type != diffmatchpatch.DiffInsert {
			charCount1 += len(aDiff.Text)
			lineCount1 += lineCount(aDiff.Text)
		}
		if aDiff.Type != diffmatchpatch.DiffDelete {
			charCount2 += len(aDiff.Text)
			lineCount2 += lineCount(aDiff.Text)
		}
	}

	// Pick up the leftover patch if not empty.
	if len(p.diffs) != 0 {
		p = patchAddContext(dmp, p, prepatchText)
		patches = append(patches, p)
	}

	return patches
}

func patchToText(patches []patch) string {
	var text bytes.Buffer
	for _, aPatch := range patches {
		_, _ = text.WriteString(aPatch.String())
	}
	return text.String()
}

// GetDiffAsUnifiedPatch Gives the diff of the arguments as a patch
func GetDiffAsUnifiedPatch(s1, s2 string) string {
	r1, r2, lineArray := diffMatchPatchForPatches.DiffLinesToRunes(s1, s2)
	diffs := diffMatchPatchForPatches.DiffMainRunes(r1, r2, true)
	diffs = diffMatchPatchForPatches.DiffCharsToLines(diffs, lineArray)
	patch := patchMake2(diffMatchPatchForPatches, s1, diffs)
	return patchToText(patch)
}

// GetUnifiedGitDiff Gives the diff of the arguments as a patch with the given filename
func GetUnifiedGitDiff(filename, s1, s2 string) string {
	return GetDiffAsUnifiedPatchWithFilenames(true, "a/"+filename, "b/"+filename, s1, s2)
}

// GetDiffAsUnifiedPatchWithFilenames Gives the diff of the arguments as a patch with the given filenames
func GetDiffAsUnifiedPatchWithFilenames(git bool, filename1, filename2, s1, s2 string) string {
	patch := GetDiffAsUnifiedPatch(s1, s2)

	if patch == "" {
		return ""
	}

	prefix := ""
	if git {
		prefix = "diff --git " + filename1 + " " + filename2 + "\n"
	}
	return prefix + "--- " + filename1 + "\n" + "+++ " + filename2 + "\n" + patch
}

// PatchToGitDiff Creates an approximate diff --git from a regural patch
func PatchToGitDiff(s string) string {
	if s == "" {
		return ""
	}
	var text bytes.Buffer

	chunks := strings.Split("\n"+s, "\n@@ ")

	lastTwoLines := ""
	first := true
	headerWritten := false

	for i, chunk := range chunks {
		lines := strings.Split(strings.TrimLeft(chunk, "\n \t"), "\n")

		lastHeader := strings.Split(lastTwoLines, "\n")

		if first && len(lines) < 2 {
			// incorrect format
			return s
		}
		header := lastHeader
		prevChunkLastTwoLines := lastTwoLines
		lastHeader = lines[len(lines)-2:]
		lastTwoLines = strings.Join(lastHeader, "\n")

		if first {
			first = false
			continue
		}

		if len(header) == 2 && strings.HasPrefix(header[0], "--- ") && !strings.HasPrefix(header[0], "+++ ") {
			var from, to string
			var fromQuoted, toQuoted bool
			var n int
			var err error

			if strings.HasPrefix(header[0], "--- \"") {
				n, err = fmt.Sscanf(header[0], "--- \"%q\"", &from)
				fromQuoted = true
			} else {
				n, err = fmt.Sscanf(header[0], "--- %s", &from)
				fromQuoted = false
			}
			if err != nil && n != 1 {
				return s
			}

			if strings.HasPrefix(header[1], "+++ \"") {
				n, err = fmt.Sscanf(header[1], "+++ \"%q\"", &to)
				toQuoted = true
			} else {
				n, err = fmt.Sscanf(header[1], "+++ %s", &to)
				toQuoted = false
			}
			if err != nil || n != 1 {
				return s
			}

			text.WriteString("diff --git ")
			if fromQuoted {
				text.WriteString(fmt.Sprintf("\"%q\"", from))
			} else {
				text.WriteString(from)
			}
			text.WriteString(" ")
			if toQuoted {
				text.WriteString(fmt.Sprintf("\"%q\"", to))
			} else {
				text.WriteString(to)
			}
			text.WriteString("\n")
			text.WriteString(prevChunkLastTwoLines)
			text.WriteString("\n")
			text.WriteString("@@ ")
			if i == len(chunks)-1 {
				text.WriteString(strings.Join(lines, "\n"))
			} else {
				if len(lines) < 2 {
					// We are not at the end of the patch but the next chunk doesn't have a header wtf
					return s
				}
				text.WriteString(strings.Join(lines[0:len(lines)-2], "\n"))
				text.WriteString("\n")
			}
		} else {
			// This chunk doesn't start with a file information about what file being compared
			if headerWritten { // this is not a new file just a new chunk.
				text.WriteString(prevChunkLastTwoLines)
				text.WriteString("\n@@ ")
				if len(lines) < 2 {
					// incorrect format
					return s
				}
				text.WriteString(strings.Join(lines[0:len(lines)-2], "\n"))
				continue
			}

			// incorrect format
			return s
		}
	}

	return text.String()
}
