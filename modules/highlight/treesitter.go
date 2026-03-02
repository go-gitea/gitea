// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"bytes"
	"errors"
	"html/template"
	"path"
	"sort"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/analyze"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	"github.com/odvcencio/gotreesitter"
	tsgrammars "github.com/odvcencio/gotreesitter/grammars"
)

type treeSitterRegistryType struct {
	byCanonicalName map[string]*tsgrammars.LangEntry
}

type treeSitterRenderer struct {
	displayName       string
	lang              *gotreesitter.Language
	parser            *gotreesitter.Parser
	query             *gotreesitter.Query
	tokenSourceFactor func([]byte) gotreesitter.TokenSource
	captureClassCache map[string]string
	mu                sync.Mutex
	cache             treeSitterRenderCache
}

type treeSitterRenderCache struct {
	source []byte

	code template.HTML
	// trim tracks whether code cache was produced with trailing newline trim.
	trim bool

	lines  []template.HTML
	ranges []normalizedHighlightRange
	tree   *gotreesitter.Tree

	altSource []byte
	altCode   template.HTML
	altTrim   bool
	altLines  []template.HTML
}

type normalizedHighlightRange struct {
	start int
	end   int
	class string
}

var (
	treeSitterRegistry = sync.OnceValue(func() *treeSitterRegistryType {
		registry := &treeSitterRegistryType{
			byCanonicalName: map[string]*tsgrammars.LangEntry{},
		}

		languages := tsgrammars.AllLanguages()
		for i := range languages {
			entry := &languages[i]
			if strings.TrimSpace(entry.HighlightQuery) == "" {
				continue
			}
			registry.byCanonicalName[canonicalLanguageKey(entry.Name)] = entry
		}

		for alias, canonical := range treeSitterLanguageAliases {
			entry, ok := registry.byCanonicalName[canonical]
			if ok {
				registry.byCanonicalName[alias] = entry
			}
		}
		return registry
	})

	treeSitterRendererCache sync.Map // map[string]*treeSitterRenderer
)

var treeSitterLanguageAliases = map[string]string{
	"csharp":          "csharp",
	"cpp":             "cpp",
	"golang":          "go",
	"javascriptreact": "javascript",
	"makefile":        "make",
	"objectivec":      "objc",
	"objc":            "objc",
	"plaintext":       "",
	"shell":           "bash",
	"shellscript":     "bash",
	"text":            "",
	"typescriptreact": "tsx",
}

func canonicalLanguageKey(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"+", "p",
		"#", "sharp",
		" ", "",
		"-", "",
		"_", "",
		".", "",
		"/", "",
	)
	return replacer.Replace(name)
}

func lookupTreeSitterEntryByLanguageName(name string) *tsgrammars.LangEntry {
	key := canonicalLanguageKey(name)
	if key == "" {
		return nil
	}
	return treeSitterRegistry().byCanonicalName[key]
}

func treeSitterDisplayName(entry *tsgrammars.LangEntry) string {
	if entry == nil {
		return ""
	}

	switch entry.Name {
	case "css":
		return "CSS"
	case "c_sharp":
		return "C#"
	case "cpp":
		return "C++"
	case "html":
		return "HTML"
	case "json":
		return "JSON"
	case "javascript":
		return "JavaScript"
	case "php":
		return "PHP"
	case "objc":
		return "Objective-C"
	case "sql":
		return "SQL"
	case "toml":
		return "TOML"
	case "typescript":
		return "TypeScript"
	case "xml":
		return "XML"
	case "yaml":
		return "YAML"
	default:
		return util.ToTitleCaseNoLower(strings.ReplaceAll(entry.Name, "_", " "))
	}
}

