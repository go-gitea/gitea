// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package indexer

import (
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-enry/go-enry/v2"
)

type SearchTermKind int

const (
	SearchTermLiteral SearchTermKind = iota // case-insensitive literal
	SearchTermRegexp                        // case-insensitive line regexp unless (?-i)
)

type SearchTerm struct {
	Kind SearchTermKind
	Text string
}

type SearchQuery struct {
	Terms    []SearchTerm
	Path     string
	Language string
	Repos    []string
	Owners   []string

	withoutLanguage string
}

type SearchQueryError struct {
	Message string
}

func (e *SearchQueryError) Error() string {
	return e.Message
}

func ParseSearchQuery(q string) (*SearchQuery, error) {
	query := &SearchQuery{}
	var tokens []string
	for remaining := strings.TrimLeftFunc(q, unicode.IsSpace); remaining != ""; remaining = strings.TrimLeftFunc(remaining, unicode.IsSpace) {
		end := strings.IndexFunc(remaining, unicode.IsSpace)
		if end < 0 {
			end = len(remaining)
		}
		name, _, hasColon := strings.Cut(remaining[:end], ":")
		qualifier := false
		if hasColon {
			switch strings.TrimPrefix(name, "-") {
			case "path", "language", "repo", "user", "org":
				if strings.HasPrefix(name, "-") {
					return nil, &SearchQueryError{Message: "Negated qualifiers are not supported"}
				}
				qualifier = true
			case "content", "symbol", "is", "in", "filename", "extension":
				return nil, &SearchQueryError{Message: "Unsupported qualifier"}
			}
		}
		start := 0
		if qualifier {
			start = len(name) + 1
		}
		text := remaining[start:end]
		quoted := strings.HasPrefix(text, `"`)
		if quoted {
			var err error
			text, end, err = parseSearchPhrase(remaining, start)
			if err != nil {
				return nil, err
			}
		}
		token := remaining[:end]
		remaining = remaining[end:]
		if strings.ContainsAny(text, "\n\x00") {
			return nil, &SearchQueryError{Message: "Terms must not contain LF or NUL"}
		}
		if qualifier {
			if text == "" {
				return nil, &SearchQueryError{Message: "Empty qualifier value"}
			}
			switch name {
			case "path":
				if query.Path != "" {
					return nil, &SearchQueryError{Message: "Repeated path qualifier"}
				}
				if strings.HasPrefix(text, "/") || slices.Contains(strings.Split(text, "/"), "..") || (!quoted && strings.ContainsAny(text, "*?[")) {
					return nil, &SearchQueryError{Message: "Invalid path qualifier"}
				}
				query.Path = path.Clean(text)
				if query.Path == "." {
					return nil, &SearchQueryError{Message: "Invalid path qualifier"}
				}
			case "language":
				if query.Language != "" {
					return nil, &SearchQueryError{Message: "Repeated language qualifier"}
				}
				var ok bool
				query.Language, ok = enry.GetLanguageByAlias(text)
				if !ok {
					return nil, &SearchQueryError{Message: "Unknown language"}
				}
			case "repo":
				parts := strings.Split(text, "/")
				if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
					return nil, &SearchQueryError{Message: "Repository must be OWNER/NAME"}
				}
				query.Repos = append(query.Repos, text)
			case "user", "org":
				query.Owners = append(query.Owners, text)
			}
		} else {
			term := SearchTerm{Kind: SearchTermLiteral, Text: text}
			if !quoted {
				switch text {
				case "AND", "OR", "NOT":
					return nil, &SearchQueryError{Message: "Boolean operators are not supported"}
				}
				if strings.HasPrefix(text, "/") && strings.HasSuffix(text, "/") {
					backslashes := 0
					for i := len(text) - 2; i >= 0 && text[i] == '\\'; i-- {
						backslashes++
					}
					if backslashes%2 == 0 && len(text) >= 2 {
						term.Kind = SearchTermRegexp
						var err error
						term.Text, err = parseSearchRegexp(text[1 : len(text)-1])
						if err != nil {
							return nil, err
						}
					}
				}
			}
			if len(term.Text) > 256 {
				return nil, &SearchQueryError{Message: "Term exceeds 256 bytes"}
			}
			query.Terms = append(query.Terms, term)
			if len(query.Terms) > 16 {
				return nil, &SearchQueryError{Message: "Query exceeds 16 terms"}
			}
		}
		if !qualifier || name != "language" {
			tokens = append(tokens, token)
		}
	}
	if len(query.Terms) == 0 {
		return nil, &SearchQueryError{Message: "Query must contain a search term"}
	}
	query.withoutLanguage = strings.Join(tokens, " ")
	return query, nil
}

func parseSearchPhrase(token string, start int) (string, int, error) {
	var phrase strings.Builder
	for i := start + 1; i < len(token); i++ {
		switch token[i] {
		case '"':
			if phrase.Len() == 0 {
				return "", 0, &SearchQueryError{Message: "Empty quoted value"}
			}
			next, _ := utf8.DecodeRuneInString(token[i+1:])
			if i+1 < len(token) && !unicode.IsSpace(next) {
				return "", 0, &SearchQueryError{Message: "Unexpected text after closing quote"}
			}
			return phrase.String(), i + 1, nil
		case '\\':
			if i+1 < len(token) && (token[i+1] == '"' || token[i+1] == '\\') {
				i++
			}
		}
		phrase.WriteByte(token[i])
	}
	return "", 0, &SearchQueryError{Message: "Unterminated quote"}
}

func parseSearchRegexp(pattern string) (string, error) {
	if pattern == "" || len(pattern) > 256 {
		return "", &SearchQueryError{Message: "Regexp must contain 1 to 256 bytes"}
	}
	var decoded strings.Builder
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '\\' && i+1 < len(pattern) {
			i++
			if pattern[i] != '/' {
				decoded.WriteByte('\\')
			}
		}
		decoded.WriteByte(pattern[i])
	}
	if _, err := regexp.Compile(decoded.String()); err != nil {
		return "", &SearchQueryError{Message: "Invalid regexp"}
	}
	return decoded.String(), nil
}

func (q *SearchQuery) Literals() (literals []string) {
	for _, term := range q.Terms {
		if term.Kind == SearchTermLiteral {
			literals = append(literals, term.Text)
		}
	}
	return literals
}

func (q *SearchQuery) WithoutLanguage() string {
	return q.withoutLanguage
}
