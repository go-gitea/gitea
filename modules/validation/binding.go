// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package validation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/go-macaron/binding"
)

const (
	// ERR_GIT_REF_NAME is git reference name error
	ERR_GIT_REF_NAME = "GitRefNameError"
)

var (
	// GitRefNamePattern is regular expression wirh unallowed characters in git reference name
	GitRefNamePattern = regexp.MustCompile("[^\\d\\w-_\\./]")
)

// AddBindingRules adds additional binding rules
func AddBindingRules() {
	addGitRefNameBindingRule()
}

func addGitRefNameBindingRule() {
	// Git refname validation rule
	binding.AddRule(&binding.Rule{
		IsMatch: func(rule string) bool {
			return strings.HasPrefix(rule, "GitRefName")
		},
		IsValid: func(errs binding.Errors, name string, val interface{}) (bool, binding.Errors) {
			str := fmt.Sprintf("%v", val)

			if GitRefNamePattern.MatchString(str) {
				errs.Add([]string{name}, ERR_GIT_REF_NAME, "GitRefName")
				return false, errs
			}
			// Additional rules as described at https://www.kernel.org/pub/software/scm/git/docs/git-check-ref-format.html
			if strings.HasPrefix(str, "/") || strings.HasSuffix(str, "/") ||
				strings.HasPrefix(str, ".") || strings.HasSuffix(str, ".") ||
				strings.HasSuffix(str, ".lock") ||
				strings.Contains(str, "..") || strings.Contains(str, "//") {
				errs.Add([]string{name}, ERR_GIT_REF_NAME, "GitRefName")
				return false, errs
			}

			return true, errs
		},
	})
}