func resolveTreeSitterEntry(fileName, fileLang string) *tsgrammars.LangEntry {
	fileName, fileLang = normalizeFileNameLang(fileName, fileLang)
	fileExt := path.Ext(fileName)

	if fileExt != "" {
		if val, ok := globalVars().highlightMapping[fileExt]; ok {
			if strings.HasPrefix(val, ".") {
				fileName = "dummy" + val
				fileLang = ""
			} else {
				fileLang = val
			}
		}
	}

	// Prefer filename detection first. This avoids expensive name-resolution
	// initialization on the hot path when extension detection already succeeds.
	var entry *tsgrammars.LangEntry
	if fileName != "" {
		entry = tsgrammars.DetectLanguage(fileName)
	}
	if entry == nil && fileLang != "" {
		entry = lookupTreeSitterEntryByLanguageName(fileLang)
	}
	return entry
}

func resolveTreeSitterEntryWithAnalyze(fileName, fileLang string, code []byte) *tsgrammars.LangEntry {
	entry := resolveTreeSitterEntry(fileName, fileLang)
	if entry != nil || len(code) == 0 {
		return entry
	}

	analyzedLanguage := analyze.GetCodeLanguage(fileName, code)
	return lookupTreeSitterEntryByLanguageName(analyzedLanguage)
}

func getTreeSitterRenderer(entry *tsgrammars.LangEntry) *treeSitterRenderer {
	if entry == nil || strings.TrimSpace(entry.HighlightQuery) == "" {
		return nil
	}

	if val, ok := treeSitterRendererCache.Load(entry.Name); ok {
		renderer, _ := val.(*treeSitterRenderer)
		return renderer
	}

	lang := entry.Language()
	if lang == nil {
		treeSitterRendererCache.Store(entry.Name, (*treeSitterRenderer)(nil))
		return nil
	}

	query, err := gotreesitter.NewQuery(entry.HighlightQuery, lang)
	if err != nil {
		log.Warn("failed to compile gotreesitter highlight query for %s: %v", entry.Name, err)
		treeSitterRendererCache.Store(entry.Name, (*treeSitterRenderer)(nil))
		return nil
	}

	var tokenSourceFactory func([]byte) gotreesitter.TokenSource
	if entry.TokenSourceFactory != nil {
		tokenSourceFactory = func(src []byte) gotreesitter.TokenSource {
			return entry.TokenSourceFactory(src, lang)
		}
	}

	renderer := &treeSitterRenderer{
		displayName:       treeSitterDisplayName(entry),
		lang:              lang,
		parser:            gotreesitter.NewParser(lang),
		query:             query,
		tokenSourceFactor: tokenSourceFactory,
		captureClassCache: make(map[string]string, 32),
	}
	val, _ := treeSitterRendererCache.LoadOrStore(entry.Name, renderer)
	resolvedRenderer, _ := val.(*treeSitterRenderer)
	return resolvedRenderer
}

func (renderer *treeSitterRenderer) render(code []byte, trimTrailingNewline bool) (template.HTML, bool) {
	if renderer == nil || renderer.parser == nil || renderer.query == nil || renderer.lang == nil {
		return "", false
	}

	defer func() {
		if err := recover(); err != nil {
			log.Warn("panic while rendering with gotreesitter: %v\n%s", err, log.Stack(2))
		}
	}()

	renderer.mu.Lock()
	if renderer.cache.matches(code) && renderer.cache.code != "" && renderer.cache.trim == trimTrailingNewline {
		cached := renderer.cache.code
		renderer.mu.Unlock()
		return cached, true
	}
	if renderer.cache.matchesAlt(code) && renderer.cache.altCode != "" && renderer.cache.altTrim == trimTrailingNewline {
		cached := renderer.cache.altCode
		renderer.mu.Unlock()
		return cached, true
	}
	ranges := renderer.highlightRangesLocked(code)
	renderer.mu.Unlock()

	var out strings.Builder
	out.Grow(len(code) + len(ranges)*32)

	last := 0
	for _, hr := range ranges {
		start := hr.start
		end := hr.end

		if start < last {
			start = last
		}
		if start > len(code) {
			break
		}
		if end > len(code) {
			end = len(code)
		}
		if end <= start {
			continue
		}

		if start > last {
			template.HTMLEscape(&out, code[last:start])
		}

		className := hr.class
		if className == "" {
			template.HTMLEscape(&out, code[start:end])
		} else {
			out.WriteString(`<span class="`)
			out.WriteString(className)
			out.WriteString(`">`)
			template.HTMLEscape(&out, code[start:end])
			out.WriteString(`</span>`)
		}
		last = end
	}

	if last < len(code) {
		template.HTMLEscape(&out, code[last:])
	}

	content := out.String()
	if trimTrailingNewline {
		content = strings.TrimSuffix(content, "\n")
	}
	rendered := template.HTML(content)

	renderer.mu.Lock()
	renderer.cache.code = rendered
	renderer.cache.trim = trimTrailingNewline
	renderer.mu.Unlock()

	return rendered, true
}

