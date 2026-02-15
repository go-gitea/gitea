// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"

	"code.gitea.io/gitea/modules/setting"

	"github.com/alecthomas/chroma/v2"
)

type BlobExcerptOptions struct {
	LastLeft      int
	LastRight     int
	LeftIndex     int
	RightIndex    int
	LeftHunkSize  int
	RightHunkSize int
	Direction     string
	Language      string
}

func fillExcerptLines(section *DiffSection, filePath string, reader io.Reader, lang string, idxLeft, idxRight, chunkSize int) error {
	buf := &bytes.Buffer{}
	scanner := bufio.NewScanner(reader)
	var diffLines []*DiffLine
	for line := 0; line < idxRight+chunkSize; line++ {
		if ok := scanner.Scan(); !ok {
			break
		}
		lineText := scanner.Text()
		if buf.Len()+len(lineText) < int(setting.UI.MaxDisplayFileSize) {
			buf.WriteString(lineText)
			buf.WriteByte('\n')
		}
		if line < idxRight {
			continue
		}
		diffLine := &DiffLine{
			LeftIdx:  idxLeft + (line - idxRight) + 1,
			RightIdx: line + 1,
			Type:     DiffLinePlain,
			Content:  " " + lineText,
		}
		diffLines = append(diffLines, diffLine)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("fillExcerptLines scan: %w", err)
	}
	section.Lines = diffLines
	// DiffLinePlain always uses right lines
	section.highlightedRightLines.value = highlightCodeLines(filePath, lang, []*DiffSection{section}, false /* right */, buf.Bytes())
	return nil
}

func BuildBlobExcerptDiffSection(filePath string, reader io.Reader, opts BlobExcerptOptions) (*DiffSection, error) {
	lastLeft, lastRight, idxLeft, idxRight := opts.LastLeft, opts.LastRight, opts.LeftIndex, opts.RightIndex
	leftHunkSize, rightHunkSize, direction := opts.LeftHunkSize, opts.RightHunkSize, opts.Direction
	language := opts.Language

	chunkSize := BlobExcerptChunkSize
	section := &DiffSection{
		language:              &diffVarMutable[string]{value: language},
		highlightLexer:        &diffVarMutable[chroma.Lexer]{},
		highlightedLeftLines:  &diffVarMutable[map[int]template.HTML]{},
		highlightedRightLines: &diffVarMutable[map[int]template.HTML]{},
		FileName:              filePath,
	}
	var err error
	if direction == "up" && (idxLeft-lastLeft) > chunkSize {
		idxLeft -= chunkSize
		idxRight -= chunkSize
		leftHunkSize += chunkSize
		rightHunkSize += chunkSize
		err = fillExcerptLines(section, filePath, reader, language, idxLeft-1, idxRight-1, chunkSize)
	} else if direction == "down" && (idxLeft-lastLeft) > chunkSize {
		err = fillExcerptLines(section, filePath, reader, language, lastLeft, lastRight, chunkSize)
		lastLeft += chunkSize
		lastRight += chunkSize
	} else {
		offset := -1
		if direction == "down" {
			offset = 0
		}
		err = fillExcerptLines(section, filePath, reader, language, lastLeft, lastRight, idxRight-lastRight+offset)
		leftHunkSize = 0
		rightHunkSize = 0
		idxLeft = lastLeft
		idxRight = lastRight
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
