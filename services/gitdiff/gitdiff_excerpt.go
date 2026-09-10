// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"

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
}

func (diffSection *DiffSection) fillExcerptLines(reader io.Reader, leftStart, rightStart, chunkSize int) error {
	buf := &bytes.Buffer{}
	scanner := bufio.NewScanner(reader)
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
			LeftIdx:  leftStart + (rightLineIdx - rightStart),
			RightIdx: rightLineIdx,
			Type:     DiffLinePlain,
			Content:  " " + lineText,
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

func BuildBlobExcerptDiffSection(filePath string, reader io.Reader, opts BlobExcerptOptions) (*DiffSection, error) {
	lastLeft, lastRight, idxLeft, idxRight := opts.LastLeft, opts.LastRight, opts.LeftIndex, opts.RightIndex
	leftHunkSize, rightHunkSize, direction := opts.LeftHunkSize, opts.RightHunkSize, opts.Direction

	expandLimit := BlobExcerptChunkSize
	section := &DiffSection{
		language:              &diffVarMutable[string]{value: opts.Language},
		highlightLexer:        &diffVarMutable[chroma.Lexer]{},
		highlightedLeftLines:  &diffVarMutable[map[int]template.HTML]{},
		highlightedRightLines: &diffVarMutable[map[int]template.HTML]{},
		FileName:              filePath,
	}
	var err error
	remainingLines := idxRight - lastRight
	if direction == "up" && remainingLines > expandLimit {
		idxLeft -= expandLimit
		idxRight -= expandLimit
		leftHunkSize += expandLimit
		rightHunkSize += expandLimit
		err = section.fillExcerptLines(reader, idxLeft, idxRight, expandLimit)
	} else if direction == "down" && remainingLines > expandLimit {
		err = section.fillExcerptLines(reader, lastLeft+1, lastRight+1, expandLimit)
		lastLeft += expandLimit
		lastRight += expandLimit
	} else /* "single" or [ ("up" or "down") and (remainingLines <= expandLimit) ] */ {
		if direction == "up" || direction == "single" {
			// if the direction is "up" or "single":
			// * top: last=0, idx=11, chunk=11: line 11 is already rendered, line 0 can be considered as a "virtually rendered line"
			//   * then need to expand line 10 lines (1-10), so "-1".
			// * middle: last=100, idx=106, chunk=6: line 100 and 106 are both already rendered
			//   * then need to expand 5 lines (101-105), so "-1".
			expandLimit = remainingLines - 1
		} else {
			// if the direction is "down": either the hidden lines are too many in the middle (otherwise "single"), or are at the bottom
			// * "last" line is already rendered, so just render the remaining lines from the next line
			expandLimit = remainingLines
		}
		err = section.fillExcerptLines(reader, lastLeft+1, lastRight+1, expandLimit)
		// now, the hidden lines are fewer than "expand limit", after expand, no hidden lines anymore,
		// no need to show new "expand buttons" (setting them to 0 will make GetExpandDirection returns "no direction")
		leftHunkSize, rightHunkSize, idxLeft, idxRight = 0, 0, 0, 0
	}
	if err != nil {
		return nil, err
	}

	newLineSection := &DiffLine{
		Type: DiffLineSection,
		SectionInfo: &DiffLineSectionInfo{
			language:      &diffVarMutable[string]{value: opts.Language},
			Path:          filePath,
			LastLeftIdx:   lastLeft,
			LastRightIdx:  lastRight,
			LeftIdx:       idxLeft,
			RightIdx:      idxRight,
			LeftHunkSize:  leftHunkSize,
			RightHunkSize: rightHunkSize,
		},
	}
	if newLineSection.GetExpandDirection() != "" {
		newLineSection.Content = fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", idxLeft, leftHunkSize, idxRight, rightHunkSize)
		switch direction {
		case "up":
			section.Lines = append([]*DiffLine{newLineSection}, section.Lines...)
		case "down":
			section.Lines = append(section.Lines, newLineSection)
		}
	}
	return section, nil
}
