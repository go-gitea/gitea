// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"bytes"
	"strconv"
	"testing"

	"gitea.dev/modules/translation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildBlobExcerptDiffSection(t *testing.T) {
	data := &bytes.Buffer{}
	for i := range 100 {
		data.WriteString("a = " + strconv.Itoa(i+1) + "\n")
	}

	locale := translation.MockLocale{}
	lineMiddle := 50
	diffSection, err := BuildBlobExcerptDiffSection("a.py", bytes.NewReader(data.Bytes()), BlobExcerptOptions{
		LeftIndex:     lineMiddle,
		RightIndex:    lineMiddle,
		LeftHunkSize:  10,
		RightHunkSize: 10,
		Direction:     "up",
	})
	require.NoError(t, err)
	assert.Len(t, diffSection.highlightedRightLines.value, BlobExcerptChunkSize)
	assert.NotEmpty(t, diffSection.highlightedRightLines.value[lineMiddle-BlobExcerptChunkSize-1])
	assert.NotEmpty(t, diffSection.highlightedRightLines.value[lineMiddle-2]) // 0-based

	diffInline := diffSection.GetComputedInlineDiffFor(diffSection.Lines[0], locale)
	assert.Equal(t, `<span class="n">a</span> <span class="o">=</span> <span class="mi">30</span>`+"\n", string(diffInline.Content))
}

func TestDiffLineSectionInfoHiddenLineRange(t *testing.T) {
	cases := []struct {
		name                            string
		info                            DiffLineSectionInfo
		leftStart, rightStart, rightEnd int
	}{
		{"top", DiffLineSectionInfo{LeftIdx: 40, RightIdx: 54, LeftHunkSize: 23, RightHunkSize: 7}, 1, 1, 53},
		{"middle", DiffLineSectionInfo{LastLeftIdx: 17, LastRightIdx: 31, LeftIdx: 40, RightIdx: 54, LeftHunkSize: 23, RightHunkSize: 7}, 18, 32, 53},
		{"end of file", DiffLineSectionInfo{LastLeftIdx: 62, LastRightIdx: 60, LeftIdx: 80, RightIdx: 78}, 63, 61, 78},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			leftStart, rightStart, rightEnd := c.info.hiddenLineRange()
			assert.Equal(t, []int{c.leftStart, c.rightStart, c.rightEnd}, []int{leftStart, rightStart, rightEnd})
			assert.Equal(t, strconv.Itoa(c.info.LastRightIdx)+"-"+strconv.Itoa(c.info.RightIdx), c.info.GapKey())
		})
	}
}

func TestBuildBlobExcerptDiffSectionsForGaps(t *testing.T) {
	data := &bytes.Buffer{}
	for i := range 100 {
		data.WriteString("a = " + strconv.Itoa(i+1) + "\n")
	}
	// a leading gap that stops before the first rendered line, and one that runs to the end of the file
	sections, err := BuildBlobExcerptDiffSectionsForGaps("a.py", bytes.NewReader(data.Bytes()), []BlobExcerptOptions{
		{LastLeft: 0, LastRight: 0, LeftIndex: 13, RightIndex: 13, LeftHunkSize: 6, RightHunkSize: 7},
		{LastLeft: 80, LastRight: 80, LeftIndex: 100, RightIndex: 100},
	})
	require.NoError(t, err)
	require.Len(t, sections, 2)
	assert.Len(t, sections[0].Lines, 12) // lines 1-12, the line at 13 is already rendered
	assert.Equal(t, 1, sections[0].Lines[0].RightIdx)
	assert.Len(t, sections[1].Lines, 20) // lines 81-100, nothing follows the gap
	assert.Equal(t, 100, sections[1].Lines[19].RightIdx)
}