func (renderer *treeSitterRenderer) renderLines(code []byte) ([]template.HTML, bool) {
	if renderer == nil || renderer.parser == nil || renderer.query == nil || renderer.lang == nil {
		return nil, false
	}

	defer func() {
		if err := recover(); err != nil {
			log.Warn("panic while rendering lines with gotreesitter: %v\n%s", err, log.Stack(2))
		}
	}()

	renderer.mu.Lock()
	if renderer.cache.matches(code) && renderer.cache.lines != nil {
		out := append([]template.HTML(nil), renderer.cache.lines...)
		renderer.mu.Unlock()
		return out, true
	}
	if renderer.cache.matchesAlt(code) && renderer.cache.altLines != nil {
		out := append([]template.HTML(nil), renderer.cache.altLines...)
		renderer.mu.Unlock()
		return out, true
	}
	ranges := renderer.highlightRangesLocked(code)
	renderer.mu.Unlock()

	lines := make([]template.HTML, 0, bytes.Count(code, []byte("\n"))+1)
	var line strings.Builder
	line.Grow(len(code) / 8)

	flushLine := func() {
		lines = append(lines, template.HTML(line.String()))
		line.Reset()
	}

	appendSegment := func(seg []byte, className string) {
		for len(seg) > 0 {
			nl := bytes.IndexByte(seg, '\n')
			part := seg
			hasNL := false
			if nl >= 0 {
				part = seg[:nl+1]
				seg = seg[nl+1:]
				hasNL = true
			} else {
				seg = nil
			}
			if len(part) > 0 {
				if className == "" {
					template.HTMLEscape(&line, part)
				} else {
					line.WriteString(`<span class="`)
					line.WriteString(className)
					line.WriteString(`">`)
					template.HTMLEscape(&line, part)
					line.WriteString(`</span>`)
				}
			}
			if hasNL {
				flushLine()
			}
		}
	}

	last := 0
	for _, hr := range ranges {
		start := hr.start
		end := hr.end

		if start > last {
			appendSegment(code[last:start], "")
		}
		appendSegment(code[start:end], hr.class)
		last = end
	}

	if last < len(code) {
		appendSegment(code[last:], "")
	}
	if line.Len() > 0 {
		flushLine()
	}

	renderer.mu.Lock()
	renderer.cache.lines = append([]template.HTML(nil), lines...)
	renderer.mu.Unlock()

	return lines, true
}

func (c *treeSitterRenderCache) matches(code []byte) bool {
	if len(c.source) != len(code) {
		return false
	}
	if len(code) == 0 {
		return true
	}
	// Fast reject: most misses differ near boundaries.
	if c.source[0] != code[0] || c.source[len(code)-1] != code[len(code)-1] {
		return false
	}
	// Additional boundary guards to avoid full slice compare on common misses.
	if len(code) > 16 {
		if !bytes.Equal(c.source[:8], code[:8]) {
			return false
		}
		if !bytes.Equal(c.source[len(code)-8:], code[len(code)-8:]) {
			return false
		}
	}
	return bytes.Equal(c.source, code)
}

