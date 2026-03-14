// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
)

type highlightOperation uint8

const (
	highlightOpRenderCode highlightOperation = iota
	highlightOpRenderCodeByLexer
	highlightOpRenderCodeSlowGuess
	highlightOpRenderFullFile
	highlightOperationCount
)

var highlightOperationLabels = [...]string{
	"render_code",
	"render_code_by_lexer",
	"render_code_slow_guess",
	"render_full_file",
}

type highlightRenderer uint8

const (
	highlightRendererTreeSitter highlightRenderer = iota
	highlightRendererChroma
	highlightRendererPlaintext
	highlightRendererCount
)

var highlightRendererLabels = [...]string{
	"treesitter",
	"chroma",
	"plaintext",
}

type highlightFallbackReason uint8

const (
	highlightFallbackEntryUnavailable highlightFallbackReason = iota
	highlightFallbackRendererUnavailable
	highlightFallbackRenderUnusable
	highlightFallbackLexerUnavailable
	highlightFallbackCount
)

var highlightFallbackReasonLabels = [...]string{
	"entry_unavailable",
	"renderer_unavailable",
	"render_unusable",
	"lexer_unavailable",
}

type highlightMetricsStore struct {
	renders   [highlightOperationCount][highlightRendererCount]atomic.Uint64
	fallbacks [highlightOperationCount][highlightFallbackCount]atomic.Uint64
}

type highlightMetricsSnapshot struct {
	Renders   [highlightOperationCount][highlightRendererCount]uint64
	Fallbacks [highlightOperationCount][highlightFallbackCount]uint64
}

var highlightMetrics highlightMetricsStore

var (
	highlightRendersDesc = prometheus.NewDesc(
		"gitea_highlight_renders_total",
		"Number of highlight renders by backend.",
		[]string{"operation", "renderer"},
		nil,
	)
	highlightFallbacksDesc = prometheus.NewDesc(
		"gitea_highlight_fallbacks_total",
		"Number of gotreesitter highlight fallbacks by operation and reason.",
		[]string{"operation", "reason"},
		nil,
	)
)

type highlightMetricsCollector struct{}

func init() {
	prometheus.MustRegister(highlightMetricsCollector{})
}

func (highlightMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- highlightRendersDesc
	ch <- highlightFallbacksDesc
}

func (highlightMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	snapshot := snapshotHighlightMetrics()
	for opIdx, opLabel := range highlightOperationLabels {
		for rendererIdx, rendererLabel := range highlightRendererLabels {
			ch <- prometheus.MustNewConstMetric(
				highlightRendersDesc,
				prometheus.CounterValue,
				float64(snapshot.Renders[opIdx][rendererIdx]),
				opLabel,
				rendererLabel,
			)
		}
		for reasonIdx, reasonLabel := range highlightFallbackReasonLabels {
			ch <- prometheus.MustNewConstMetric(
				highlightFallbacksDesc,
				prometheus.CounterValue,
				float64(snapshot.Fallbacks[opIdx][reasonIdx]),
				opLabel,
				reasonLabel,
			)
		}
	}
}

func recordHighlightRender(op highlightOperation, renderer highlightRenderer) {
	highlightMetrics.renders[op][renderer].Add(1)
}

func recordHighlightFallback(op highlightOperation, reason highlightFallbackReason) {
	highlightMetrics.fallbacks[op][reason].Add(1)
}

func snapshotHighlightMetrics() highlightMetricsSnapshot {
	var snapshot highlightMetricsSnapshot
	for opIdx := range highlightOperationLabels {
		for rendererIdx := range highlightRendererLabels {
			snapshot.Renders[opIdx][rendererIdx] = highlightMetrics.renders[opIdx][rendererIdx].Load()
		}
		for reasonIdx := range highlightFallbackReasonLabels {
			snapshot.Fallbacks[opIdx][reasonIdx] = highlightMetrics.fallbacks[opIdx][reasonIdx].Load()
		}
	}
	return snapshot
}

func resetHighlightMetricsForTesting() {
	for opIdx := range highlightOperationLabels {
		for rendererIdx := range highlightRendererLabels {
			highlightMetrics.renders[opIdx][rendererIdx].Store(0)
		}
		for reasonIdx := range highlightFallbackReasonLabels {
			highlightMetrics.fallbacks[opIdx][reasonIdx].Store(0)
		}
	}
}
