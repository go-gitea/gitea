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

	"github.com/alecthomas/chroma/v2"
	"github.com/odvcencio/gotreesitter"
	tsgrammars "github.com/odvcencio/gotreesitter/grammars"
	"golang.org/x/sync/singleflight"
)

type treeSitterRenderer struct {
	languageName      string
	displayName       string
	highlighter       *gotreesitter.Highlighter
	captureClassCache map[string]string
	mu                sync.Mutex
	cache             treeSitterRenderCache
}

type normalizedHighlightRange struct {
	start int
	end   int
	class string
}

type treeSitterRenderAttempt struct {
	rendered       template.HTML
	lines          []template.HTML
	displayName    string
	ok             bool
	fallbackReason highlightFallbackReason
}

var (
	treeSitterRendererCache sync.Map           // map[string]*treeSitterRenderer
	treeSitterRendererGroup singleflight.Group // dedup concurrent NewHighlighter calls
)

func isGenericPlainTextExtension(fileExt string) bool {
	switch strings.ToLower(fileExt) {
	case ".txt", ".text":
		return true
	default:
		return false
	}
}

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

func lookupTreeSitterEntryByChromaLexer(lexer chroma.Lexer) *tsgrammars.LangEntry {
	if lexer == nil || lexer.Config() == nil {
		return nil
	}
	cfg := lexer.Config()
	candidates := make([]string, 0, 1+len(cfg.Aliases))
	if cfg.Name != "" {
		candidates = append(candidates, cfg.Name)
	}
	candidates = append(candidates, cfg.Aliases...)
	for _, candidate := range candidates {
		if entry := lookupTreeSitterEntryByLanguageName(candidate); entry != nil {
			return entry
		}
	}
	return nil
}

