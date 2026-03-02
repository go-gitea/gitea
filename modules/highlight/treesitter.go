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

	"github.com/odvcencio/gotreesitter"
	tsgrammars "github.com/odvcencio/gotreesitter/grammars"
)

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

type normalizedHighlightRange struct {
	start int
	end   int
	class string
}

var treeSitterRendererCache sync.Map // map[string]*treeSitterRenderer

// lookupTreeSitterEntryByLanguageName resolves a language name to a
// tree-sitter grammar entry using gotreesitter's built-in linguist
// name mapping. Accepts any linguist name, alias, or grammar name
// (e.g., "C++", "golang", "Shell", "bash").
func lookupTreeSitterEntryByLanguageName(name string) *tsgrammars.LangEntry {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	// Reject plaintext/text — these have no grammar.
	switch strings.ToLower(name) {
	case "plaintext", "text":
		return nil
	}
	entry := tsgrammars.DetectLanguageByName(name)
	if entry != nil && strings.TrimSpace(entry.HighlightQuery) == "" {
		return nil // grammar exists but has no highlight query
	}
	return entry
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

	// Prefer explicit language metadata (enry/gitattributes) before filename
	// fallback. This avoids wrong grammar selection on ambiguous extensions
	// like ".h" where metadata can disambiguate C vs Objective-C vs C++.
	var entry *tsgrammars.LangEntry
	if fileLang != "" {
		entry = lookupTreeSitterEntryByLanguageName(fileLang)
	}
	// DetectLanguage now checks registry extensions, linguist filenames
	// (e.g. "Makefile", ".bashrc"), and linguist extended extensions.
	if entry == nil && fileName != "" {
		entry = tsgrammars.DetectLanguage(fileName)
		// Filter out entries without highlight queries.
		if entry != nil && strings.TrimSpace(entry.HighlightQuery) == "" {
			entry = nil
		}
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
		displayName:       tsgrammars.DisplayName(entry),
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

func tryRenderCodeByTreeSitter(fileName, fileLang string, code []byte, allowAnalyze bool) (template.HTML, string, bool) {
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

	rendered, ok := renderer.render(code, true)
	if !ok {
		return "", "", false
	}
	return rendered, renderer.displayName, true
}

func tryRenderCodeByTreeSitterWithLexer(lexerName string, code []byte) (template.HTML, string, bool) {
	entry := lookupTreeSitterEntryByLanguageName(lexerName)
	renderer := getTreeSitterRenderer(entry)
	if renderer == nil {
		return "", "", false
	}
	rendered, ok := renderer.render(code, true)
	if !ok {
		return "", "", false
	}
	return rendered, renderer.displayName, true
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
