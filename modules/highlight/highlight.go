// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"bytes"
	gohtml "html"
	"html/template"
	"sync"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
)

// don't highlight files larger than this many bytes for performance purposes
const sizeLimit = 1024 * 1024

type globalVarsType struct {
	highlightMapping map[string]string
	githubStyles     *chroma.Style
}

var (
	globalVarsMu  sync.Mutex
	globalVarsPtr *globalVarsType
)

func globalVars() *globalVarsType {
	// in the future, the globalVars might need to be re-initialized when settings change, so don't use sync.Once here
	globalVarsMu.Lock()
	defer globalVarsMu.Unlock()
	if globalVarsPtr == nil {
		globalVarsPtr = &globalVarsType{}
		globalVarsPtr.githubStyles = styles.Get("github")
		globalVarsPtr.highlightMapping = setting.GetHighlightMapping()
	}
	return globalVarsPtr
}

// UnsafeSplitHighlightedLines splits highlighted code into lines preserving HTML tags
// It always includes '\n', '\n' can appear at the end of each line or in the middle of HTML tags
// The '\n' is necessary for copying code from web UI to preserve original code lines
// ATTENTION: It uses the unsafe conversion between string and []byte for performance reason
// DO NOT make any modification to the returned [][]byte slice items
func UnsafeSplitHighlightedLines(code template.HTML) (ret [][]byte) {
	buf := util.UnsafeStringToBytes(string(code))
	lineCount := bytes.Count(buf, []byte("\n")) + 1
	ret = make([][]byte, 0, lineCount)
	nlTagClose := []byte("\n</")
	for {
		pos := bytes.IndexByte(buf, '\n')
		if pos == -1 {
			if len(buf) > 0 {
				ret = append(ret, buf)
			}
			return ret
		}
		// Chroma highlighting output sometimes have "</span>" right after \n, sometimes before.
		// * "<span>text\n</span>"
		// * "<span>text</span>\n"
		if bytes.HasPrefix(buf[pos:], nlTagClose) {
			pos1 := bytes.IndexByte(buf[pos:], '>')
			if pos1 != -1 {
				pos += pos1
			}
		}
		ret = append(ret, buf[:pos+1])
		buf = buf[pos+1:]
	}
}

func htmlEscape(code string) template.HTML {
	return template.HTML(gohtml.EscapeString(code))
}

// RenderCodeSlowGuess tries to get a lexer by file name and language first,
// if not found, it will try to guess the lexer by code content, which is slow (more than several hundreds of milliseconds).
func RenderCodeSlowGuess(fileName, language, code string) (output template.HTML, lexer chroma.Lexer, lexerDisplayName string) {
	// diff view newline will be passed as empty, change to literal '\n' so it can be copied
	// preserve literal newline in blame view
	if code == "" || code == "\n" {
		return "\n", nil, ""
	}

	if len(code) > sizeLimit {
		return htmlEscape(code), nil, ""
	}

	lexer = detectChromaLexerWithAnalyze(fileName, language, util.UnsafeStringToBytes(code)) // it is also slow
	return RenderCodeByLexer(lexer, code), lexer, formatLexerName(lexer.Config().Name)
}

// RenderCodeByLexer returns a HTML version of code string with chroma syntax highlighting classes
func RenderCodeByLexer(lexer chroma.Lexer, code string) template.HTML {
	formatter := chromahtml.New(chromahtml.WithClasses(true),
		chromahtml.WithLineNumbers(false),
		chromahtml.PreventSurroundingPre(true),
	)

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		log.Error("Can't tokenize code: %v", err)
		return htmlEscape(code)
	}

	htmlBuf := &bytes.Buffer{}
	// style not used for live site but need to pass something
	err = formatter.Format(htmlBuf, globalVars().githubStyles, iterator)
	if err != nil {
		log.Error("Can't format code: %v", err)
		return htmlEscape(code)
	}
	return template.HTML(util.UnsafeBytesToString(htmlBuf.Bytes()))
}

// RenderFullFile returns a slice of chroma syntax highlighted HTML lines of code and the matched lexer name
func RenderFullFile(fileName, language string, code []byte) ([]template.HTML, string) {
	if language == LanguagePlaintext || len(code) > sizeLimit {
		return renderPlainText(code), formatLexerName(LanguagePlaintext)
	}
	lexer := detectChromaLexerWithAnalyze(fileName, language, code)
	lexerName := formatLexerName(lexer.Config().Name)
	rendered := RenderCodeByLexer(lexer, util.UnsafeBytesToString(code))
	unsafeLines := UnsafeSplitHighlightedLines(rendered)
	lines := make([]template.HTML, len(unsafeLines))
	for idx, lineBytes := range unsafeLines {
		lines[idx] = template.HTML(util.UnsafeBytesToString(lineBytes))
	}
	return lines, lexerName
}

// renderPlainText returns non-highlighted HTML for code
func renderPlainText(code []byte) []template.HTML {
	lines := make([]template.HTML, 0, bytes.Count(code, []byte{'\n'})+1)
	pos := 0
	for pos < len(code) {
		var content []byte
		nextPos := bytes.IndexByte(code[pos:], '\n')
		if nextPos == -1 {
			content = code[pos:]
			pos = len(code)
		} else {
			content = code[pos : pos+nextPos+1]
			pos += nextPos + 1
		}
		lines = append(lines, htmlEscape(util.UnsafeBytesToString(content)))
	}
	return lines
}

func formatLexerName(name string) string {
	if name == LanguagePlaintext || name == chromaLexerFallback {
		return "Plaintext"
	}
	return util.ToTitleCaseNoLower(name)
}
