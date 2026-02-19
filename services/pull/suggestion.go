// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

type SuggestionBlock struct {
	Content string
}

// ParseSuggestionBlocks extracts ```suggestion fenced blocks from markdown content.
func ParseSuggestionBlocks(markdown string) []SuggestionBlock {
	normalized := string(util.NormalizeEOL([]byte(markdown)))
	lines := strings.Split(normalized, "\n")
	blocks := make([]SuggestionBlock, 0)
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !strings.HasPrefix(line, "```") {
			continue
		}
		fenceLen := 0
		for fenceLen < len(line) && line[fenceLen] == '`' {
			fenceLen++
		}
		if fenceLen < 3 {
			continue
		}
		info := strings.TrimSpace(line[fenceLen:])
		if info != "suggestion" {
			continue
		}
		var contentLines []string
		for j := i + 1; j < len(lines); j++ {
			if strings.HasPrefix(lines[j], strings.Repeat("`", fenceLen)) {
				blocks = append(blocks, SuggestionBlock{Content: strings.Join(contentLines, "\n")})
				i = j
				break
			}
			contentLines = append(contentLines, lines[j])
		}
	}
	return blocks
}

// BuildSuggestionPatch creates a unified diff patch that replaces startLine..endLine with suggestion content.
func BuildSuggestionPatch(filePath, fileContent string, startLine, endLine int, suggestion string, contextLines int) (string, error) {
	if startLine <= 0 || endLine <= 0 {
		return "", errors.New("invalid line range")
	}
	if startLine > endLine {
		startLine, endLine = endLine, startLine
	}

	normalizedContent := string(util.NormalizeEOL([]byte(fileContent)))
	normalizedContent = strings.TrimSuffix(normalizedContent, "\n")
	var contentLines []string
	if normalizedContent != "" {
		contentLines = strings.Split(normalizedContent, "\n")
	}

	if startLine > len(contentLines) || endLine > len(contentLines) {
		return "", errors.New("line range out of bounds")
	}

	normalizedSuggestion := string(util.NormalizeEOL([]byte(suggestion)))
	normalizedSuggestion = strings.TrimSuffix(normalizedSuggestion, "\n")
	suggestionLines := []string{}
	if normalizedSuggestion != "" {
		suggestionLines = strings.Split(normalizedSuggestion, "\n")
	}

	preStart := max(startLine-contextLines, 1)
	postEnd := min(endLine+contextLines, len(contentLines))

	preLines := contentLines[preStart-1 : startLine-1]
	oldLines := contentLines[startLine-1 : endLine]
	postLines := contentLines[endLine:postEnd]

	oldCount := len(preLines) + len(oldLines) + len(postLines)
	newCount := len(preLines) + len(suggestionLines) + len(postLines)

	var sb strings.Builder
	fmt.Fprintf(&sb, "diff --git a/%s b/%s\n", filePath, filePath)
	fmt.Fprintf(&sb, "--- a/%s\n", filePath)
	fmt.Fprintf(&sb, "+++ b/%s\n", filePath)
	fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@\n", preStart, oldCount, preStart, newCount)
	for _, line := range preLines {
		sb.WriteString(" " + line + "\n")
	}
	for _, line := range oldLines {
		sb.WriteString("-" + line + "\n")
	}
	for _, line := range suggestionLines {
		sb.WriteString("+" + line + "\n")
	}
	for _, line := range postLines {
		sb.WriteString(" " + line + "\n")
	}

	return sb.String(), nil
}
