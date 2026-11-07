// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conan

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
)

var globalVars = sync.OnceValue(func() (ret struct {
	patternAuthor      *regexp.Regexp
	patternHomepage    *regexp.Regexp
	patternURL         *regexp.Regexp
	patternLicense     *regexp.Regexp
	patternDescription *regexp.Regexp
	patternTopics      *regexp.Regexp
	patternTopicList   *regexp.Regexp
	namePattern        *regexp.Regexp
	revisionPattern    *regexp.Regexp
},
) {
	ret.patternAuthor = compilePattern("author")
	ret.patternHomepage = compilePattern("homepage")
	ret.patternURL = compilePattern("url")
	ret.patternLicense = compilePattern("license")
	ret.patternDescription = compilePattern("description")
	ret.patternTopics = regexp.MustCompile(`(?im)^\s*topics\s*=\s*\((.+)\)`)
	ret.patternTopicList = regexp.MustCompile(`\s*['"](.+?)['"]\s*,?`)
	ret.namePattern = regexp.MustCompile(fmt.Sprintf(`^[a-zA-Z0-9_][a-zA-Z0-9_\+\.-]{%d,%d}$`, minChars-1, maxChars-1))
	ret.revisionPattern = regexp.MustCompile(fmt.Sprintf(`^[a-zA-Z0-9]{1,%d}$`, maxChars))
	return ret
})

func compilePattern(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?im)^\s*` + name + `\s*=\s*['"\(](.+)['"\)]`)
}

func ParseConanfile(r io.Reader) (*Metadata, error) {
	buf, err := io.ReadAll(io.LimitReader(r, 1<<20))
	if err != nil {
		return nil, err
	}

	metadata := &Metadata{}

	m := globalVars().patternAuthor.FindSubmatch(buf)
	if len(m) > 1 && len(m[1]) > 0 {
		metadata.Author = string(m[1])
	}
	m = globalVars().patternHomepage.FindSubmatch(buf)
	if len(m) > 1 && len(m[1]) > 0 {
		metadata.ProjectURL = string(m[1])
	}
	m = globalVars().patternURL.FindSubmatch(buf)
	if len(m) > 1 && len(m[1]) > 0 {
		metadata.RepositoryURL = string(m[1])
	}
	m = globalVars().patternLicense.FindSubmatch(buf)
	if len(m) > 1 && len(m[1]) > 0 {
		metadata.License = strings.ReplaceAll(strings.ReplaceAll(string(m[1]), "'", ""), "\"", "")
	}
	m = globalVars().patternDescription.FindSubmatch(buf)
	if len(m) > 1 && len(m[1]) > 0 {
		metadata.Description = string(m[1])
	}
	m = globalVars().patternTopics.FindSubmatch(buf)
	if len(m) > 1 && len(m[1]) > 0 {
		m2 := globalVars().patternTopicList.FindAllSubmatch(m[1], -1)
		if len(m2) > 0 {
			metadata.Keywords = make([]string, 0, len(m2))
			for _, g := range m2 {
				if len(g) > 1 {
					metadata.Keywords = append(metadata.Keywords, string(g[1]))
				}
			}
		}
	}
	return metadata, nil
}
