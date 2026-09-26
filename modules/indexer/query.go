// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package indexer

import (
	"slices"
	"strings"
)

// SearchQuery is a search query in the GitHub syntax: keywords with "name:value" qualifiers
type SearchQuery struct {
	Keyword    string
	Qualifiers []SearchQualifier
}

type SearchQualifier struct {
	Name    string
	Value   string
	Exclude bool // "-name:value"
}

// ParseSearchQuery separates the qualifiers with the given names from the keywords.
// Only known names are qualifiers so that code like "std::string" stays a keyword, and quoted terms are never qualifiers.
func ParseSearchQuery(q string, names ...string) *SearchQuery {
	query := &SearchQuery{}
	var keywords []string
	for _, term := range splitSearchTerms(q) {
		if term.text == "" {
			continue
		}
		if !term.quoted {
			text, exclude := strings.CutPrefix(term.text, "-")
			if name, value, ok := strings.Cut(text, ":"); ok && value != "" && slices.Contains(names, name) {
				query.Qualifiers = append(query.Qualifiers, SearchQualifier{Name: name, Value: value, Exclude: exclude})
				continue
			}
		}
		keywords = append(keywords, term.text)
	}
	query.Keyword = strings.Join(keywords, " ")
	return query
}

// Values returns the values of the included qualifiers with the name
func (q *SearchQuery) Values(name string) (values []string) {
	for _, qualifier := range q.Qualifiers {
		if qualifier.Name == name && !qualifier.Exclude {
			values = append(values, qualifier.Value)
		}
	}
	return values
}

type searchTerm struct {
	text   string
	quoted bool
}

// splitSearchTerms splits at whitespace outside of double quotes, removing the quotes; `\"` is a literal quote
func splitSearchTerms(q string) (terms []searchTerm) {
	var sb strings.Builder
	inTerm, inQuotes, quoted := false, false, false
	endTerm := func() {
		if inTerm {
			terms = append(terms, searchTerm{text: sb.String(), quoted: quoted})
		}
		sb.Reset()
		inTerm, quoted = false, false
	}
	for i := 0; i < len(q); i++ {
		c := q[i]
		switch {
		case c == '\\' && i+1 < len(q) && q[i+1] == '"':
			sb.WriteByte('"')
			inTerm = true
			i++
		case c == '"':
			quoted = quoted || !inTerm
			inQuotes = !inQuotes
			inTerm = true
		case !inQuotes && (c == ' ' || c == '\t' || c == '\n' || c == '\r'):
			endTerm()
		default:
			sb.WriteByte(c)
			inTerm = true
		}
	}
	endTerm()
	return terms
}
