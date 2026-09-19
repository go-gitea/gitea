// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"bytes"
	"fmt"
	"html/template"
	"io"

	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"

	"github.com/alecthomas/chroma/v2"
)

type BlobExcerptOptions struct {
	// More details in DiffLineSectionInfo struct
	LastLeft      int
	LastRight     int
	LeftIndex     int
	RightIndex    int
	LeftHunkSize  int
	RightHunkSize int
	Direction     string
	Language      string
	GapKey        string // the gap being expanded, so its lines stay attributable to it
}

func (diffSection *DiffSection) fillExcerptLines(reader io.Reader, leftStart, rightStart, chunkSize int, gapKey string) error {
	buf := &bytes.Buffer{}
	scanner := git.NewGitDiffScanner(reader)
	var diffLines []*DiffLine
	for rightLineIdx := 1; rightLineIdx < rightStart+chunkSize; rightLineIdx++ {
		if ok := scanner.Scan(); !ok {
			break
		}
		lineText := scanner.Text()
		if buf.Len()+len(lineText) < int(setting.UI.MaxDisplayFileSize) {
			buf.WriteString(lineText)
			buf.WriteByte('\n')
		}
		if rightLineIdx < rightStart {
			continue
		}
		diffLine := &DiffLine{
			LeftIdx:         leftStart + (rightLineIdx - rightStart),
			RightIdx:        rightLineIdx,
			Type:            DiffLinePlain,
			Content:         " " + lineText,
			ExpandedFromGap: gapKey,
		}
		diffLines = append(diffLines, diffLine)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("fillExcerptLines scan: %w", err)
	}
	diffSection.Lines = diffLines
	// DiffLinePlain always uses right lines
	diffSection.highlightedRightLines.value = highlightCodeLines(diffSection.FileName, diffSection.language.value, []*DiffSection{diffSection}, false /* right */, buf.Bytes())
	return nil
}

// a gap with no hunk on either side runs to the end of the file, so nothing follows it
func gapReachesFileEnd(leftHunkSize, rightHunkSize int) bool {
	return leftHunkSize <= 0 && rightHunkSize <= 0
}

func newExcerptSection(filePath, language string) *DiffSection {
	return &DiffSection{
		language:              &diffVarMutable[string]{value: language},
		highlightLexer:        &diffVarMutable[chroma.Lexer]{},
		highlightedLeftLines:  &diffVarMutable[map[int]template.HTML]{},
		highlightedRightLines: &diffVarMutable[map[int]template.HTML]{},
		FileName:              filePath,
	}
}

// BuildBlobExcerptDiffSectionsForGaps reveals several gaps of one file in a single pass over the
// blob, so that showing a whole file takes one request rather than one per gap. The caller passes
// the gaps in the order they appear in the file.
func BuildBlobExcerptDiffSectionsForGaps(filePath string, reader io.Reader, optsList []BlobExcerptOptions) ([]*DiffSection, error) {
	buf := &bytes.Buffer{}
	scanner := git.NewGitDiffScanner(reader)
	scanned := 0 // the last line number read from the blob
	sections := make([]*DiffSection, 0, len(optsList))
	for _, opts := range optsList {
		rightEnd := opts.RightIndex
		if !gapReachesFileEnd(opts.LeftHunkSize, opts.RightHunkSize) {
			rightEnd-- // the line at "right" is already rendered
		}
		leftStart, rightStart := opts.LastLeft+1, opts.LastRight+1
		var lines []*DiffLine
		for scanned < rightEnd {
			if ok := scanner.Scan(); !ok {
				break
			}
			scanned++
			lineText := scanner.Text()
			if buf.Len()+len(lineText) < int(setting.UI.MaxDisplayFileSize) {
				buf.WriteString(lineText)
				buf.WriteByte('\n')
			}
			if scanned < rightStart {
				continue
			}
			lines = append(lines, &DiffLine{
				LeftIdx:         leftStart + (scanned - rightStart),
				RightIdx:        scanned,
				Type:            DiffLinePlain,
				Content:         " " + lineText,
				ExpandedFromGap: opts.GapKey,
			})
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("BuildBlobExcerptDiffSectionsForGaps scan: %w", err)
		}
		section := newExcerptSection(filePath, opts.Language)
		section.Lines = lines
		sections = append(sections, section)
	}
	if len(sections) > 0 {
		// DiffLinePlain always uses right lines, and one pass highlights them all
		highlighted := highlightCodeLines(filePath, optsList[0].Language, sections, false /* right */, buf.Bytes())
		for _, section := range sections {
			section.highlightedRightLines.value = highlighted
		}
	}
	return sections, nil
}

// BuildBlobExcerptDiffSection reveals one chunk of a gap. The caller keeps track of what the gap has
// left, so this only produces the lines.
func BuildBlobExcerptDiffSection(filePath string, reader io.Reader, opts BlobExcerptOptions) (*DiffSection, error) {
	lastLeft, lastRight, idxLeft, idxRight := opts.LastLeft, opts.LastRight, opts.LeftIndex, opts.RightIndex
	expandLimit := BlobExcerptChunkSize
	section := newExcerptSection(filePath, opts.Language)
	var err error
	remainingLines := idxRight - lastRight
	switch {
	case opts.Direction == "up" && remainingLines > expandLimit:
		err = section.fillExcerptLines(reader, idxLeft-expandLimit, idxRight-expandLimit, expandLimit, opts.GapKey)
	case opts.Direction == "down" && remainingLines > expandLimit:
		err = section.fillExcerptLines(reader, lastLeft+1, lastRight+1, expandLimit, opts.GapKey)
	default:
		// the whole gap is revealed at once: the line at "idx" is already rendered, except where the
		// gap runs to the end of the file and nothing follows it
		expandLimit = remainingLines
		if !gapReachesFileEnd(opts.LeftHunkSize, opts.RightHunkSize) {
			expandLimit--
		}
		err = section.fillExcerptLines(reader, lastLeft+1, lastRight+1, expandLimit, opts.GapKey)
	}
	if err != nil {
		return nil, err
	}
	return section, nil
}
