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
	Direction     string // an arrow reveals one chunk from the end it points at, otherwise the whole gap
	Language      string
	GapKey        string // the gap being revealed, so its lines stay attributable to it
}

// a gap with no hunk on either side runs to the end of the file, so nothing follows it
func gapReachesFileEnd(leftHunkSize, rightHunkSize int) bool {
	return leftHunkSize <= 0 && rightHunkSize <= 0
}

// revealRange returns the 1-based inclusive lines a request reveals from a gap
func (opts BlobExcerptOptions) revealRange() (leftStart, rightStart, rightEnd int) {
	if opts.RightIndex-opts.LastRight > BlobExcerptChunkSize {
		switch opts.Direction {
		case "up": // the chunk nearest the hunk below the gap
			return opts.LeftIndex - BlobExcerptChunkSize, opts.RightIndex - BlobExcerptChunkSize, opts.RightIndex - 1
		case "down": // the chunk nearest the hunk above it
			return opts.LastLeft + 1, opts.LastRight + 1, opts.LastRight + BlobExcerptChunkSize
		}
	}
	// the whole gap: the line at "right" is already rendered, except where the gap runs to the end
	// of the file and nothing follows it
	rightEnd = opts.RightIndex
	if !gapReachesFileEnd(opts.LeftHunkSize, opts.RightHunkSize) {
		rightEnd--
	}
	return opts.LastLeft + 1, opts.LastRight + 1, rightEnd
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

// BuildBlobExcerptDiffSections reveals the requested part of each gap in a single pass over the
// blob, so that showing a whole file takes one request rather than one per gap. The caller passes
// the gaps in the order they appear in the file.
func BuildBlobExcerptDiffSections(filePath string, reader io.Reader, optsList []BlobExcerptOptions) ([]*DiffSection, error) {
	buf := &bytes.Buffer{}
	scanner := git.NewGitDiffScanner(reader)
	scanned := 0 // the last line number read from the blob
	sections := make([]*DiffSection, 0, len(optsList))
	for _, opts := range optsList {
		leftStart, rightStart, rightEnd := opts.revealRange()
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
				LeftIdx:  leftStart + (scanned - rightStart),
				RightIdx: scanned,
				Type:     DiffLinePlain,
				Content:  " " + lineText,
			})
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("BuildBlobExcerptDiffSections scan: %w", err)
		}
		section := newExcerptSection(filePath, opts.Language)
		section.Lines, section.ExpandedFromGap = lines, opts.GapKey
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
