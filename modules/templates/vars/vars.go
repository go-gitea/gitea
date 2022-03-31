// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package vars

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ErrWrongSyntax represents a wrong syntax with a tempate
type ErrWrongSyntax struct {
	Template string
}

func (err ErrWrongSyntax) Error() string {
	return fmt.Sprintf("Wrong syntax found in %s", err.Template)
}

// IsErrWrongSyntax returns true if the error is ErrWrongSyntax
func IsErrWrongSyntax(err error) bool {
	_, ok := err.(ErrWrongSyntax)
	return ok
}

// ErrNoMatchedVar represents an error that no matched vars
type ErrNoMatchedVar struct {
	Template string
	Var      string
}

func (err ErrNoMatchedVar) Error() string {
	return fmt.Sprintf("No matched variable %s found for %s", err.Var, err.Template)
}

// IsErrNoMatchedVar returns true if the error is ErrNoMatchedVar
func IsErrNoMatchedVar(err error) bool {
	_, ok := err.(ErrNoMatchedVar)
	return ok
}

// Expand replaces all variables like {var} to match, if error occurs,
// the error part doesn't change and is returned as it is.
func Expand(template string, match map[string]string) (string, error) {
	// in the future, if necessary, we can introduce some escape-char,
	// for example: it will use `#' as a reversed char, templates will use `{#{}` to do escape and output char '{'.
	var buf strings.Builder
	var err error

	posBegin := 0
	strLen := len(template)
	for posBegin < strLen {
		// find the next `{`
		pos := strings.IndexByte(template[posBegin:], '{')
		if pos == -1 {
			buf.WriteString(template[posBegin:])
			break
		}

		// copy texts between vars
		buf.WriteString(template[posBegin : posBegin+pos])

		// find the var between `{` and `}`/end
		posBegin += pos
		posEnd := posBegin + 1
		for posEnd < strLen {
			if template[posEnd] == '#' {
				// escape char, skip next
				posEnd += 2
				continue
			} else if template[posEnd] == '}' {
				posEnd++
				break
			}
			posEnd++
		}

		// the var part, it can be "{", "{}", "{..." or or "{...}"
		part := template[posBegin:posEnd]
		posBegin = posEnd
		if part == "{}" || part[len(part)-1] != '}' {
			// treat "{}" or "{..." as error
			err = ErrWrongSyntax{Template: template}
			buf.WriteString(part)
		} else {
			// now we get a valid key "{...}"
			key := part[1 : len(part)-1]
			keyFirst, _ := utf8.DecodeRuneInString(key)
			if unicode.IsSpace(keyFirst) || unicode.IsPunct(keyFirst) || unicode.IsControl(keyFirst) {
				// the if key doesn't start with a letter, then we do not treat it as a var now
				buf.WriteString(part)
			} else {
				// look up in the map
				if val, ok := match[key]; ok {
					buf.WriteString(val)
				} else {
					// write the non-existing var as it is
					buf.WriteString(part)
					err = ErrNoMatchedVar{Template: template, Var: key}
				}
			}
		}
	}

	return buf.String(), err
}
