// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"path"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/analyze"
	"code.gitea.io/gitea/modules/log"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/go-enry/go-enry/v2"
)

const mapKeyLowerPrefix = "lower/"

// chromaLexers is fully managed by us to do fast lookup for chroma lexers by file name or language name
// Don't use lexers.Get because it is very slow in many cases (iterate all rules, filepath glob match, etc.)
var chromaLexers = sync.OnceValue(func() (ret struct {
	conflictingExtLangMap map[string]string

	lowerNameMap map[string]chroma.Lexer // lexer name (lang name) in lower-case
	fileBaseMap  map[string]chroma.Lexer
	fileExtMap   map[string]chroma.Lexer
	fileParts    []struct {
		part  string
		lexer chroma.Lexer
	}
},
) {
	ret.lowerNameMap = make(map[string]chroma.Lexer)
	ret.fileBaseMap = make(map[string]chroma.Lexer)
	ret.fileExtMap = make(map[string]chroma.Lexer)

	// Chroma has overlaps in file extension for different languages,
	// When we need to do fast render, there is no way to detect the language by content,
	// So we can only choose some default languages for the overlapped file extensions.
	ret.conflictingExtLangMap = map[string]string{
		".as":      "ActionScript 3", // ActionScript
		".asm":     "NASM",           // TASM, NASM, RGBDS Assembly, Z80 Assembly
		".ASM":     "NASM",
		".bas":     "VB.net",       // QBasic
		".bf":      "Beef",         // Brainfuck
		".fs":      "FSharp",       // Forth
		".gd":      "GDScript",     // GDScript3
		".h":       "C",            // Objective-C
		".hcl":     "Terraform",    // HCL
		".hh":      "C++",          // HolyC
		".inc":     "PHP",          // ObjectPascal, POVRay, SourcePawn, PHTML
		".m":       "Objective-C",  // Matlab, Mathematica, Mason
		".mc":      "Mason",        // MonkeyC
		".network": "SYSTEMD",      // INI
		".php":     "PHP",          // PHTML
		".php3":    "PHP",          // PHTML
		".php4":    "PHP",          // PHTML
		".php5":    "PHP",          // PHTML
		".pl":      "Perl",         // Prolog, Raku
		".pm":      "Perl",         // Promela, Raku
		".pp":      "ObjectPascal", // Puppet
		".s":       "ArmAsm",       // GAS
		".S":       "ArmAsm",       // R, GAS
		".service": "SYSTEMD",      // INI
		".socket":  "SYSTEMD",      // INI
		".sql":     "SQL",          // MySQL
		".t":       "Perl",         // Raku
		".ts":      "TypeScript",   // TypoScript
		".v":       "V",            // verilog
		".xslt":    "HTML",         // XML
	}

	isPlainPattern := func(key string) bool {
		return !strings.ContainsAny(key, "*?[]") // only support simple patterns
	}

	setMapWithLowerKey := func(m map[string]chroma.Lexer, key string, lexer chroma.Lexer) {
		if _, conflict := m[key]; conflict {
			panic("duplicate key in lexer map: " + key + ", need to add it to conflictingExtLangMap")
		}
		m[key] = lexer
		m[mapKeyLowerPrefix+strings.ToLower(key)] = lexer
	}

	processFileName := func(fileName string, lexer chroma.Lexer) bool {
		if isPlainPattern(fileName) {
			// full base name match
			setMapWithLowerKey(ret.fileBaseMap, fileName, lexer)
			return true
		}
		if strings.HasPrefix(fileName, "*") {
			// ext name match: "*.js"
			fileExt := strings.Trim(fileName, "*")
			if isPlainPattern(fileExt) {
				presetName := ret.conflictingExtLangMap[fileExt]
				if presetName == "" || lexer.Config().Name == presetName {
					setMapWithLowerKey(ret.fileExtMap, fileExt, lexer)
				}
				return true
			}
		}
		if strings.HasSuffix(fileName, "*") {
			// part match: "*.env.*"
			filePart := strings.Trim(fileName, "*")
			if isPlainPattern(filePart) {
				ret.fileParts = append(ret.fileParts, struct {
					part  string
					lexer chroma.Lexer
				}{
					part:  filePart,
					lexer: lexer,
				})
				return true
			}
		}
		return false
	}

	expandGlobPatterns := func(patterns []string) []string {
		// expand patterns like "file.[ch]" to "file.c" and "file.h", only one pair of "[]" is supported, enough for current Chroma lexers
		for idx, s := range patterns {
			idx1 := strings.IndexByte(s, '[')
			idx2 := strings.IndexByte(s, ']')
			if idx1 != -1 && idx2 != -1 && idx2 > idx1+1 {
				left, mid, right := s[:idx1], s[idx1+1:idx2], s[idx2+1:]
				patterns[idx] = left + mid[0:1] + right
				for i := 1; i < len(mid); i++ {
					patterns = append(patterns, left+mid[i:i+1]+right)
				}
			}
		}
		return patterns
	}

	// add lexers to our map, for fast lookup
	for _, lexer := range lexers.GlobalLexerRegistry.Lexers {
		cfg := lexer.Config()
		ret.lowerNameMap[strings.ToLower(lexer.Config().Name)] = lexer
		for _, alias := range cfg.Aliases {
			ret.lowerNameMap[strings.ToLower(alias)] = lexer
		}
		for _, s := range expandGlobPatterns(cfg.Filenames) {
			if !processFileName(s, lexer) {
				panic("unsupported file name pattern in lexer: " + s)
			}
		}
		for _, s := range expandGlobPatterns(cfg.AliasFilenames) {
			if !processFileName(s, lexer) {
				panic("unsupported alias file name pattern in lexer: " + s)
			}
		}
	}

	// final check: make sure the default ext-lang mapping is correct, nothing is missing
	for ext, lexerName := range ret.conflictingExtLangMap {
		if lexer, ok := ret.fileExtMap[ext]; !ok || lexer.Config().Name != lexerName {
			panic("missing default ext-lang mapping for: " + ext)
		}
	}
	return ret
})

