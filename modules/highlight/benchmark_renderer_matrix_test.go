// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"html/template"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/odvcencio/gotreesitter"
	tsgrammars "github.com/odvcencio/gotreesitter/grammars"
)

var (
	benchmarkPythonSnippet = []byte(strings.Repeat(`def fib(n):
    if n < 2:
        return n
    return fib(n - 1) + fib(n - 2)
`, 700))

	benchmarkJavaScriptSnippet = []byte(strings.Repeat(`export function sum(values) {
  return values.reduce((acc, v) => acc + v, 0);
}
`, 700))

	benchmarkSQLLargeSnippet = []byte(strings.Repeat(`SELECT u.id, u.name, o.total
FROM users u
JOIN orders o ON o.user_id = u.id
WHERE o.total > 100
ORDER BY o.created_unix DESC;
`, 4000))
)

type highlightBenchmarkCase struct {
	name     string
	fileName string
	fileLang string
	code     []byte
}

var highlightBenchmarkCases = []highlightBenchmarkCase{
	{name: "go_medium", fileName: "bench.go", fileLang: "Go", code: benchmarkGoSnippet},
	{name: "python_medium", fileName: "bench.py", fileLang: "Python", code: benchmarkPythonSnippet},
	{name: "javascript_medium", fileName: "bench.js", fileLang: "JavaScript", code: benchmarkJavaScriptSnippet},
	{name: "sql_xlarge", fileName: "bench.sql", fileLang: "SQL", code: benchmarkSQLLargeSnippet},
}

func resetTreeSitterRendererCacheForBench() {
	treeSitterRendererCache.Range(func(_, v any) bool {
		renderer, ok := v.(*treeSitterRenderer)
		if !ok || renderer == nil {
			return true
		}
		renderer.mu.Lock()
		renderer.cache.setTree(nil)
		renderer.cache = treeSitterRenderCache{}
		renderer.mu.Unlock()
		return true
	})
	treeSitterRendererCache = sync.Map{}
}

func BenchmarkHighlightRendererMatrix(b *testing.B) {
	for _, tc := range highlightBenchmarkCases {
		tc := tc
		b.Run(tc.name+"/render_code", func(b *testing.B) {
			b.Run("treesitter", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(tc.code)))
				if _, _, ok := tryRenderCodeByTreeSitter(tc.fileName, tc.fileLang, tc.code, false, true); !ok {
					b.Skip("tree-sitter renderer is unavailable")
				}
				for b.Loop() {
					if _, _, ok := tryRenderCodeByTreeSitter(tc.fileName, tc.fileLang, tc.code, false, true); !ok {
						b.Fatal("tree-sitter renderer became unavailable")
					}
				}
			})

			b.Run("chroma", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(tc.code)))
				code := string(tc.code)
				lexer := DetectChromaLexerByFileName(tc.fileName, tc.fileLang)
				for b.Loop() {
					_ = renderCodeByChromaLexer(lexer, code)
				}
			})
		})

		b.Run(tc.name+"/render_full_file", func(b *testing.B) {
			b.Run("treesitter", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(tc.code)))
				if _, _, err := renderFullFileByTreeSitter(tc.fileName, tc.fileLang, tc.code); err != nil {
					b.Skipf("tree-sitter renderer is unavailable: %v", err)
				}
				for b.Loop() {
					if _, _, err := renderFullFileByTreeSitter(tc.fileName, tc.fileLang, tc.code); err != nil {
						b.Fatalf("tree-sitter renderer became unavailable: %v", err)
					}
				}
			})

			b.Run("chroma", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(tc.code)))
				for b.Loop() {
					_, _, err := renderFullFileByChroma(tc.fileName, tc.fileLang, tc.code)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

func BenchmarkHighlightRendererParallelGo(b *testing.B) {
	code := benchmarkGoSnippet
	codeStr := string(code)
	fileName := "bench.go"
	fileLang := "Go"

	b.Run("treesitter", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(code)))
		if _, _, ok := tryRenderCodeByTreeSitter(fileName, fileLang, code, false, true); !ok {
			b.Skip("tree-sitter renderer is unavailable")
		}
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if _, _, ok := tryRenderCodeByTreeSitter(fileName, fileLang, code, false, true); !ok {
					b.Fatal("tree-sitter renderer became unavailable")
				}
			}
		})
	})

	b.Run("chroma", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(code)))
		b.RunParallel(func(pb *testing.PB) {
			lexer := DetectChromaLexerByFileName(fileName, fileLang)
			for pb.Next() {
				_ = renderCodeByChromaLexer(lexer, codeStr)
			}
		})
	})
}

