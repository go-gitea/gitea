// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package conan

import (
	"io"
	"regexp"
	"strings"
)

var (
	patternAuthor      = compilePattern("author")
	patternHomepage    = compilePattern("homepage")
	patternURL         = compilePattern("url")
	patternLicense     = compilePattern("license")
	patternDescription = compilePattern("description")
	patternTopics      = regexp.MustCompile(`(?im)^\s*topics\s*=\s*\((.+)\)`)
	patternTopicList   = regexp.MustCompile(`\s*['"](.+?)['"]\s*,?`)
)

func compilePattern(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?im)^\s*` + name + `\s*=\s*['"\(](.+)['"\)]`)
}

func ParseConanfile(r io.Reader) (*Metadata, error) {
	buf, err := io.ReadAll(io.LimitReader(r, 1<<20))
	if err != nil {
		return nil, err
	}

	metadata := &Metadata{}

	m := patternAuthor.FindSubmatch(buf)
	if len(m) > 1 && len(m[1]) > 0 {
		metadata.Author = string(m[1])
	}
	m = patternHomepage.FindSubmatch(buf)
	if len(m) > 1 && len(m[1]) > 0 {
		metadata.ProjectURL = string(m[1])
	}
	m = patternURL.FindSubmatch(buf)
	if len(m) > 1 && len(m[1]) > 0 {
		metadata.RepositoryURL = string(m[1])
	}
	m = patternLicense.FindSubmatch(buf)
	if len(m) > 1 && len(m[1]) > 0 {
		metadata.License = strings.ReplaceAll(strings.ReplaceAll(string(m[1]), "'", ""), "\"", "")
	}
	m = patternDescription.FindSubmatch(buf)
	if len(m) > 1 && len(m[1]) > 0 {
		metadata.Description = string(m[1])
	}
	m = patternTopics.FindSubmatch(buf)
	if len(m) > 1 && len(m[1]) > 0 {
		m2 := patternTopicList.FindAllSubmatch(m[1], -1)
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
