// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"errors"
	"html/template"
	"path"
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
	displayName string
	highlighter *gotreesitter.Highlighter
	mu          sync.Mutex
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

	treeSitterDetectCache   sync.Map // map[string]*tsgrammars.LangEntry
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

	cacheKey := fileName + "\x00" + fileLang
	if cached, ok := treeSitterDetectCache.Load(cacheKey); ok {
		if entry, ok := cached.(*tsgrammars.LangEntry); ok {
			return entry
		}
		return nil
	}

	var entry *tsgrammars.LangEntry
	if fileLang != "" {
		entry = lookupTreeSitterEntryByLanguageName(fileLang)
	}
	if entry == nil && fileName != "" {
		entry = tsgrammars.DetectLanguage(fileName)
	}

	treeSitterDetectCache.Store(cacheKey, entry)
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

	opts := make([]gotreesitter.HighlighterOption, 0, 1)
	if entry.TokenSourceFactory != nil {
		opts = append(opts, gotreesitter.WithTokenSourceFactory(func(src []byte) gotreesitter.TokenSource {
			return entry.TokenSourceFactory(src, lang)
		}))
	}

	highlighter, err := gotreesitter.NewHighlighter(lang, entry.HighlightQuery, opts...)
	if err != nil {
		log.Warn("failed to create gotreesitter highlighter for %s: %v", entry.Name, err)
		treeSitterRendererCache.Store(entry.Name, (*treeSitterRenderer)(nil))
		return nil
	}

	renderer := &treeSitterRenderer{
		displayName: treeSitterDisplayName(entry),
		highlighter: highlighter,
	}
	val, _ := treeSitterRendererCache.LoadOrStore(entry.Name, renderer)
	resolvedRenderer, _ := val.(*treeSitterRenderer)
	return resolvedRenderer
}

func (renderer *treeSitterRenderer) render(code []byte, trimTrailingNewline bool) (template.HTML, bool) {
	if renderer == nil || renderer.highlighter == nil {
		return "", false
	}

	defer func() {
		if err := recover(); err != nil {
			log.Warn("panic while rendering with gotreesitter: %v\n%s", err, log.Stack(2))
		}
	}()

	renderer.mu.Lock()
	ranges := renderer.highlighter.Highlight(code)
	renderer.mu.Unlock()

	var out strings.Builder
	out.Grow(len(code) + len(ranges)*32)

	last := 0
	for _, hr := range ranges {
		start := int(hr.StartByte)
		end := int(hr.EndByte)

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
			out.WriteString(template.HTMLEscapeString(util.UnsafeBytesToString(code[last:start])))
		}

		className := treeSitterCaptureToChromaClass(hr.Capture)
		escaped := template.HTMLEscapeString(util.UnsafeBytesToString(code[start:end]))
		if className == "" {
			out.WriteString(escaped)
		} else {
			out.WriteString(`<span class="`)
			out.WriteString(className)
			out.WriteString(`">`)
			out.WriteString(escaped)
			out.WriteString(`</span>`)
		}
		last = end
	}

	if last < len(code) {
		out.WriteString(template.HTMLEscapeString(util.UnsafeBytesToString(code[last:])))
	}

	content := out.String()
	if trimTrailingNewline {
		content = strings.TrimSuffix(content, "\n")
	}
	return template.HTML(content), true
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
	rendered, displayName, ok := tryRenderCodeByTreeSitter(fileName, language, code, true, false)
	if !ok {
		return nil, "", errors.New("tree-sitter renderer unavailable")
	}
	return splitTreeSitterRenderedLines(rendered), displayName, nil
}
