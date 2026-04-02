// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"bytes"
	"html/template"
	"slices"
	"sync"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
)

// don't highlight files larger than this many bytes for performance purposes
const sizeLimit = 1024 * 1024

type globalVarsType struct {
	highlightMapping map[string]string
	githubStyles     *chroma.Style
	escapeFull       []template.HTML
	escCtrlCharsMap  []template.HTML
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
		globalVarsPtr.escCtrlCharsMap = make([]template.HTML, 256)
		// ASCII Table 0x00 - 0x1F
		controlCharNames := []string{
			"NUL", "SOH", "STX", "ETX", "EOT", "ENQ", "ACK", "BEL",
			"BS", "HT", "LF", "VT", "FF", "CR", "SO", "SI",
			"DLE", "DC1", "DC2", "DC3", "DC4", "NAK", "SYN", "ETB",
			"CAN", "EM", "SUB", "ESC", "FS", "GS", "RS", "US",
		}
		// Uncomment this line if you'd debug the layout without creating a special file, then Space (0x20) will also be escaped.
		// Don't worry, even if you forget to comment it out and push it to git repo, the CI tests will catch it and fail.
		// controlCharNames = append(controlCharNames, "SP")
		for i, s := range controlCharNames {
			globalVarsPtr.escCtrlCharsMap[i] = template.HTML(`<span class="broken-code-point" data-escaped="` + s + `"><span class="char">` + string(byte(i)) + `</span></span>`)
		}
		globalVarsPtr.escCtrlCharsMap[0x7f] = template.HTML(`<span class="broken-code-point" data-escaped="DEL"><span class="char">` + string(byte(0x7f)) + `</span></span>`)
		globalVarsPtr.escCtrlCharsMap['\t'] = ""
		globalVarsPtr.escCtrlCharsMap['\n'] = ""
		globalVarsPtr.escCtrlCharsMap['\r'] = ""

		globalVarsPtr.escapeFull = slices.Clone(globalVarsPtr.escCtrlCharsMap)
		// exactly the same as Golang's html.EscapeString
		globalVarsPtr.escapeFull['&'] = "&amp;"
		globalVarsPtr.escapeFull['\''] = "&#39;"
		globalVarsPtr.escapeFull['<'] = "&lt;"
		globalVarsPtr.escapeFull['>'] = "&gt;"
		globalVarsPtr.escapeFull['"'] = "&#34;"
	}
	return globalVarsPtr
}

func escapeByMap(code []byte, escapeMap []template.HTML) template.HTML {
	firstEscapePos := -1
	for i, c := range code {
		if escapeMap[c] != "" {
			firstEscapePos = i
			break
		}
	}
	if firstEscapePos == -1 {
		return template.HTML(util.UnsafeBytesToString(code))
	}

	buf := make([]byte, firstEscapePos, len(code)*2)
	copy(buf[:firstEscapePos], code[:firstEscapePos])
	for i := firstEscapePos; i < len(code); i++ {
		c := code[i]
		if esc := escapeMap[c]; esc != "" {
			buf = append(buf, esc...)
		} else {
			buf = append(buf, c)
		}
	}
	return template.HTML(util.UnsafeBytesToString(buf))
}

func escapeFullString(code string) template.HTML {
	return escapeByMap(util.UnsafeStringToBytes(code), globalVars().escapeFull)
}

func escapeControlChars(code []byte) template.HTML {
	return escapeByMap(code, globalVars().escCtrlCharsMap)
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

// RenderCodeSlowGuess tries to get a lexer by file name and language first,
// if not found, it will try to guess the lexer by code content, which is slow (more than several hundreds of milliseconds).
func RenderCodeSlowGuess(fileName, language, code string) (output template.HTML, lexer chroma.Lexer, lexerDisplayName string) {
	// diff view newline will be passed as empty, change to literal '\n' so it can be copied
	// preserve literal newline in blame view
	if code == "" || code == "\n" {
		return "\n", nil, ""
	}

	if len(code) > sizeLimit {
		return escapeFullString(code), nil, ""
	}

	lexer = detectChromaLexerWithAnalyze(fileName, language, util.UnsafeStringToBytes(code)) // it is also slow
	return RenderCodeByLexer(lexer, code), lexer, formatLexerName(lexer.Config().Name)
}

// RenderCodeByLexer returns a HTML version of code string with chroma syntax highlighting classes
func RenderCodeByLexer(lexer chroma.Lexer, code string) template.HTML {
	formatter := html.New(html.WithClasses(true),
		html.WithLineNumbers(false),
		html.PreventSurroundingPre(true),
	)

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		log.Error("Can't tokenize code: %v", err)
		return escapeFullString(code)
	}

	htmlBuf := &bytes.Buffer{}
	// style not used for live site but need to pass something
	err = formatter.Format(htmlBuf, globalVars().githubStyles, iterator)
	if err != nil {
		log.Error("Can't format code: %v", err)
		return escapeFullString(code)
	}

	// At the moment, we do not escape control chars here (unlike RenderFullFile which escapes control chars).
	// The reason is: it is a very rare case that a text file contains control chars.
	// This function is usually used by highlight diff and blame, not quite sure whether there will be side effects.
	// If there would be new user feedback about this, we can re-consider about various edge cases.
	return template.HTML(htmlBuf.String())
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
	lines := make([]template.HTML, 0, len(unsafeLines))
	for _, lineBytes := range unsafeLines {
		line := escapeControlChars(lineBytes)
		lines = append(lines, line)
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
		lines = append(lines, escapeFullString(util.UnsafeBytesToString(content)))
	}
	return lines
}

func formatLexerName(name string) string {
	if name == LanguagePlaintext || name == chromaLexerFallback {
		return "Plaintext"
	}
	return util.ToTitleCaseNoLower(name)
}