func (c *treeSitterRenderCache) setSource(code []byte) {
	if cap(c.source) < len(code) {
		c.source = make([]byte, len(code))
	} else {
		c.source = c.source[:len(code)]
	}
	copy(c.source, code)
}

func (c *treeSitterRenderCache) setAltSource(code []byte) {
	if cap(c.altSource) < len(code) {
		c.altSource = make([]byte, len(code))
	} else {
		c.altSource = c.altSource[:len(code)]
	}
	copy(c.altSource, code)
}

func (c *treeSitterRenderCache) setRanges(ranges []normalizedHighlightRange) {
	c.ranges = ranges
}

func (c *treeSitterRenderCache) setTree(tree *gotreesitter.Tree) {
	if c.tree != nil && c.tree != tree {
		c.tree.Release()
	}
	c.tree = tree
}

func (c *treeSitterRenderCache) clearDerived() {
	c.code = ""
	c.trim = false
	c.lines = nil
}

func (c *treeSitterRenderCache) archiveCurrentDerivedToAlt() {
	if len(c.source) == 0 {
		c.altSource = c.altSource[:0]
		c.altCode = ""
		c.altTrim = false
		c.altLines = nil
		return
	}
	c.setAltSource(c.source)
	c.altCode = c.code
	c.altTrim = c.trim
	if c.lines != nil {
		c.altLines = append([]template.HTML(nil), c.lines...)
	} else {
		c.altLines = nil
	}
}

func (renderer *treeSitterRenderer) highlightRangesLocked(code []byte) []normalizedHighlightRange {
	if renderer.cache.matches(code) && renderer.cache.ranges != nil {
		return renderer.cache.ranges
	}

	ranges, tree := renderer.highlightIncrementalLocked(code)
	if !renderer.cache.matches(code) {
		renderer.cache.archiveCurrentDerivedToAlt()
	}
	renderer.cache.setSource(code)
	renderer.cache.setTree(tree)
	renderer.cache.setRanges(ranges)
	renderer.cache.clearDerived()
	return renderer.cache.ranges
}

func (renderer *treeSitterRenderer) highlightIncrementalLocked(code []byte) ([]normalizedHighlightRange, *gotreesitter.Tree) {
	oldTree := renderer.cache.tree
	oldSource := renderer.cache.source
	if oldTree != nil && len(oldSource) > 0 {
		if edit, ok := computeSingleInputEdit(oldSource, code); ok {
			oldTree.Edit(edit)
			newTree := renderer.parseTree(code, oldTree)
			if oldTree != newTree {
				oldTree.Release()
			}
			if newTree == nil || newTree.RootNode() == nil {
				return nil, newTree
			}
			return renderer.queryNormalizedRanges(newTree), newTree
		}
		oldTree.Release()
	}
	tree := renderer.parseTree(code, nil)
	if tree == nil || tree.RootNode() == nil {
		return nil, tree
	}
	return renderer.queryNormalizedRanges(tree), tree
}

func (renderer *treeSitterRenderer) parseTree(code []byte, oldTree *gotreesitter.Tree) *gotreesitter.Tree {
	if renderer == nil || renderer.parser == nil {
		return nil
	}

	var (
		tree *gotreesitter.Tree
		err  error
	)
	if renderer.tokenSourceFactor != nil {
		ts := renderer.tokenSourceFactor(code)
		if oldTree != nil {
			tree, err = renderer.parser.ParseIncrementalWithTokenSource(code, oldTree, ts)
		} else {
			tree, err = renderer.parser.ParseWithTokenSource(code, ts)
		}
	} else if oldTree != nil {
		tree, err = renderer.parser.ParseIncremental(code, oldTree)
	} else {
		tree, err = renderer.parser.Parse(code)
	}
	if err != nil {
		return gotreesitter.NewTree(nil, code, renderer.lang)
	}
	return tree
}

