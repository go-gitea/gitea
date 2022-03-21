// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package vars

import (
	"fmt"
	"strings"
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

// Expand replaces all variables like {var} to match
func Expand(template string, match map[string]string, subs ...string) (string, error) {
	var (
		buf   strings.Builder
		key   strings.Builder
		enter bool
	)
	for _, c := range template {
		switch {
		case c == '{':
			if enter {
				return "", ErrWrongSyntax{
					Template: template,
				}
			}
			enter = true
		case c == '}':
			if !enter {
				return "", ErrWrongSyntax{
					Template: template,
				}
			}
			if key.Len() == 0 {
				return "", ErrWrongSyntax{
					Template: template,
				}
			}

			if len(match) == 0 {
				return "", ErrNoMatchedVar{
					Template: template,
					Var:      key.String(),
				}
			}

			v, ok := match[key.String()]
			if !ok {
				if len(subs) == 0 {
					return "", ErrNoMatchedVar{
						Template: template,
						Var:      key.String(),
					}
				}
				v = subs[0]
			}

			if _, err := buf.WriteString(v); err != nil {
				return "", err
			}
			key.Reset()

			enter = false
		case enter:
			if _, err := key.WriteRune(c); err != nil {
				return "", err
			}
		default:
			if _, err := buf.WriteRune(c); err != nil {
				return "", err
			}
		}
	}
	return buf.String(), nil
}
