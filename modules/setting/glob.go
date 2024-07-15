// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "github.com/gobwas/glob"

type GlobMatcher struct {
	compiledGlob  glob.Glob
	patternString string
}

var _ glob.Glob = (*GlobMatcher)(nil)

func (g *GlobMatcher) Match(s string) bool {
	return g.compiledGlob.Match(s)
}

func (g *GlobMatcher) PatternString() string {
	return g.patternString
}

func GlobMatcherCompile(pattern string, separators ...rune) (*GlobMatcher, error) {
	g, err := glob.Compile(pattern, separators...)
	if err != nil {
		return nil, err
	}
	return &GlobMatcher{
		compiledGlob:  g,
		patternString: pattern,
	}, nil
}
