// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package highlight

import (
	"bufio"
	"bytes"
	"fmt"
	gohtml "html"
	"path/filepath"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/analyze"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/formatters/html"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	lru "github.com/hashicorp/golang-lru"
)

// don't index files larger than this many bytes for performance purposes
const sizeLimit = 1000000

var (
	// For custom user mapping
	highlightMapping = map[string]string{}

	once sync.Once

	cache *lru.TwoQueueCache
)

// NewContext loads custom highlight map from local config
func NewContext() {
	once.Do(func() {
		keys := setting.Cfg.Section("highlight.mapping").Keys()
		for i := range keys {
			highlightMapping[keys[i].Name()] = keys[i].Value()
		}

		// The size 512 is simply a conservative rule of thumb
		c, err := lru.New2Q(512)
		if err != nil {
			panic(fmt.Sprintf("failed to initialize LRU cache for highlighter: %s", err))
		}
		cache = c
	})
}

// Code returns a HTML version of code string with chroma syntax highlighting classes
func Code(fileName, code string) string {
	NewContext()

	// diff view newline will be passed as empty, change to literal \n so it can be copied
	// preserve literal newline in blame view
	if code == "" || code == "\n" {
		return "\n"
	}

	if len(code) > sizeLimit {
		return code
	}

	var lexer chroma.Lexer
	if val, ok := highlightMapping[filepath.Ext(fileName)]; ok {
		//use mapped value to find lexer
		lexer = lexers.Get(val)
	}

	if lexer == nil {
		if l, ok := cache.Get(fileName); ok {
			lexer = l.(chroma.Lexer)
		}
	}

	if lexer == nil {
		lexer = lexers.Match(fileName)
		if lexer == nil {
			lexer = lexers.Fallback
		}
		cache.Add(fileName, lexer)
	}
	return CodeFromLexer(lexer, code)
}

// CodeFromLexer returns a HTML version of code string with chroma syntax highlighting classes
func CodeFromLexer(lexer chroma.Lexer, code string) string {
	formatter := html.New(html.WithClasses(true),
		html.WithLineNumbers(false),
		html.PreventSurroundingPre(true),
	)

	htmlbuf := bytes.Buffer{}
	htmlw := bufio.NewWriter(&htmlbuf)

	iterator, err := lexer.Tokenise(nil, string(code))
	if err != nil {
		log.Error("Can't tokenize code: %v", err)
		return code
	}
	// style not used for live site but need to pass something
	err = formatter.Format(htmlw, styles.GitHub, iterator)
	if err != nil {
		log.Error("Can't format code: %v", err)
		return code
	}

	htmlw.Flush()
	// Chroma will add newlines for certain lexers in order to highlight them properly
	// Once highlighted, strip them here so they don't cause copy/paste trouble in HTML output
	return strings.TrimSuffix(htmlbuf.String(), "\n")
}

// File returns a slice of chroma syntax highlighted lines of code
func File(numLines int, fileName string, code []byte) []string {
	NewContext()

	if len(code) > sizeLimit {
		return plainText(string(code), numLines)
	}
	formatter := html.New(html.WithClasses(true),
		html.WithLineNumbers(false),
		html.PreventSurroundingPre(true),
	)

	if formatter == nil {
		log.Error("Couldn't create chroma formatter")
		return plainText(string(code), numLines)
	}

	htmlbuf := bytes.Buffer{}
	htmlw := bufio.NewWriter(&htmlbuf)

	var lexer chroma.Lexer
	if val, ok := highlightMapping[filepath.Ext(fileName)]; ok {
		lexer = lexers.Get(val)
	}

	if lexer == nil {
		language := analyze.GetCodeLanguage(fileName, code)

		lexer = lexers.Get(language)
		if lexer == nil {
			lexer = lexers.Match(fileName)
			if lexer == nil {
				lexer = lexers.Fallback
			}
		}
	}

	iterator, err := lexer.Tokenise(nil, string(code))
	if err != nil {
		log.Error("Can't tokenize code: %v", err)
		return plainText(string(code), numLines)
	}

	err = formatter.Format(htmlw, styles.GitHub, iterator)
	if err != nil {
		log.Error("Can't format code: %v", err)
		return plainText(string(code), numLines)
	}

	htmlw.Flush()
	finalNewLine := false
	if len(code) > 0 {
		finalNewLine = code[len(code)-1] == '\n'
	}

	m := make([]string, 0, numLines)
	for _, v := range strings.SplitN(htmlbuf.String(), "\n", numLines) {
		content := string(v)
		//need to keep lines that are only \n so copy/paste works properly in browser
		if content == "" {
			content = "\n"
		} else if content == `</span><span class="w">` {
			content += "\n</span>"
		}
		content = strings.TrimSuffix(content, `<span class="w">`)
		content = strings.TrimPrefix(content, `</span>`)
		m = append(m, content)
	}
	if finalNewLine {
		m = append(m, "<span class=\"w\">\n</span>")
	}

	return m
}

// return unhiglighted map
func plainText(code string, numLines int) []string {
	m := make([]string, 0, numLines)
	for _, v := range strings.SplitN(string(code), "\n", numLines) {
		content := string(v)
		//need to keep lines that are only \n so copy/paste works properly in browser
		if content == "" {
			content = "\n"
		}
		m = append(m, gohtml.EscapeString(content))
	}
	return m
}