func BenchmarkHighlightRendererColdStartGo(b *testing.B) {
	code := benchmarkGoSnippet
	codeStr := string(code)
	fileName := "bench.go"
	fileLang := "Go"

	b.Run("treesitter", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(code)))
		for b.Loop() {
			resetTreeSitterRendererCacheForBench()
			if _, _, ok := tryRenderCodeByTreeSitter(fileName, fileLang, code, false, true); !ok {
				b.Fatal("tree-sitter renderer is unavailable")
			}
		}
	})

	b.Run("chroma", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(code)))
		for b.Loop() {
			lexer := DetectChromaLexerByFileName(fileName, fileLang)
			_ = renderCodeByChromaLexer(lexer, codeStr)
		}
	})
}

func BenchmarkHighlightRendererColdStartGoPipeline(b *testing.B) {
	code := benchmarkGoSnippet
	fileName := "bench.go"
	fileLang := "Go"

	entry := resolveTreeSitterEntry(fileName, fileLang)
	if entry == nil {
		b.Skip("tree-sitter renderer is unavailable for Go")
	}

	lang := entry.Language()
	if lang == nil {
		b.Skip("language unavailable")
	}

	// Structural counters (single setup parse) to sanity-check that timing wins
	// come from faster execution, not reduced work.
	preParser := gotreesitter.NewParser(lang)
	preTree, err := parseHighlightBenchTree(preParser, entry, lang, code)
	if err != nil {
		b.Fatalf("pre-parse failed: %v", err)
	}
	preRoot := preTree.RootNode()
	if preRoot == nil || preRoot.EndByte() != uint32(len(code)) {
		rt := preTree.ParseRuntime()
		b.Fatalf("pre-parse truncated: root=%v summary=%s", preRoot, rt.Summary())
	}
	preQuery, err := gotreesitter.NewQuery(entry.HighlightQuery, lang)
	if err != nil {
		preTree.Release()
		b.Fatalf("pre-query compile failed: %v", err)
	}
	preMatches := preQuery.Execute(preTree)
	preSpanCount, preRenderedLen := renderHighlightMatchesForBench(code, preMatches)
	preCaptureCount := 0
	for i := range preMatches {
		preCaptureCount += len(preMatches[i].Captures)
	}
	preNodeCount := 0
	gotreesitter.Walk(preRoot, func(_ *gotreesitter.Node, _ int) gotreesitter.WalkAction {
		preNodeCount++
		return gotreesitter.WalkContinue
	})
	preTree.Release()

	var nsTotal int64
	var nsLanguageLoad int64
	var nsParserInit int64
	var nsQueryInit int64
	var nsParse int64
	var nsQueryRender int64

	b.ReportAllocs()
	b.SetBytes(int64(len(code)))
	for b.Loop() {
		iterStart := time.Now()

		phaseStart := time.Now()
		// Force cold semantics each iteration.
		tsgrammars.PurgeEmbeddedLanguageCache()
		resetTreeSitterRendererCacheForBench()
		iterEntry := resolveTreeSitterEntry(fileName, fileLang)
		if iterEntry == nil {
			b.Fatal("tree-sitter entry missing")
		}
		iterLang := iterEntry.Language()
		if iterLang == nil {
			b.Fatal("language unavailable")
		}
		nsLanguageLoad += time.Since(phaseStart).Nanoseconds()

		phaseStart = time.Now()
		parser := gotreesitter.NewParser(iterLang)
		nsParserInit += time.Since(phaseStart).Nanoseconds()

		phaseStart = time.Now()
		query, err := gotreesitter.NewQuery(iterEntry.HighlightQuery, iterLang)
		if err != nil {
			b.Fatalf("query compile failed: %v", err)
		}
		nsQueryInit += time.Since(phaseStart).Nanoseconds()

		phaseStart = time.Now()
		tree, err := parseHighlightBenchTree(parser, iterEntry, iterLang, code)
		if err != nil {
			b.Fatalf("parse failed: %v", err)
		}
		root := tree.RootNode()
		if root == nil {
			tree.Release()
			b.Fatal("parse returned nil root")
		}
		if got, want := root.EndByte(), uint32(len(code)); got != want {
			rt := tree.ParseRuntime()
			tree.Release()
			b.Fatalf("parse truncated: end=%d want=%d %s", got, want, rt.Summary())
		}
		rt := tree.ParseRuntime()
		if rt.Truncated || rt.StopReason != gotreesitter.ParseStopAccepted {
			tree.Release()
			b.Fatalf("parse runtime invalid: %s", rt.Summary())
		}
		nsParse += time.Since(phaseStart).Nanoseconds()

		phaseStart = time.Now()
		matches := query.Execute(tree)
		if len(matches) == 0 {
			tree.Release()
			b.Fatal("query returned no matches")
		}
		spanCount, renderedLen := renderHighlightMatchesForBench(code, matches)
		tree.Release()
		if spanCount == 0 || renderedLen == 0 {
			b.Fatal("render produced empty output")
		}
		nsQueryRender += time.Since(phaseStart).Nanoseconds()

		nsTotal += time.Since(iterStart).Nanoseconds()
	}

	phaseSum := nsLanguageLoad + nsParserInit + nsQueryInit + nsParse + nsQueryRender
	unattributed := nsTotal - phaseSum
	b.ReportMetric(float64(nsTotal)/float64(b.N), "cold_total-ns/op")
	b.ReportMetric(float64(nsLanguageLoad)/float64(b.N), "language_load-ns/op")
	b.ReportMetric(float64(nsParserInit)/float64(b.N), "parser_init-ns/op")
	b.ReportMetric(float64(nsQueryInit)/float64(b.N), "query_init-ns/op")
	b.ReportMetric(float64(nsParse)/float64(b.N), "parse-ns/op")
	b.ReportMetric(float64(nsQueryRender)/float64(b.N), "query_render-ns/op")
	b.ReportMetric(float64(phaseSum)/float64(b.N), "phase_sum-ns/op")
	b.ReportMetric(float64(unattributed)/float64(b.N), "unattributed-ns/op")
	b.ReportMetric(float64(len(code)), "source_bytes")
	b.ReportMetric(float64(preNodeCount), "tree_nodes")
	b.ReportMetric(float64(preCaptureCount), "captures")
	b.ReportMetric(float64(preSpanCount), "render_spans")
	b.ReportMetric(float64(preRenderedLen), "render_bytes")
	b.ReportMetric(float64(preQuery.PatternCount()), "query_patterns")
}

