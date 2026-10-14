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

	diffInline := diffSection.GetComputedInlineDiffFor(diffSection.Lines[1], locale)
	assert.Equal(t, `<span class="n">a</span> <span class="o">=</span> <span class="mi">30</span>`+"\n", string(diffInline.Content))
}

func TestBuildBlobExcerptDiffSection_DirectionAll(t *testing.T) {
	data := &bytes.Buffer{}
	for i := range 100 {
		data.WriteString("a = " + strconv.Itoa(i+1) + "\n")
	}

	tests := []struct {
		name          string
		opts          BlobExcerptOptions
		firstLeft     int
		firstRight    int
		lastLineLeft  int
		lastLineRight int
	}{
		{
			// leading gap: before the first hunk, so LastLeft/LastRight are both 0
			name: "leading gap",
			opts: BlobExcerptOptions{
				LastLeft: 0, LastRight: 0,
				LeftIndex: 50, RightIndex: 50,
				LeftHunkSize: 10, RightHunkSize: 10,
				Direction: "all",
			},
			firstLeft: 1, firstRight: 1,
			lastLineLeft: 49, lastLineRight: 49,
		},
		{
			// middle gap: between two hunks, hunk sizes are both > 0
			name: "middle gap",
			opts: BlobExcerptOptions{
				LastLeft: 10, LastRight: 10,
				LeftIndex: 90, RightIndex: 90,
				LeftHunkSize: 10, RightHunkSize: 10,
				Direction: "all",
			},
			firstLeft: 11, firstRight: 11,
			lastLineLeft: 89, lastLineRight: 89,
		},
		{
			// end-of-file gap: no following hunk, so hunk sizes are both 0, and the last line is inclusive
			name: "end-of-file gap",
			opts: BlobExcerptOptions{
				LastLeft: 40, LastRight: 40,
				LeftIndex: 100, RightIndex: 100,
				LeftHunkSize: 0, RightHunkSize: 0,
				Direction: "all",
			},
			firstLeft: 41, firstRight: 41,
			lastLineLeft: 100, lastLineRight: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diffSection, err := BuildBlobExcerptDiffSection("a.py", bytes.NewReader(data.Bytes()), tt.opts)
			require.NoError(t, err)
			require.NotEmpty(t, diffSection.Lines)

			firstLine := diffSection.Lines[0]
			lastLine := diffSection.Lines[len(diffSection.Lines)-1]
			assert.Equal(t, tt.firstLeft, firstLine.LeftIdx)
			assert.Equal(t, tt.firstRight, firstLine.RightIdx)
			assert.Equal(t, tt.lastLineLeft, lastLine.LeftIdx)
			assert.Equal(t, tt.lastLineRight, lastLine.RightIdx)

			for _, line := range diffSection.Lines {
				assert.NotEqual(t, DiffLineSection, line.Type, "a full-gap expansion must not emit a new expander row")
			}

			assert.NotEmpty(t, diffSection.highlightedRightLines.value[tt.firstRight-1])
			assert.NotEmpty(t, diffSection.highlightedRightLines.value[tt.lastLineRight-1])
		})
	}
}
