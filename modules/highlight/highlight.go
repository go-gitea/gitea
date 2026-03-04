// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"bufio"
	"bytes"
	"fmt"
	gohtml "html"
	"html/template"
	"io"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
)

// don't index files larger than this many bytes for performance purposes
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

// RenderCodeSlowGuess tries to get a lexer by file name and language first,
// if not found, it will try to guess the lexer by code content, which is slow (more than several hundreds of milliseconds).
func RenderCodeSlowGuess(fileName, language, code string) (output template.HTML, lexer chroma.Lexer, lexerDisplayName string) {
	// diff view newline will be passed as empty, change to literal '\n' so it can be copied
	// preserve literal newline in blame view
	if code == "" || code == "\n" {
		return "\n", nil, ""
	}

	if len(code) > sizeLimit {
		return template.HTML(template.HTMLEscapeString(code)), nil, ""
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

	htmlbuf := bytes.Buffer{}
	htmlw := bufio.NewWriter(&htmlbuf)

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		log.Error("Can't tokenize code: %v", err)
		return template.HTML(template.HTMLEscapeString(code))
	}
	// style not used for live site but need to pass something
	err = formatter.Format(htmlw, globalVars().githubStyles, iterator)
	if err != nil {
		log.Error("Can't format code: %v", err)
		return template.HTML(template.HTMLEscapeString(code))
	}

	_ = htmlw.Flush()
	// Chroma will add newlines for certain lexers in order to highlight them properly
	// Once highlighted, strip them here, so they don't cause copy/paste trouble in HTML output
	return template.HTML(strings.TrimSuffix(htmlbuf.String(), "\n"))
}

// RenderFullFile returns a slice of chroma syntax highlighted HTML lines of code and the matched lexer name
func RenderFullFile(fileName, language string, code []byte) ([]template.HTML, string, error) {
	if len(code) > sizeLimit {
		return RenderPlainText(code), "", nil
	}

	formatter := html.New(html.WithClasses(true),
		html.WithLineNumbers(false),
		html.PreventSurroundingPre(true),
	)

	lexer := detectChromaLexerWithAnalyze(fileName, language, code)
	lexerName := formatLexerName(lexer.Config().Name)

	iterator, err := lexer.Tokenise(nil, string(code))
	if err != nil {
		return nil, "", fmt.Errorf("can't tokenize code: %w", err)
	}

	tokensLines := chroma.SplitTokensIntoLines(iterator.Tokens())
	htmlBuf := &bytes.Buffer{}

	lines := make([]template.HTML, 0, len(tokensLines))
	for _, tokens := range tokensLines {
		iterator = chroma.Literator(tokens...)
		err = formatter.Format(htmlBuf, globalVars().githubStyles, iterator)
		if err != nil {
			return nil, "", fmt.Errorf("can't format code: %w", err)
		}
		lines = append(lines, template.HTML(htmlBuf.String()))
		htmlBuf.Reset()
	}

	return lines, lexerName, nil
}

// RenderPlainText returns non-highlighted HTML for code
func RenderPlainText(code []byte) []template.HTML {
	r := bufio.NewReader(bytes.NewReader(code))
	m := make([]template.HTML, 0, bytes.Count(code, []byte{'\n'})+1)
	for {
		content, err := r.ReadString('\n')
		if err != nil && err != io.EOF {
			log.Error("failed to read string from buffer: %v", err)
			break
		}
		if content == "" && err == io.EOF {
			break
		}
		s := template.HTML(gohtml.EscapeString(content))
		m = append(m, s)
	}
	return m
}

func formatLexerName(name string) string {
	if name == "fallback" {
		return "Plaintext"
	}

	return util.ToTitleCaseNoLower(name)
}
