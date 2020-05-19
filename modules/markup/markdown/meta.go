// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markdown

import (
	"strings"

	"gopkg.in/yaml.v2"
)

func isYAMLSeparator(line string) bool {
	line = strings.TrimSpace(line)
	for i := 0; i < len(line); i++ {
		if line[i] != '-' {
			return false
		}
	}
	return len(line) != 0
}

// ExtractMetadata consumes a markdown file, parses YAML frontmatter,
// and returns the frontmatter metadata separated from the markdown content
func ExtractMetadata(contents string) (map[string]interface{}, string, error) {
	var front, body []string
	var seps int
	lines := strings.Split(contents, "\n")
	for idx, line := range lines {
		if seps == 2 {
			body = append(body, lines[idx:]...)
			break
		}
		if isYAMLSeparator(line) {
			seps++
			continue
		}
		front = append(front, line)
	}

	var meta map[string]interface{}
	if err := yaml.Unmarshal([]byte(strings.Join(front, "\n")), &meta); err != nil {
		return nil, "", err
	}
	return meta, strings.Join(body, "\n"), nil
}
