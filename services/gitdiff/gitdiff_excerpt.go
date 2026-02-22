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

// fillExcerptLines reads from reader and populates section.Lines.
// It returns the accumulated content buffer for later highlighting.
func fillExcerptLines(section *DiffSection, reader io.Reader, idxLeft, idxRight, chunkSize int) ([]byte, error) {
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
		return nil, fmt.Errorf("fillExcerptLines scan: %w", err)
	}
	section.Lines = diffLines
	return buf.Bytes(), nil
}

// buildExcerptDiffSection builds a single excerpt section without highlighting.
// It returns the section and the accumulated content buffer.
func buildExcerptDiffSection(filePath string, reader io.Reader, opts BlobExcerptOptions) (*DiffSection, []byte, error) {
	lastLeft, lastRight, idxLeft, idxRight := opts.LastLeft, opts.LastRight, opts.LeftIndex, opts.RightIndex
	leftHunkSize, rightHunkSize, direction := opts.LeftHunkSize, opts.RightHunkSize, opts.Direction

	chunkSize := BlobExcerptChunkSize
	section := &DiffSection{
		language:              &diffVarMutable[string]{value: opts.Language},
		highlightLexer:        &diffVarMutable[chroma.Lexer]{},
		highlightedLeftLines:  &diffVarMutable[map[int]template.HTML]{},
		highlightedRightLines: &diffVarMutable[map[int]template.HTML]{},
		FileName:              filePath,
	}
	var bufContent []byte
	var err error
	if direction == "up" && (idxLeft-lastLeft) > chunkSize {
		idxLeft -= chunkSize
		idxRight -= chunkSize
		leftHunkSize += chunkSize
		rightHunkSize += chunkSize
		bufContent, err = fillExcerptLines(section, reader, idxLeft-1, idxRight-1, chunkSize)
	} else if direction == "down" && (idxLeft-lastLeft) > chunkSize {
		bufContent, err = fillExcerptLines(section, reader, lastLeft, lastRight, chunkSize)
		lastLeft += chunkSize
		lastRight += chunkSize
	} else {
		offset := -1
		if direction == "down" {
			offset = 0
		}
		bufContent, err = fillExcerptLines(section, reader, lastLeft, lastRight, idxRight-lastRight+offset)
		leftHunkSize = 0
		rightHunkSize = 0
		idxLeft = lastLeft
		idxRight = lastRight
	}
	if err != nil {
		return nil, nil, err
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
	return section, bufContent, nil
}

// BuildBlobExcerptDiffSection builds a single excerpt section with highlighting.
func BuildBlobExcerptDiffSection(filePath string, reader io.Reader, opts BlobExcerptOptions) (*DiffSection, error) {
	section, bufContent, err := buildExcerptDiffSection(filePath, reader, opts)
	if err != nil {
		return nil, err
	}
	// DiffLinePlain always uses right lines
	section.highlightedRightLines.value = highlightCodeLines(filePath, opts.Language, []*DiffSection{section}, false /* right */, bufContent)
	return section, nil
}

// BuildBlobExcerptDiffSections builds multiple excerpt sections from the same file content,
// highlighting the content only once for all sections.
func BuildBlobExcerptDiffSections(filePath string, content []byte, optsList []BlobExcerptOptions) ([]*DiffSection, error) {
	sections := make([]*DiffSection, len(optsList))
	for i, opts := range optsList {
		section, _, err := buildExcerptDiffSection(filePath, bytes.NewReader(content), opts)
		if err != nil {
			return nil, err
		}
		sections[i] = section
	}

	// Highlight once for all sections
	if len(optsList) > 0 {
		highlighted := highlightCodeLines(filePath, optsList[0].Language, sections, false /* right */, content)
		for _, section := range sections {
			section.highlightedRightLines.value = highlighted
		}
	}

	return sections, nil
}