func (renderer *treeSitterRenderer) queryNormalizedRanges(tree *gotreesitter.Tree) []normalizedHighlightRange {
	if renderer == nil || renderer.query == nil || renderer.lang == nil || tree == nil {
		return nil
	}
	root := tree.RootNode()
	if root == nil {
		return nil
	}

	cursor := renderer.query.Exec(root, renderer.lang, tree.Source())
	var ranges []gotreesitter.HighlightRange
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
		return nil
	}

	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].StartByte != ranges[j].StartByte {
			return ranges[i].StartByte < ranges[j].StartByte
		}
		wi := ranges[i].EndByte - ranges[i].StartByte
		wj := ranges[j].EndByte - ranges[j].StartByte
		return wi > wj
	})
	resolved := resolveHighlightOverlaps(ranges)
	if len(resolved) == 0 {
		return nil
	}
	return renderer.normalizeResolvedRanges(len(tree.Source()), resolved)
}

func (renderer *treeSitterRenderer) normalizeResolvedRanges(codeLen int, ranges []gotreesitter.HighlightRange) []normalizedHighlightRange {
	out := make([]normalizedHighlightRange, 0, len(ranges))
	last := 0
	for _, hr := range ranges {
		start := int(hr.StartByte)
		end := int(hr.EndByte)
		if start < last {
			start = last
		}
		if start > codeLen {
			break
		}
		if end > codeLen {
			end = codeLen
		}
		if end <= start {
			continue
		}

		className := renderer.captureClass(hr.Capture)
		n := len(out)
		if n > 0 && out[n-1].class == className && out[n-1].end == start {
			out[n-1].end = end
		} else {
			out = append(out, normalizedHighlightRange{start: start, end: end, class: className})
		}
		last = end
	}
	return out
}

func (renderer *treeSitterRenderer) captureClass(capture string) string {
	if renderer.captureClassCache == nil {
		renderer.captureClassCache = make(map[string]string, 32)
	}
	if className, ok := renderer.captureClassCache[capture]; ok {
		return className
	}
	className := treeSitterCaptureToChromaClass(capture)
	renderer.captureClassCache[capture] = className
	return className
}

func resolveHighlightOverlaps(ranges []gotreesitter.HighlightRange) []gotreesitter.HighlightRange {
	if len(ranges) == 0 {
		return nil
	}

	type event struct {
		pos     uint32
		isStart bool
		idx     int
	}

	events := make([]event, 0, len(ranges)*2)
	for i := range ranges {
		events = append(events,
			event{pos: ranges[i].StartByte, isStart: true, idx: i},
			event{pos: ranges[i].EndByte, isStart: false, idx: i},
		)
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].pos != events[j].pos {
			return events[i].pos < events[j].pos
		}
		if events[i].isStart != events[j].isStart {
			return !events[i].isStart
		}
		if events[i].isStart {
			return events[i].idx < events[j].idx
		}
		return events[i].idx > events[j].idx
	})

	type stackEntry struct{ idx int }
	var stack []stackEntry
	active := make([]bool, len(ranges))
	var result []gotreesitter.HighlightRange
	var lastPos uint32
	var lastCapture string
	hasLast := false

	flush := func(end uint32) {
		if hasLast && end > lastPos && lastCapture != "" {
			result = append(result, gotreesitter.HighlightRange{
				StartByte: lastPos,
				EndByte:   end,
				Capture:   lastCapture,
			})
		}
	}

	for _, ev := range events {
		if ev.pos > lastPos && hasLast {
			flush(ev.pos)
		}
		if ev.isStart {
			stack = append(stack, stackEntry{idx: ev.idx})
			active[ev.idx] = true
		} else {
			active[ev.idx] = false
			for len(stack) > 0 && !active[stack[len(stack)-1].idx] {
				stack = stack[:len(stack)-1]
			}
		}

		lastPos = ev.pos
		lastCapture = ""
		hasLast = true
		for i := len(stack) - 1; i >= 0; i-- {
			if active[stack[i].idx] {
				lastCapture = ranges[stack[i].idx].Capture
				break
			}
		}
	}
	return result
}

