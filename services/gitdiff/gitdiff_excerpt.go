// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"slices"
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

// DeserializeGapRequest reads a gap's numbers, plus the end an arrow takes a single chunk from
func DeserializeGapRequest(gapRequest string) (BlobExcerptOptions, error) {
	if numbers, direction, ok := strings.CutLast(gapRequest, ","); ok && (direction == "up" || direction == "down") {
		opts, err := DeserializeGapNumbers(numbers)
		if err != nil {
			return BlobExcerptOptions{}, err
		}
		opts.Direction = direction
		return opts, nil
	}
	return DeserializeGapNumbers(gapRequest)
}

const maxExcerptGaps = 1000 // rejects absurd input, the line limit is what bounds the work

// MaxExcerptLines is how many lines one request may expand, the same bound a diff file renders
func MaxExcerptLines() int {
	return max(setting.Git.MaxGitDiffLines, BlobExcerptChunkSize)
}

// DeserializeGapRequests reads every gap a request names, in the order they appear in the file
func DeserializeGapRequests(gapRequests []string, language string) ([]BlobExcerptOptions, error) {
	if len(gapRequests) == 0 {
		return nil, errors.New("no gap requested")
	}
	if len(gapRequests) > maxExcerptGaps {
		return nil, errors.New("too many gaps requested")
	}
	gapOpts := make([]BlobExcerptOptions, 0, len(gapRequests))
	lineCount := 0
	for _, gapRequest := range gapRequests {
		opts, err := DeserializeGapRequest(gapRequest)
		if err != nil {
			return nil, err
		}
		_, rightStart, rightEnd := opts.expandRange()
		if lineCount += max(rightEnd-rightStart+1, 0); lineCount > MaxExcerptLines() {
			return nil, errors.New("too many lines requested")
		}
		opts.Language = language
		gapOpts = append(gapOpts, opts)
	}
	// expanded in one pass over the file, so the gaps have to be in the order they appear in it
	slices.SortFunc(gapOpts, func(a, b BlobExcerptOptions) int { return a.LastRight - b.LastRight })
	return gapOpts, nil
}

// a gap with no hunk on either side runs to the end of the file, so nothing follows it
func gapReachesFileEnd(leftHunkSize, rightHunkSize int) bool {
	return leftHunkSize <= 0 && rightHunkSize <= 0
}

// the last line a whole gap hides: the line at "right" is already rendered, unless nothing follows the gap
func gapLastHiddenLine(rightIdx, leftHunkSize, rightHunkSize int) int {
	if gapReachesFileEnd(leftHunkSize, rightHunkSize) {
		return rightIdx
	}
	return rightIdx - 1
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
	return opts.LastLeft + 1, opts.LastRight + 1, gapLastHiddenLine(opts.RightIndex, opts.LeftHunkSize, opts.RightHunkSize)
}

// SerializeRemainingGap writes the numbers of what the gap still hides once this request expanded it
func (opts BlobExcerptOptions) SerializeRemainingGap() string {
	remaining := DiffLineSectionInfo{LastLeftIdx: opts.LastLeft, LastRightIdx: opts.LastRight}
	if opts.RightIndex-opts.LastRight > BlobExcerptChunkSize {
		switch opts.Direction {
		case "up":
			remaining.LeftIdx, remaining.RightIdx = opts.LeftIndex-BlobExcerptChunkSize, opts.RightIndex-BlobExcerptChunkSize
			remaining.LeftHunkSize, remaining.RightHunkSize = opts.LeftHunkSize+BlobExcerptChunkSize, opts.RightHunkSize+BlobExcerptChunkSize
		case "down":
			remaining.LastLeftIdx, remaining.LastRightIdx = opts.LastLeft+BlobExcerptChunkSize, opts.LastRight+BlobExcerptChunkSize
			remaining.LeftIdx, remaining.RightIdx = opts.LeftIndex, opts.RightIndex
			remaining.LeftHunkSize, remaining.RightHunkSize = opts.LeftHunkSize, opts.RightHunkSize
		}
	}
	return remaining.SerializeGapNumbers() // all zero after the last line means nothing is left
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

// BuildBlobExcerptDiffSections expands each gap, given in file order, in one pass over the blob
func BuildBlobExcerptDiffSections(filePath string, reader io.Reader, optsList []BlobExcerptOptions) ([]*DiffSection, error) {
	buf := &bytes.Buffer{}
	bufFull := false
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
			// the highlighted lines are matched by number, so the buffer must stop at the first line that does not fit
			if bufFull = bufFull || buf.Len()+len(lineText)+1 >= MaxFullFileHighlightSizeLimit; !bufFull {
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