func BenchmarkHighlightQueryRenderGo(b *testing.B) {
	code := benchmarkGoSnippet
	fileName := "bench.go"
	fileLang := "Go"

	entry := resolveTreeSitterEntry(fileName, fileLang)
	if entry == nil {
		b.Skip("tree-sitter entry unavailable")
	}
	lang := entry.Language()
	if lang == nil {
		b.Skip("language unavailable")
	}

	parser := gotreesitter.NewParser(lang)
	tree, err := parseHighlightBenchTree(parser, entry, lang, code)
	if err != nil {
		b.Fatalf("parse failed: %v", err)
	}
	defer tree.Release()
	root := tree.RootNode()
	if root == nil || root.EndByte() != uint32(len(code)) {
		rt := tree.ParseRuntime()
		b.Fatalf("parse truncated: root=%v summary=%s", root, rt.Summary())
	}

	query, err := gotreesitter.NewQuery(entry.HighlightQuery, lang)
	if err != nil {
		b.Fatalf("query compile failed: %v", err)
	}

	b.Run("execute_plus_render", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(code)))
		for b.Loop() {
			matches := query.Execute(tree)
			if len(matches) == 0 {
				b.Fatal("query returned no matches")
			}
			spanCount, renderedLen := renderHighlightMatchesForBench(code, matches)
			if spanCount == 0 || renderedLen == 0 {
				b.Fatal("render produced empty output")
			}
		}
	})

	b.Run("cursor_plus_render", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(code)))
		for b.Loop() {
			spanCount, renderedLen := renderHighlightCursorForBench(query, tree, lang, code)
			if spanCount == 0 || renderedLen == 0 {
				b.Fatal("render produced empty output")
			}
		}
	})
}

