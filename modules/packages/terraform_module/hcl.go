// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform_module

import (
	"bytes"
	"strconv"
	"strings"

	"gitea.dev/modules/container"
)

var hclIndexedBlocks = container.SetOf("variable", "output", "terraform", "required_providers") // bodies of other blocks are skipped

type hclBlock struct {
	typ    string
	labels []string
	attrs  map[string]string // attribute name -> expression source
	blocks []*hclBlock
}

// hclScanner reads HCL block structure without evaluating expressions, nesting is capped to bound recursion
type hclScanner struct {
	src     []byte
	pos     int
	nesting int
}

const hclMaxNesting = 32

func (s *hclScanner) peek(offset int) byte {
	if s.pos+offset < len(s.src) {
		return s.src[s.pos+offset]
	}
	return 0
}

func (s *hclScanner) body(depth int) *hclBlock {
	body := &hclBlock{attrs: map[string]string{}}
	for {
		s.skipSpace(true)
		if s.pos >= len(s.src) {
			return body
		}
		if s.src[s.pos] == '}' {
			s.pos++
			return body
		}
		start := s.pos
		name := s.ident()
		s.skipSpace(false)
		if name != "" && s.peek(0) == '=' {
			s.pos++
			s.skipSpace(false)
			exprStart := s.pos
			body.attrs[name] = string(s.src[exprStart:s.skipExpr()])
			continue
		}
		var labels []string
		for name != "" && s.peek(0) != '{' {
			labelStart := s.pos
			if s.peek(0) == '"' {
				s.skipString()
				labels = append(labels, hclString(string(s.src[labelStart:s.pos])))
			} else if label := s.ident(); label != "" {
				labels = append(labels, label)
			} else {
				break
			}
			s.skipSpace(false)
		}
		if name == "" || s.peek(0) != '{' || depth >= 2 || !hclIndexedBlocks.Contains(name) { // only terraform.required_providers needs two levels
			s.skipExpr() // skips unparsed nested blocks and anything malformed
			if s.pos == start {
				s.pos++
			}
			continue
		}
		s.pos++
		block := s.body(depth + 1)
		block.typ, block.labels = name, labels
		body.blocks = append(body.blocks, block)
	}
}

// skipExpr advances to the newline, comma or unbalanced closing bracket that ends an expression and returns where its source ends
func (s *hclScanner) skipExpr() (end int) {
	end = s.pos
	for depth := 0; s.pos < len(s.src); {
		switch c := s.src[s.pos]; {
		case c == '"':
			s.skipString()
		case c == '<' && s.skipHeredoc():
		case s.skipComment():
			continue
		case (c == '\n' || c == ',') && depth == 0:
			return end
		case c == '}' || c == ']' || c == ')':
			if depth == 0 {
				return end
			}
			depth--
			s.pos++
		case c == '{' || c == '[' || c == '(':
			depth++
			s.pos++
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			s.pos++
			continue
		default:
			s.pos++
		}
		end = s.pos
	}
	return end
}

func (s *hclScanner) skipString() {
	for s.pos++; s.pos < len(s.src); s.pos++ {
		switch c := s.src[s.pos]; {
		case c == '\\' || (c == '$' || c == '%') && s.peek(1) == c: // escape sequence
			s.pos++
		case c == '"':
			s.pos++
			return
		case c == '\n':
			return
		case (c == '$' || c == '%') && s.peek(1) == '{':
			if s.nesting++; s.nesting > hclMaxNesting {
				return
			}
			s.pos += 2
			s.skipExpr()
			s.nesting--
			if s.peek(0) != '}' {
				return
			}
		}
	}
}

func (s *hclScanner) skipHeredoc() bool {
	start := s.pos
	if s.peek(1) != '<' {
		return false
	}
	s.pos += 2
	if s.peek(0) == '-' {
		s.pos++
	}
	delim := s.ident()
	if s.peek(0) == '\r' {
		s.pos++
	}
	if delim == "" || s.peek(0) != '\n' {
		s.pos = start
		return false
	}
	for s.pos < len(s.src) {
		lineStart := s.pos + 1
		s.pos = len(s.src)
		if lineEnd := bytes.IndexByte(s.src[lineStart:], '\n'); lineEnd >= 0 {
			s.pos = lineStart + lineEnd
		}
		if string(bytes.TrimSpace(s.src[lineStart:s.pos])) == delim {
			break
		}
	}
	return true
}

func (s *hclScanner) skipComment() bool {
	switch {
	case s.peek(0) == '#' || s.peek(0) == '/' && s.peek(1) == '/':
		for s.pos < len(s.src) && s.src[s.pos] != '\n' {
			s.pos++
		}
	case s.peek(0) == '/' && s.peek(1) == '*':
		if end := bytes.Index(s.src[s.pos+2:], []byte("*/")); end >= 0 {
			s.pos += end + 4
		} else {
			s.pos = len(s.src)
		}
	default:
		return false
	}
	return true
}

func (s *hclScanner) skipSpace(newlines bool) {
	for s.pos < len(s.src) {
		switch c := s.src[s.pos]; {
		case c == ' ' || c == '\t' || c == '\r' || newlines && (c == '\n' || c == ','):
			s.pos++
		case !s.skipComment():
			return
		}
	}
}

func (s *hclScanner) ident() string {
	start := s.pos
	for ; s.pos < len(s.src); s.pos++ {
		c := s.src[s.pos]
		if !(c == '_' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || s.pos > start && (c == '-' || '0' <= c && c <= '9')) {
			break
		}
	}
	return string(s.src[start:s.pos])
}

var (
	hclWithoutEscapedTemplates = strings.NewReplacer("$${", "", "%%{", "")
	hclUnescapeTemplates       = strings.NewReplacer("$${", "${", "%%{", "%{")
)

// hclString decodes a string literal or heredoc, returning "" for expressions and templates that need evaluation
func hclString(expr string) string {
	if strings.HasPrefix(expr, "<<") {
		_, content, _ := strings.Cut(expr, "\n")
		return content[:strings.LastIndexByte(content, '\n')+1]
	}
	if unescaped := hclWithoutEscapedTemplates.Replace(expr); strings.Contains(unescaped, "${") || strings.Contains(unescaped, "%{") {
		return ""
	}
	s, err := strconv.Unquote(expr)
	if err != nil {
		return ""
	}
	return hclUnescapeTemplates.Replace(s)
}
