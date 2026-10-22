// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"bytes"
	"slices"
	"strconv"
	"strings"
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
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

func TestSerializeGapNumbersRoundTrip(t *testing.T) {
	info := DiffLineSectionInfo{LastLeftIdx: 17, LastRightIdx: 31, LeftIdx: 40, RightIdx: 54, LeftHunkSize: 23, RightHunkSize: 7}
	assert.Equal(t, "17,31,40,54,23,7", info.SerializeGapNumbers())

	opts, err := DeserializeGapNumbers(info.SerializeGapNumbers())
	require.NoError(t, err)
	assert.Equal(t, BlobExcerptOptions{LastLeft: 17, LastRight: 31, LeftIndex: 40, RightIndex: 54, LeftHunkSize: 23, RightHunkSize: 7}, opts)

	// the numbers come back from the browser, so they are checked rather than trusted
	for _, bad := range []string{"", "1,2,3", "a,b,c,d,e,f", "-1,0,17,17,7,7", "1,2,3,4,5,6,7"} {
		_, err := DeserializeGapNumbers(bad)
		require.Error(t, err, bad)
	}
}

func TestDeserializeGapRequest(t *testing.T) {
	// a request is the gap's numbers, and an arrow appends the end it expands from
	opts, err := DeserializeGapRequest("17,31,40,54,23,7")
	require.NoError(t, err)
	assert.Empty(t, opts.Direction) // the whole gap

	opts, err = DeserializeGapRequest("17,31,40,54,23,7,up")
	require.NoError(t, err)
	assert.Equal(t, "up", opts.Direction)
	assert.Equal(t, 54, opts.RightIndex) // the numbers still read the same

	for _, bad := range []string{"17,31,40,54,23,7,sideways", "17,31,40,54,23,up", "up", "17,31,40,54,23,7,up,up"} {
		_, err := DeserializeGapRequest(bad)
		require.Error(t, err, bad)
	}
}

func TestDeserializeGapRequests(t *testing.T) {
	// one pass over the blob expands them all, so they come back in the order they appear in it
	gapOpts, err := DeserializeGapRequests([]string{"80,80,100,100,0,0", "0,0,13,13,6,7"}, "python")
	require.NoError(t, err)
	require.Len(t, gapOpts, 2)
	assert.Equal(t, 0, gapOpts[0].LastRight)
	assert.Equal(t, 80, gapOpts[1].LastRight)
	assert.Equal(t, "python", gapOpts[0].Language)

	_, err = DeserializeGapRequests(nil, "")
	require.Error(t, err)
	_, err = DeserializeGapRequests(slices.Repeat([]string{"5,5,6,6,1,1"}, maxExcerptGaps+1), "") // gaps hiding no line
	require.Error(t, err)

	defer test.MockVariableValue(&setting.Git.MaxGitDiffLines, 100)()
	_, err = DeserializeGapRequests([]string{"0,0,101,101,0,0"}, "") // a whole file of 101 lines
	require.Error(t, err)
	_, err = DeserializeGapRequests([]string{"0,0,101,101,0,0,down"}, "") // one chunk of it
	require.NoError(t, err)
}

func TestSerializeRemainingGap(t *testing.T) {
	gap := BlobExcerptOptions{LastLeft: 23, LastRight: 23, LeftIndex: 57, RightIndex: 57, LeftHunkSize: 7, RightHunkSize: 7}
	gap.Direction = "down"
	assert.Equal(t, "43,43,57,57,7,7", gap.SerializeRemainingGap())
	gap.Direction = "up"
	assert.Equal(t, "23,23,37,37,27,27", gap.SerializeRemainingGap())
	gap.Direction = ""
	assert.Equal(t, "23,23,0,0,0,0", gap.SerializeRemainingGap())
	// a chunk that takes the rest of the gap leaves nothing
	gap = BlobExcerptOptions{LastLeft: 43, LastRight: 43, LeftIndex: 57, RightIndex: 57, LeftHunkSize: 7, RightHunkSize: 7, Direction: "up"}
	assert.Equal(t, "43,43,0,0,0,0", gap.SerializeRemainingGap())
}

func TestBuildBlobExcerptDiffSections_HighlightStopsAtOversizedLine(t *testing.T) {
	data := &bytes.Buffer{}
	data.WriteString("a = 1\n")
	data.WriteString("b = \"" + strings.Repeat("x", MaxFullFileHighlightSizeLimit) + "\"\n")
	data.WriteString("c = 3\n")
	sections, err := BuildBlobExcerptDiffSections("a.py", bytes.NewReader(data.Bytes()), []BlobExcerptOptions{{RightIndex: 3, LeftIndex: 3}})
	require.NoError(t, err)
	highlighted := sections[0].highlightedRightLines.value
	assert.Contains(t, string(highlighted[0]), ">a<")
	assert.NotContains(t, highlighted, 2) // the line after the oversized one must not take another line's highlighting
}

func TestCanExpandAllHiddenLines(t *testing.T) {
	gapLine := func(info DiffLineSectionInfo) *DiffLine {
		return &DiffLine{Type: DiffLineSection, SectionInfo: &info}
	}
	file := &DiffFile{Sections: []*DiffSection{
		{Lines: []*DiffLine{gapLine(DiffLineSectionInfo{LeftIdx: 17, RightIdx: 17, LeftHunkSize: 7, RightHunkSize: 7})}},     // lines 1-16
		{Lines: []*DiffLine{gapLine(DiffLineSectionInfo{LastLeftIdx: 103, LastRightIdx: 103, LeftIdx: 120, RightIdx: 120})}}, // lines 104-120
	}}
	defer test.MockVariableValue(&setting.Git.MaxGitDiffLines, 33)()
	assert.True(t, file.CanExpandAllHiddenLines())
	setting.Git.MaxGitDiffLines = 32
	assert.False(t, file.CanExpandAllHiddenLines())
	assert.False(t, (&DiffFile{}).CanExpandAllHiddenLines())
}
