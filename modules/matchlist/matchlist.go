// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package matchlist

import (
	"strings"

	"github.com/gobwas/glob"
)

// Matchlist represents a block or allow list
type Matchlist struct {
	ruleGlobs []glob.Glob
}

// NewMatchlist creates a new block or allow list
func NewMatchlist(rules ...string) (*Matchlist, error) {
	for i := range rules {
		rules[i] = strings.ToLower(rules[i])
	}
	list := Matchlist{
		ruleGlobs: make([]glob.Glob, 0, len(rules)),
	}

	for _, rule := range rules {
		rg, err := glob.Compile(rule)
		if err != nil {
			return nil, err
		}
		list.ruleGlobs = append(list.ruleGlobs, rg)
	}

	return &list, nil
}

// Match will matches
func (b *Matchlist) Match(u string) bool {
	for _, r := range b.ruleGlobs {
		if r.Match(u) {
			return true
		}
	}
	return false
}