func (c *treeSitterRenderCache) matchesAlt(code []byte) bool {
	if len(c.altSource) != len(code) {
		return false
	}
	if len(code) == 0 {
		return true
	}
	if c.altSource[0] != code[0] || c.altSource[len(code)-1] != code[len(code)-1] {
		return false
	}
	if len(code) > 16 {
		if !bytes.Equal(c.altSource[:8], code[:8]) {
			return false
		}
		if !bytes.Equal(c.altSource[len(code)-8:], code[len(code)-8:]) {
			return false
		}
	}
	return bytes.Equal(c.altSource, code)
}

func computeSingleInputEdit(oldSource, newSource []byte) (gotreesitter.InputEdit, bool) {
	if bytes.Equal(oldSource, newSource) {
		return gotreesitter.InputEdit{}, false
	}

	minLen := len(oldSource)
	if len(newSource) < minLen {
		minLen = len(newSource)
	}
	prefix := 0
	for prefix < minLen && oldSource[prefix] == newSource[prefix] {
		prefix++
	}

	oldRemain := len(oldSource) - prefix
	newRemain := len(newSource) - prefix
	suffix := 0
	for suffix < oldRemain && suffix < newRemain &&
		oldSource[len(oldSource)-1-suffix] == newSource[len(newSource)-1-suffix] {
		suffix++
	}

	oldEnd := len(oldSource) - suffix
	newEnd := len(newSource) - suffix

	startPoint := pointAtOffset(oldSource, prefix)
	oldEndPoint := pointAtOffset(oldSource, oldEnd)
	newEndPoint := pointAtOffset(newSource, newEnd)

	return gotreesitter.InputEdit{
		StartByte:   uint32(prefix),
		OldEndByte:  uint32(oldEnd),
		NewEndByte:  uint32(newEnd),
		StartPoint:  startPoint,
		OldEndPoint: oldEndPoint,
		NewEndPoint: newEndPoint,
	}, true
}

func pointAtOffset(source []byte, offset int) gotreesitter.Point {
	if offset <= 0 {
		return gotreesitter.Point{}
	}
	if offset > len(source) {
		offset = len(source)
	}
	var row, col uint32
	for i := 0; i < offset; i++ {
		if source[i] == '\n' {
			row++
			col = 0
			continue
		}
		col++
	}
	return gotreesitter.Point{Row: row, Column: col}
}

func normalizeHighlightRanges(codeLen int, ranges []gotreesitter.HighlightRange) []normalizedHighlightRange {
	out := make([]normalizedHighlightRange, 0, len(ranges))
	classByCapture := make(map[string]string, 32)
	last := 0
	for _, hr := range ranges {
		start := int(hr.StartByte)
		end := int(hr.EndByte)
		if start < last {
			start = last
		}
		if start > codeLen {
			break
		}
		if end > codeLen {
			end = codeLen
		}
		if end <= start {
			continue
		}

		className, ok := classByCapture[hr.Capture]
		if !ok {
			className = treeSitterCaptureToChromaClass(hr.Capture)
			classByCapture[hr.Capture] = className
		}
		n := len(out)
		if n > 0 && out[n-1].class == className && out[n-1].end == start {
			out[n-1].end = end
		} else {
			out = append(out, normalizedHighlightRange{start: start, end: end, class: className})
		}
		last = end
	}
	return out
}

func tryRenderCodeByTreeSitter(fileName, fileLang string, code []byte, allowAnalyze, trimTrailingNewline bool) (template.HTML, string, bool) {
	var entry *tsgrammars.LangEntry
	if allowAnalyze {
		entry = resolveTreeSitterEntryWithAnalyze(fileName, fileLang, code)
	} else {
		entry = resolveTreeSitterEntry(fileName, fileLang)
	}

	renderer := getTreeSitterRenderer(entry)
	if renderer == nil {
		return "", "", false
	}

	rendered, ok := renderer.render(code, trimTrailingNewline)
	if !ok {
		return "", "", false
	}
	return rendered, renderer.displayName, true
}