func parseHighlightBenchTree(parser *gotreesitter.Parser, entry *tsgrammars.LangEntry, lang *gotreesitter.Language, code []byte) (*gotreesitter.Tree, error) {
	if entry.TokenSourceFactory != nil {
		ts := entry.TokenSourceFactory(code, lang)
		return parser.ParseWithTokenSource(code, ts)
	}
	return parser.Parse(code)
}

func renderHighlightMatchesForBench(code []byte, matches []gotreesitter.QueryMatch) (spanCount int, renderedLen int) {
	if len(matches) == 0 {
		return 0, 0
	}

	ranges := make([]gotreesitter.HighlightRange, 0, len(matches)*2)
	for i := range matches {
		for _, c := range matches[i].Captures {
			node := c.Node
			if node == nil || node.StartByte() == node.EndByte() {
				continue
			}
			ranges = append(ranges, gotreesitter.HighlightRange{
				StartByte: node.StartByte(),
				EndByte:   node.EndByte(),
				Capture:   c.Name,
			})
		}
	}
	if len(ranges) == 0 {
		return 0, 0
	}

	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].StartByte != ranges[j].StartByte {
			return ranges[i].StartByte < ranges[j].StartByte
		}
		wi := ranges[i].EndByte - ranges[i].StartByte
		wj := ranges[j].EndByte - ranges[j].StartByte
		return wi > wj
	})
	normalized := normalizeHighlightRanges(len(code), ranges)

	var out strings.Builder
	out.Grow(len(code) + len(normalized)*24)
	last := 0
	for _, hr := range normalized {
		start := hr.start
		end := hr.end
		if start > last {
			template.HTMLEscape(&out, code[last:start])
		}
		if hr.class == "" {
			template.HTMLEscape(&out, code[start:end])
		} else {
			out.WriteString(`<span class="`)
			out.WriteString(hr.class)
			out.WriteString(`">`)
			template.HTMLEscape(&out, code[start:end])
			out.WriteString(`</span>`)
		}
		last = end
	}
	if last < len(code) {
		template.HTMLEscape(&out, code[last:])
	}
	return len(normalized), out.Len()
}

func renderHighlightCursorForBench(query *gotreesitter.Query, tree *gotreesitter.Tree, lang *gotreesitter.Language, code []byte) (spanCount int, renderedLen int) {
	if query == nil || tree == nil || lang == nil {
		return 0, 0
	}
	root := tree.RootNode()
	if root == nil {
		return 0, 0
	}

	cursor := query.Exec(root, lang, code)
	ranges := make([]gotreesitter.HighlightRange, 0, 1024)
	for {
		c, ok := cursor.NextCapture()
		if !ok {
			break
		}
		node := c.Node
		if node == nil || node.StartByte() == node.EndByte() {
			continue
		}
		ranges = append(ranges, gotreesitter.HighlightRange{
			StartByte: node.StartByte(),
			EndByte:   node.EndByte(),
			Capture:   c.Name,
		})
	}
	if len(ranges) == 0 {
		return 0, 0
	}

	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].StartByte != ranges[j].StartByte {
			return ranges[i].StartByte < ranges[j].StartByte
		}
		wi := ranges[i].EndByte - ranges[i].StartByte
		wj := ranges[j].EndByte - ranges[j].StartByte
		return wi > wj
	})
	normalized := normalizeHighlightRanges(len(code), ranges)

	var out strings.Builder
	out.Grow(len(code) + len(normalized)*24)
	last := 0
	for _, hr := range normalized {
		start := hr.start
		end := hr.end
		if start > last {
			template.HTMLEscape(&out, code[last:start])
		}
		if hr.class == "" {
			template.HTMLEscape(&out, code[start:end])
		} else {
			out.WriteString(`<span class="`)
			out.WriteString(hr.class)
			out.WriteString(`">`)
			template.HTMLEscape(&out, code[start:end])
			out.WriteString(`</span>`)
		}
		last = end
	}
	if last < len(code) {
		template.HTMLEscape(&out, code[last:])
	}
	return len(normalized), out.Len()
}
