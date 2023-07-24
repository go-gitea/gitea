// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"bytes"
	"errors"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

func isYAMLSeparator(line []byte) bool {
	idx := 0
	for ; idx < len(line); idx++ {
		if line[idx] >= utf8.RuneSelf {
			r, sz := utf8.DecodeRune(line[idx:])
			if !unicode.IsSpace(r) {
				return false
			}
			idx += sz
			continue
		}
		if line[idx] != ' ' {
			break
		}
	}
	dashCount := 0
	for ; idx < len(line); idx++ {
		if line[idx] != '-' {
			break
		}
		dashCount++
	}
	if dashCount < 3 {
		return false
	}
	for ; idx < len(line); idx++ {
		if line[idx] >= utf8.RuneSelf {
			r, sz := utf8.DecodeRune(line[idx:])
			if !unicode.IsSpace(r) {
				return false
			}
			idx += sz
			continue
		}
		if line[idx] != ' ' {
			return false
		}
	}
	return true
}

// ExtractMetadata consumes a markdown file, parses YAML frontmatter,
// and returns the frontmatter metadata separated from the markdown content
func ExtractMetadata(contents string, out any) (string, error) {
	body, err := ExtractMetadataBytes([]byte(contents), out)
	return string(body), err
}

// ExtractMetadata consumes a markdown file, parses YAML frontmatter,
// and returns the frontmatter metadata separated from the markdown content
func ExtractMetadataBytes(contents []byte, out any) ([]byte, error) {
	var front, body []byte

	start, end := 0, len(contents)
	idx := bytes.IndexByte(contents[start:], '\n')
	if idx >= 0 {
		end = start + idx
	}
	line := contents[start:end]

	if !isYAMLSeparator(line) {
		return contents, errors.New("frontmatter must start with a separator line")
	}
	frontMatterStart := end + 1
	for start = frontMatterStart; start < len(contents); start = end + 1 {
		end = len(contents)
		idx := bytes.IndexByte(contents[start:], '\n')
		if idx >= 0 {
			end = start + idx
		}
		line := contents[start:end]
		if isYAMLSeparator(line) {
			front = contents[frontMatterStart:start]
			if end+1 < len(contents) {
				body = contents[end+1:]
			}
			break
		}
	}

	if len(front) == 0 {
		return contents, errors.New("could not determine metadata")
	}

	if err := yaml.Unmarshal(front, out); err != nil {
		return contents, err
	}
	return body, nil
}
