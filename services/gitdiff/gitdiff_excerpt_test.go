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

func TestBuildBlobExcerptDiffSections_OneChunk(t *testing.T) {
	data := &bytes.Buffer{}
	for i := range 100 {
		data.WriteString("a = " + strconv.Itoa(i+1) + "\n")
	}

	locale := translation.MockLocale{}
	lineMiddle := 50
	sections, err := BuildBlobExcerptDiffSections("a.py", bytes.NewReader(data.Bytes()), []BlobExcerptOptions{{
		LeftIndex:     lineMiddle,
		RightIndex:    lineMiddle,
		LeftHunkSize:  10,
		RightHunkSize: 10,
		Direction:     "up",
	}})
	require.NoError(t, err)
	require.Len(t, sections, 1)
	diffSection := sections[0]
	assert.Len(t, diffSection.highlightedRightLines.value, BlobExcerptChunkSize)
	assert.NotEmpty(t, diffSection.highlightedRightLines.value[lineMiddle-BlobExcerptChunkSize-1])
	assert.NotEmpty(t, diffSection.highlightedRightLines.value[lineMiddle-2]) // 0-based

	diffInline := diffSection.GetComputedInlineDiffFor(diffSection.Lines[0], locale)
	assert.Equal(t, `<span class="n">a</span> <span class="o">=</span> <span class="mi">30</span>`+"\n", string(diffInline.Content))
}

func TestBuildBlobExcerptDiffSections_WholeGaps(t *testing.T) {
	data := &bytes.Buffer{}
	for i := range 100 {
		data.WriteString("a = " + strconv.Itoa(i+1) + "\n")
	}
	// a leading gap that stops before the first rendered line, and one that runs to the end of the file
	sections, err := BuildBlobExcerptDiffSections("a.py", bytes.NewReader(data.Bytes()), []BlobExcerptOptions{
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

func TestGapNumbersRoundTrip(t *testing.T) {
	info := DiffLineSectionInfo{LastLeftIdx: 17, LastRightIdx: 31, LeftIdx: 40, RightIdx: 54, LeftHunkSize: 23, RightHunkSize: 7}
	assert.Equal(t, "17,31,40,54,23,7", info.GapNumbers())

	opts, err := ParseGapNumbers(info.GapNumbers())
	require.NoError(t, err)
	assert.Equal(t, BlobExcerptOptions{LastLeft: 17, LastRight: 31, LeftIndex: 40, RightIndex: 54, LeftHunkSize: 23, RightHunkSize: 7}, opts)

	// the numbers come back from the browser, so they are checked rather than trusted
	for _, bad := range []string{"", "1,2,3", "a,b,c,d,e,f", "-1,0,17,17,7,7", "1,2,3,4,5,6,7"} {
		_, err := ParseGapNumbers(bad)
		require.Error(t, err, bad)
	}
}
