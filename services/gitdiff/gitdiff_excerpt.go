// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"strconv"
	"strings"

	"gitea.dev/modules/base"
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
	Direction     string // an arrow expands one chunk from the end it points at, otherwise the whole gap
	Language      string
}

// SerializeGapNumbers writes the six numbers a gap is, for the browser to hand back to
// DeserializeGapNumbers
func (s *DiffLineSectionInfo) SerializeGapNumbers() string {
	return fmt.Sprintf("%d,%d,%d,%d,%d,%d",
		s.LastLeftIdx, s.LastRightIdx, s.LeftIdx, s.RightIdx, s.LeftHunkSize, s.RightHunkSize)
}

// HiddenCommentIDsCSV lists the comments a gap hides, for a CSS "attr*=,id," match to find them
func (s *DiffLineSectionInfo) HiddenCommentIDsCSV() string {
	return strings.Join(base.Int64sToStrings(s.HiddenCommentIDs), ",")
}

// DeserializeGapNumbers reads back what SerializeGapNumbers wrote
func DeserializeGapNumbers(gapNumbers string) (BlobExcerptOptions, error) {
	invalid := fmt.Errorf("invalid gap: %q", gapNumbers)
	nums := strings.Split(gapNumbers, ",")
	if len(nums) != 6 {
		return BlobExcerptOptions{}, invalid
	}
	var parsed [6]int
	for i, num := range nums {
		v, err := strconv.Atoi(num)
		if err != nil || v < 0 {
			return BlobExcerptOptions{}, invalid
		}
		parsed[i] = v
	}
	return BlobExcerptOptions{
		LastLeft: parsed[0], LastRight: parsed[1],
		LeftIndex: parsed[2], RightIndex: parsed[3],
		LeftHunkSize: parsed[4], RightHunkSize: parsed[5],
	}, nil
}

// a gap with no hunk on either side runs to the end of the file, so nothing follows it
func gapReachesFileEnd(leftHunkSize, rightHunkSize int) bool {
	return leftHunkSize <= 0 && rightHunkSize <= 0
}

// expandRange returns the 1-based inclusive lines a request expands from a gap
func (opts BlobExcerptOptions) expandRange() (leftStart, rightStart, rightEnd int) {
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

// BuildBlobExcerptDiffSections expands the requested part of each gap in a single pass over the
// blob, so that showing a whole file takes one request rather than one per gap. The caller passes
// the gaps in the order they appear in the file.
func BuildBlobExcerptDiffSections(filePath string, reader io.Reader, optsList []BlobExcerptOptions) ([]*DiffSection, error) {
	buf := &bytes.Buffer{}
	scanner := git.NewGitDiffScanner(reader)
	scanned := 0 // the last line number read from the blob
	sections := make([]*DiffSection, 0, len(optsList))
	for _, opts := range optsList {
		leftStart, rightStart, rightEnd := opts.expandRange()
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
