// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"strings"

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

// BuildBlobExcerptDiffSectionsFull builds multiple excerpt sections from the same file content
// for a full-file expand. All sections share the same diffVarMutable pointers so syntax
// highlighting is computed once and shared across all sections.
func BuildBlobExcerptDiffSectionsFull(filePath string, content []byte, optsList []BlobExcerptOptions) ([]*DiffSection, error) {
	if len(optsList) == 0 || len(content) == 0 {
		return nil, nil
	}

	contentStr := strings.ReplaceAll(string(content), "\r\n", "\n")
	allLines := strings.Split(strings.TrimSuffix(contentStr, "\n"), "\n")

	// Shared mutable state across all sections — the diffVarMutable pattern
	sharedLanguage := &diffVarMutable[string]{value: optsList[0].Language}
	sharedLexer := &diffVarMutable[chroma.Lexer]{}
	sharedLeftLines := &diffVarMutable[map[int]template.HTML]{}
	sharedRightLines := &diffVarMutable[map[int]template.HTML]{}

	sections := make([]*DiffSection, 0, len(optsList))
	for _, opts := range optsList {
		section := &DiffSection{
			language:              sharedLanguage,
			highlightLexer:        sharedLexer,
			highlightedLeftLines:  sharedLeftLines,
			highlightedRightLines: sharedRightLines,
			FileName:              filePath,
		}

		// Determine line range: 1-indexed inclusive
		startLine := opts.LastRight + 1
		endLine := opts.RightIndex - 1 // between-hunk: stop before next hunk
		if opts.LeftHunkSize == 0 && opts.RightHunkSize == 0 {
			endLine = len(allLines) // EOF: expand to end of file
		}

		leftStart := opts.LastLeft + 1
		var diffLines []*DiffLine
		for lineNum := startLine; lineNum <= endLine; lineNum++ {
			lineIdx := lineNum - 1 // 0-indexed
			if lineIdx >= len(allLines) {
				break
			}
			diffLines = append(diffLines, &DiffLine{
				LeftIdx:  leftStart + (lineNum - startLine),
				RightIdx: lineNum,
				Type:     DiffLinePlain,
				Content:  " " + allLines[lineIdx],
			})
		}
		section.Lines = diffLines
		sections = append(sections, section)
	}

	// Highlight all sections in one pass — DiffLinePlain always uses right lines
	sharedRightLines.value = highlightCodeLines(filePath, sharedLanguage.value, sections, false, content)
	return sections, nil
}