func tryRenderCodeByTreeSitterWithLexer(lexerName string, code []byte, trimTrailingNewline bool) (template.HTML, string, bool) {
	entry := lookupTreeSitterEntryByLanguageName(lexerName)
	renderer := getTreeSitterRenderer(entry)
	if renderer == nil {
		return "", "", false
	}
	rendered, ok := renderer.render(code, trimTrailingNewline)
	if !ok {
		return "", "", false
	}
	return rendered, renderer.displayName, true
}

func treeSitterCaptureToChromaClass(capture string) string {
	capture = strings.ToLower(capture)
	switch {
	case capture == "", capture == "none", capture == "spell", capture == "embedded":
		return ""
	case strings.HasPrefix(capture, "comment"):
		return "c"
	case strings.HasPrefix(capture, "string.escape"):
		return "se"
	case strings.HasPrefix(capture, "string.regex"):
		return "sr"
	case strings.HasPrefix(capture, "string.special"):
		return "ss"
	case strings.HasPrefix(capture, "string"):
		return "s"
	case strings.HasPrefix(capture, "keyword"),
		strings.HasPrefix(capture, "conditional"),
		strings.HasPrefix(capture, "repeat"),
		strings.HasPrefix(capture, "exception"):
		return "k"
	case strings.HasPrefix(capture, "include"),
		strings.HasPrefix(capture, "namespace"):
		return "nn"
	case strings.HasPrefix(capture, "operator"):
		return "o"
	case strings.HasPrefix(capture, "punctuation"),
		strings.HasPrefix(capture, "delimiter"):
		return "p"
	case strings.HasPrefix(capture, "number"),
		strings.HasPrefix(capture, "float"):
		return "m"
	case strings.HasPrefix(capture, "boolean"):
		return "kc"
	case strings.HasPrefix(capture, "function.builtin"):
		return "nb"
	case strings.HasPrefix(capture, "function"),
		strings.HasPrefix(capture, "method"),
		strings.HasPrefix(capture, "constructor"):
		return "nf"
	case strings.HasPrefix(capture, "type"):
		return "kt"
	case strings.HasPrefix(capture, "attribute"),
		strings.HasPrefix(capture, "field"):
		return "na"
	case strings.HasPrefix(capture, "property"):
		return "py"
	case strings.HasPrefix(capture, "variable.builtin"):
		return "nb"
	case strings.HasPrefix(capture, "variable"),
		strings.HasPrefix(capture, "parameter"):
		return "nv"
	case strings.HasPrefix(capture, "constant"):
		return "no"
	case strings.HasPrefix(capture, "label"):
		return "nl"
	case strings.HasPrefix(capture, "tag"):
		return "nt"
	case strings.HasPrefix(capture, "error"):
		return "err"
	default:
		return "nx"
	}
}

func splitTreeSitterRenderedLines(code template.HTML) []template.HTML {
	rawLines := UnsafeSplitHighlightedLines(code)
	lines := make([]template.HTML, 0, len(rawLines))
	for _, line := range rawLines {
		lines = append(lines, template.HTML(util.UnsafeBytesToString(line)))
	}
	return lines
}

func renderFullFileByTreeSitter(fileName, language string, code []byte) ([]template.HTML, string, error) {
	entry := resolveTreeSitterEntryWithAnalyze(fileName, language, code)
	renderer := getTreeSitterRenderer(entry)
	if renderer == nil {
		return nil, "", errors.New("tree-sitter renderer unavailable")
	}
	lines, ok := renderer.renderLines(code)
	if !ok {
		return nil, "", errors.New("tree-sitter renderer unavailable")
	}
	return lines, renderer.displayName, nil
}
