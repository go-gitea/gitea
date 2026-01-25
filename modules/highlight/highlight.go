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
	"path"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/analyze"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// don't index files larger than this many bytes for performance purposes
const sizeLimit = 1024 * 1024

type globalVarsType struct {
	inited           bool
	highlightMapping map[string]string
	githubStyles     *chroma.Style
}

var (
	globalVarsMu  sync.Mutex
	globalVarsPtr *globalVarsType
)

func globalVars() *globalVarsType {
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

// toChromaLanguage normalizes language names to Chroma-compatible names
func toChromaLanguage(entryLang string) string {
	lang, _, _ := strings.Cut(entryLang, "?") // maybe, the value from gitattributes might contain `?` parameters?
	switch lang {
	case "F#":
		return "FSharp"
	}
	return lang
}

func GetChromaLexer(fileName, lang string, code []byte) chroma.Lexer {
	// lexers.Get is slow if the language name can't be matched directly: it does extra "Match" call to iterate all lexers
	var lexer chroma.Lexer
	if lang != "" {
		lexer = lexers.Get(toChromaLanguage(lang))
	}

	if lexer == nil {
		fileExt := path.Ext(fileName)
		if val, ok := globalVars().highlightMapping[fileExt]; ok {
			lexer = lexers.Get(toChromaLanguage(val)) // use mapped value to find lexer
		}
	}

	if lexer == nil {
		lexer = lexers.Match(fileName) // Chroma will search by its basename and extname
	}

	if lexer == nil && code != nil {
		// analyze.GetCodeLanguage is slower, it iterates many rules to detect language from content
		enryLanguage := analyze.GetCodeLanguage(fileName, code)
		lexer = lexers.Get(toChromaLanguage(enryLanguage))
	}

	if lexer == nil {
		lexer = lexers.Fallback
	}
	return lexer
}

// Code returns an HTML version of code string with chroma syntax highlighting classes and the matched lexer name
func Code(fileName, language, code string) (output template.HTML, lexerName string) {
	// diff view newline will be passed as empty, change to literal '\n' so it can be copied
	// preserve literal newline in blame view
	if code == "" || code == "\n" {
		return "\n", ""
	}

	if len(code) > sizeLimit {
		return template.HTML(template.HTMLEscapeString(code)), ""
	}

	lexer := GetChromaLexer(fileName, language, nil) // don't use content to detect, it is too slow
	return CodeFromLexer(lexer, code), formatLexerName(lexer.Config().Name)
}

// CodeFromLexer returns a HTML version of code string with chroma syntax highlighting classes
func CodeFromLexer(lexer chroma.Lexer, code string) template.HTML {
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

// File returns a slice of chroma syntax highlighted HTML lines of code and the matched lexer name
func File(fileName, language string, code []byte) ([]template.HTML, string, error) {
	if len(code) > sizeLimit {
		return PlainText(code), "", nil
	}

	formatter := html.New(html.WithClasses(true),
		html.WithLineNumbers(false),
		html.PreventSurroundingPre(true),
	)

	lexer := GetChromaLexer(fileName, language, code)
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

// PlainText returns non-highlighted HTML for code
func PlainText(code []byte) []template.HTML {
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
