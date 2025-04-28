// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package vars

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ErrWrongSyntax represents a wrong syntax with a template
type ErrWrongSyntax struct {
	Template string
}

func (err ErrWrongSyntax) Error() string {
	return "wrong syntax found in " + err.Template
}

// ErrVarMissing represents an error that no matched variable
type ErrVarMissing struct {
	Template string
	Var      string
}

func (err ErrVarMissing) Error() string {
	return fmt.Sprintf("the variable %s is missing for %s", err.Var, err.Template)
}

// Expand replaces all variables like {var} by `vars` map, it always returns the expanded string regardless of errors
// if error occurs, the error part doesn't change and is returned as it is.
func Expand(template string, vars map[string]string) (string, error) {
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
			if template[posEnd] == '}' {
				posEnd++
				break
			} // in the future, if we need to support escape chars, we can do: if (isEscapeChar) { posEnd+=2 }
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
				if val, ok := vars[key]; ok {
					buf.WriteString(val)
				} else {
					// write the non-existing var as it is
					buf.WriteString(part)
					err = ErrVarMissing{Template: template, Var: key}
				}
			}
		}
	}

	return buf.String(), err
}