func resolveTreeSitterEntryByChroma(fileName, fileLang string, code []byte, allowAnalyze bool) *tsgrammars.LangEntry {
	if allowAnalyze {
		return lookupTreeSitterEntryByChromaLexer(detectChromaLexerWithAnalyze(fileName, fileLang, code))
	}
	lexer, _ := detectChromaLexerByFileName(fileName, fileLang)
	return lookupTreeSitterEntryByChromaLexer(lexer)
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

	// Keep generic plaintext extensions on the plaintext/chroma fallback path
	// unless the caller provided explicit language metadata. The bundled
	// gotreesitter linguist maps ".txt" to Vim help files, which is too
	// aggressive for Gitea's generic repository viewer.
	if fileLang == "" && isGenericPlainTextExtension(path.Ext(fileName)) {
		switch path.Base(fileName) {
		case "CMakeLists.txt":
			// CMakeLists.txt is a canonical exact filename, not generic plaintext.
		default:
			return nil
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
	if entry == nil {
		// Final fallback: use chroma/enry heuristics, then map lexer names back
		// to a tree-sitter grammar when possible. Code is nil because
		// allowAnalyze is false — only filename/language-based detection.
		entry = resolveTreeSitterEntryByChroma(fileName, fileLang, nil, false)
	}
	return entry
}

func resolveTreeSitterEntryWithAnalyze(fileName, fileLang string, code []byte) *tsgrammars.LangEntry {
	entry := resolveTreeSitterEntry(fileName, fileLang)
	if entry != nil || len(code) == 0 {
		return entry
	}

	analyzedLanguage := analyze.GetCodeLanguage(fileName, code)
	entry = lookupTreeSitterEntryByLanguageName(analyzedLanguage)
	if entry != nil {
		return entry
	}
	return resolveTreeSitterEntryByChroma(fileName, fileLang, code, true)
}

func getTreeSitterRenderer(entry *tsgrammars.LangEntry) *treeSitterRenderer {
	if entry == nil || strings.TrimSpace(entry.HighlightQuery) == "" {
		return nil
	}

	if val, ok := treeSitterRendererCache.Load(entry.Name); ok {
		renderer, _ := val.(*treeSitterRenderer)
		return renderer
	}

	// Use singleflight to avoid duplicate expensive NewHighlighter calls
	// when multiple goroutines race for the same language.
	val, _, _ := treeSitterRendererGroup.Do(entry.Name, func() (any, error) {
		// Double-check after winning the race.
		if val, ok := treeSitterRendererCache.Load(entry.Name); ok {
			return val, nil
		}

		lang := entry.Language()
		if lang == nil {
			treeSitterRendererCache.Store(entry.Name, (*treeSitterRenderer)(nil))
			return (*treeSitterRenderer)(nil), nil
		}

		var opts []gotreesitter.HighlighterOption
		if entry.TokenSourceFactory != nil {
			opts = append(opts, gotreesitter.WithTokenSourceFactory(func(src []byte) gotreesitter.TokenSource {
				return entry.TokenSourceFactory(src, lang)
			}))
		}

		highlighter, err := gotreesitter.NewHighlighter(lang, entry.HighlightQuery, opts...)
		if err != nil {
			log.Warn("failed to construct gotreesitter highlighter for %s: %v", entry.Name, err)
			treeSitterRendererCache.Store(entry.Name, (*treeSitterRenderer)(nil))
			return (*treeSitterRenderer)(nil), nil
		}

		renderer := &treeSitterRenderer{
			languageName:      entry.Name,
			displayName:       tsgrammars.DisplayName(entry),
			highlighter:       highlighter,
			captureClassCache: make(map[string]string, 32),
		}
		treeSitterRendererCache.Store(entry.Name, renderer)
		return renderer, nil
	})

	renderer, _ := val.(*treeSitterRenderer)
	return renderer
}

func tryRenderCodeByTreeSitter(fileName, fileLang string, code []byte) (template.HTML, bool) {
	attempt := tryRenderCodeByTreeSitterDetailed(fileName, fileLang, code, false)
	return attempt.rendered, attempt.ok
}

func tryRenderCodeByTreeSitterDetailed(fileName, fileLang string, code []byte, allowAnalyze bool) treeSitterRenderAttempt {
	var entry *tsgrammars.LangEntry
	if allowAnalyze {
		entry = resolveTreeSitterEntryWithAnalyze(fileName, fileLang, code)
	} else {
		entry = resolveTreeSitterEntry(fileName, fileLang)
	}
	if entry == nil {
		return treeSitterRenderAttempt{fallbackReason: highlightFallbackEntryUnavailable}
	}

	renderer := getTreeSitterRenderer(entry)
	if renderer == nil {
		return treeSitterRenderAttempt{fallbackReason: highlightFallbackRendererUnavailable}
	}

	rendered, ok := renderer.render(code, true)
	if !ok {
		return treeSitterRenderAttempt{fallbackReason: highlightFallbackRenderUnusable}
	}
	return treeSitterRenderAttempt{
		rendered:       rendered,
		displayName:    renderer.displayName,
		ok:             true,
		fallbackReason: highlightFallbackNone,
	}
}

func tryRenderCodeByTreeSitterWithLexerDetailed(lexerName string, code []byte) treeSitterRenderAttempt {
	entry := lookupTreeSitterEntryByLanguageName(lexerName)
	if entry == nil {
		return treeSitterRenderAttempt{fallbackReason: highlightFallbackEntryUnavailable}
	}
	renderer := getTreeSitterRenderer(entry)
	if renderer == nil {
		return treeSitterRenderAttempt{fallbackReason: highlightFallbackRendererUnavailable}
	}
	rendered, ok := renderer.render(code, true)
	if !ok {
		return treeSitterRenderAttempt{fallbackReason: highlightFallbackRenderUnusable}
	}
	return treeSitterRenderAttempt{
		rendered:       rendered,
		displayName:    renderer.displayName,
		ok:             true,
		fallbackReason: highlightFallbackNone,
	}
}

func renderFullFileByTreeSitter(fileName, language string, code []byte) ([]template.HTML, string, error) {
	attempt, err := renderFullFileByTreeSitterDetailed(fileName, language, code)
	if err != nil {
		return nil, "", err
	}
	return attempt.lines, attempt.displayName, nil
}

func renderFullFileByTreeSitterDetailed(fileName, language string, code []byte) (treeSitterRenderAttempt, error) {
	entry := resolveTreeSitterEntryWithAnalyze(fileName, language, code)
	if entry == nil {
		return treeSitterRenderAttempt{fallbackReason: highlightFallbackEntryUnavailable}, errors.New("tree-sitter renderer unavailable")
	}
	renderer := getTreeSitterRenderer(entry)
	if renderer == nil {
		return treeSitterRenderAttempt{fallbackReason: highlightFallbackRendererUnavailable}, errors.New("tree-sitter renderer unavailable")
	}
	lines, ok := renderer.renderLines(code)
	if !ok {
		return treeSitterRenderAttempt{fallbackReason: highlightFallbackRenderUnusable}, errors.New("tree-sitter renderer unavailable")
	}
	return treeSitterRenderAttempt{
		lines:          lines,
		displayName:    renderer.displayName,
		ok:             true,
		fallbackReason: highlightFallbackNone,
	}, nil
}

func treeSitterCaptureToChromaClass(capture string) string {
	capture = strings.ToLower(capture)
	switch {
	case capture == "", capture == "none", capture == "spell", capture == "embedded":
		return ""
	case strings.HasPrefix(capture, "comment"):
		return "c"
	case strings.HasPrefix(capture, "diff.plus"):
		return "gi"
	case strings.HasPrefix(capture, "diff.minus"):
		return "gd"
	case strings.HasPrefix(capture, "escape"):
		return "se"
	case strings.HasPrefix(capture, "character"):
		return "s"
	case strings.HasPrefix(capture, "string.escape"):
		return "se"
	case strings.HasPrefix(capture, "string.regex"):
		return "sr"
	case strings.HasPrefix(capture, "string.special.key"):
		return "nt"
	case strings.HasPrefix(capture, "string.special"):
		return "ss"
	case strings.HasPrefix(capture, "string"):
		return "s"
	case strings.HasPrefix(capture, "text.literal"),
		strings.HasPrefix(capture, "text.uri"),
		strings.HasPrefix(capture, "markup.raw"),
		strings.HasPrefix(capture, "markup.link"):
		return "s"
	case strings.HasPrefix(capture, "keyword"),
		strings.HasPrefix(capture, "conditional"),
		strings.HasPrefix(capture, "repeat"),
		strings.HasPrefix(capture, "exception"),
		strings.HasPrefix(capture, "constant.builtin"),
		strings.HasPrefix(capture, "constant.language"):
		return "k"
	case strings.HasPrefix(capture, "include"),
		strings.HasPrefix(capture, "namespace"),
		strings.HasPrefix(capture, "module"):
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
		return "nt"
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
