// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package vars

import (
	"fmt"
	"strings"
)

// ErrWrongSyntax represents a syntax error within a template
type ErrWrongSyntax struct {
	Template string
	Reason   string
}

func (err ErrWrongSyntax) Error() string {
	return fmt.Sprintf("Wrong syntax found in %s: %s", err.Template, err.Reason)
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

// Expand replaces all variables like {var} to match
func Expand(template string, match map[string]string, subs ...string) (string, error) {
	var (
		buf         strings.Builder
		keyStartPos = -1
	)
	for i, c := range template {
		switch {
		case c == '{':
			if keyStartPos > -1 {
				return "", ErrWrongSyntax{
					Template: template,
					Reason:   "\"{\" is not allowed to occur again before closing the variable",
				}
			}
			keyStartPos = i
		case c == '}':
			if keyStartPos == -1 {
				return "", ErrWrongSyntax{
					Template: template,
					Reason:   "\"}\" can only occur after an opening \"{\"",
				}
			}
			if i-keyStartPos <= 1 {
				return "", ErrWrongSyntax{
					Template: template,
					Reason:   "the empty variable (\"{}\") is not allowed",
				}
			}

			if len(match) == 0 {
				return "", ErrNoMatchedVar{
					Template: template,
					Var:      template[keyStartPos+1 : i],
				}
			}

			v, ok := match[template[keyStartPos+1:i]]
			if !ok {
				if len(subs) == 0 {
					return "", ErrNoMatchedVar{
						Template: template,
						Var:      template[keyStartPos+1 : i],
					}
				}
				v = subs[0]
			}

			if _, err := buf.WriteString(v); err != nil {
				return "", err
			}

			keyStartPos = -1
		case keyStartPos > -1:
		default:
			if _, err := buf.WriteRune(c); err != nil {
				return "", err
			}
		}
	}
	return buf.String(), nil
}