func normalizeFileNameLang(fileName, fileLang string) (string, string) {
	fileName = path.Base(fileName)
	fileLang, _, _ = strings.Cut(fileLang, "?") // maybe, the value from gitattributes might contain `?` parameters?
	ext := path.Ext(fileName)
	// the "lang" might come from enry or gitattributes, it has different naming for some languages
	switch fileLang {
	case "F#":
		fileLang = "FSharp"
	case "Pascal":
		fileLang = "ObjectPascal"
	case "C":
		if ext == ".C" || ext == ".H" {
			fileLang = "C++"
		}
	}
	return fileName, fileLang
}

func DetectChromaLexerByFileName(fileName, fileLang string) chroma.Lexer {
	lexer, _ := detectChromaLexerByFileName(fileName, fileLang)
	return lexer
}

func detectChromaLexerByFileName(fileName, fileLang string) (_ chroma.Lexer, byLang bool) {
	fileName, fileLang = normalizeFileNameLang(fileName, fileLang)
	fileExt := path.Ext(fileName)

	// apply custom mapping for file extension, highest priority, for example:
	// * ".my-js" -> ".js"
	// * ".my-html" -> "HTML"
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

	// try to use language for lexer name
	if fileLang != "" {
		lexer := chromaLexers().lowerNameMap[strings.ToLower(fileLang)]
		if lexer != nil {
			return lexer, true
		}
	}

	if fileName == "" {
		return lexers.Fallback, false
	}

	// try base name
	{
		baseName := path.Base(fileName)
		if lexer, ok := chromaLexers().fileBaseMap[baseName]; ok {
			return lexer, false
		} else if lexer, ok = chromaLexers().fileBaseMap[mapKeyLowerPrefix+strings.ToLower(baseName)]; ok {
			return lexer, false
		}
	}

	if fileExt == "" {
		return lexers.Fallback, false
	}

	// try ext name
	{
		if lexer, ok := chromaLexers().fileExtMap[fileExt]; ok {
			return lexer, false
		} else if lexer, ok = chromaLexers().fileExtMap[mapKeyLowerPrefix+strings.ToLower(fileExt)]; ok {
			return lexer, false
		}
	}

	// try file part match, for example: ".env.local" for "*.env.*"
	// it assumes that there must be a dot in filename (fileExt isn't empty)
	for _, item := range chromaLexers().fileParts {
		if strings.Contains(fileName, item.part) {
			return item.lexer, false
		}
	}
	return lexers.Fallback, false
}

// detectChromaLexerWithAnalyze returns a chroma lexer by given file name, language and code content. All parameters can be optional.
// When code content is provided, it will be slow if no lexer is found by file name or language.
// If no lexer is found, it will return the fallback lexer.
func detectChromaLexerWithAnalyze(fileName, lang string, code []byte) chroma.Lexer {
	lexer, byLang := detectChromaLexerByFileName(fileName, lang)

	// if lang is provided, and it matches a lexer, use it directly
	if byLang {
		return lexer
	}

	// if a lexer is detected and there is no conflict for the file extension, use it directly
	fileExt := path.Ext(fileName)
	_, hasConflicts := chromaLexers().conflictingExtLangMap[fileExt]
	if !hasConflicts && lexer != lexers.Fallback {
		return lexer
	}

	// try to detect language by content, for best guessing for the language
	// when using "code" to detect, analyze.GetCodeLanguage is slow, it iterates many rules to detect language from content
	analyzedLanguage := analyze.GetCodeLanguage(fileName, code)
	lexer = DetectChromaLexerByFileName(fileName, analyzedLanguage)
	if lexer == lexers.Fallback {
		if analyzedLanguage != enry.OtherLanguage {
			log.Warn("No chroma lexer found for enry detected language: %s (file: %s), need to fix the language mapping between enry and chroma.", analyzedLanguage, fileName)
		}
	}
	return lexer
}
