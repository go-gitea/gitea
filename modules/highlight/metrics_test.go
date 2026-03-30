// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func sumHighlightRenderCounts(snapshot highlightMetricsSnapshot, op highlightOperation) uint64 {
	var total uint64
	for rendererIdx := range highlightRendererLabels {
		total += snapshot.Renders[op][rendererIdx]
	}
	return total
}

func sumHighlightFallbackCounts(snapshot highlightMetricsSnapshot, op highlightOperation) uint64 {
	var total uint64
	for reasonIdx := range highlightFallbackReasonLabels {
		total += snapshot.Fallbacks[op][reasonIdx]
	}
	return total
}

func TestRepresentativeSamplesAvoidChromaFallbacks(t *testing.T) {
	resetHighlightMetricsForTesting()
	t.Cleanup(resetHighlightMetricsForTesting)

	for _, tc := range visualParityTop50 {
		rendered := RenderCode(tc.fileName, "", tc.code)
		assert.NotEmpty(t, rendered, tc.name)

		lines, _, err := RenderFullFile(tc.fileName, "", []byte(tc.code))
		assert.NoError(t, err, tc.name)
		assert.NotEmpty(t, lines, tc.name)
	}

	snapshot := snapshotHighlightMetrics()
	expectedTreeSitter := uint64(len(visualParityStableTreeSitterSamples()))
	expectedFallbacks := uint64(len(visualParityKnownTreeSitterFallbacks))

	assert.Equal(t, expectedTreeSitter, snapshot.Renders[highlightOpRenderCode][highlightRendererTreeSitter])
	assert.Equal(t, expectedFallbacks, snapshot.Renders[highlightOpRenderCode][highlightRendererChroma])
	assert.Zero(t, snapshot.Renders[highlightOpRenderCode][highlightRendererPlaintext])
	assert.Equal(t, expectedFallbacks, sumHighlightFallbackCounts(snapshot, highlightOpRenderCode))
	assert.Equal(t, expectedFallbacks, snapshot.Fallbacks[highlightOpRenderCode][highlightFallbackRenderUnusable])

	assert.Equal(t, expectedTreeSitter, snapshot.Renders[highlightOpRenderFullFile][highlightRendererTreeSitter])
	assert.Equal(t, expectedFallbacks, snapshot.Renders[highlightOpRenderFullFile][highlightRendererChroma])
	assert.Zero(t, snapshot.Renders[highlightOpRenderFullFile][highlightRendererPlaintext])
	assert.Equal(t, expectedFallbacks, sumHighlightFallbackCounts(snapshot, highlightOpRenderFullFile))
	assert.Equal(t, expectedFallbacks, snapshot.Fallbacks[highlightOpRenderFullFile][highlightFallbackRenderUnusable])
}

func TestRenderCodeFallbackMetrics(t *testing.T) {
	resetHighlightMetricsForTesting()
	t.Cleanup(resetHighlightMetricsForTesting)

	fileName := "bench.go"
	fileLang := "Go"
	code := "package main\nfunc main() { println(1) }\n"

	withForcedNilTreeSitterRenderer(t, fileName, fileLang, func() {
		got := RenderCode(fileName, fileLang, code)
		assert.NotEmpty(t, got)
	})

	snapshot := snapshotHighlightMetrics()
	assert.EqualValues(t, 1, snapshot.Renders[highlightOpRenderCode][highlightRendererChroma])
	assert.Zero(t, snapshot.Renders[highlightOpRenderCode][highlightRendererTreeSitter])
	assert.EqualValues(t, 1, snapshot.Fallbacks[highlightOpRenderCode][highlightFallbackRendererUnavailable])
	assert.EqualValues(t, 1, sumHighlightRenderCounts(snapshot, highlightOpRenderCode))
}
