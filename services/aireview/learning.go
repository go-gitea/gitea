// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import (
	"strings"
	"sync"
)

// Learning stores user feedback to improve future reviews.
type Learning struct {
	PathGlob    string // file path glob this applies to
	Instruction string // instruction text (e.g., "This is intentional, not a bug")
}

// learningStore holds per-repo learnings in memory.
type learningStore struct {
	mu  sync.RWMutex
	all map[int64][]Learning // repoID → learnings
}

var learnings = &learningStore{all: make(map[int64][]Learning)}

// AddLearning stores a new learning for a repo.
func AddLearning(repoID int64, l Learning) {
	learnings.mu.Lock()
	defer learnings.mu.Unlock()
	learnings.all[repoID] = append(learnings.all[repoID], l)
}

// GetLearnings returns all learnings for a repo.
func GetLearnings(repoID int64) []Learning {
	learnings.mu.RLock()
	defer learnings.mu.RUnlock()
	result := make([]Learning, len(learnings.all[repoID]))
	copy(result, learnings.all[repoID])
	return result
}

// DetectAndStoreLearnings parses a user's chat message for feedback patterns
// and stores them as learnings.
func DetectAndStoreLearnings(repoID int64, message string) {
	msg := strings.ToLower(strings.TrimSpace(message))

	// Pattern: "ignore X" or "this is not a bug" or "false positive"
	switch {
	case strings.Contains(msg, "ignore"), strings.Contains(msg, "not a bug"),
		strings.Contains(msg, "false positive"), strings.Contains(msg, "not an issue"):
		AddLearning(repoID, Learning{
			PathGlob:    "*",
			Instruction: message,
		})
	case strings.HasPrefix(msg, "learn:"):
		parts := strings.SplitN(message, ":", 3)
		var glob, instruction string
		if len(parts) >= 3 {
			glob = strings.TrimSpace(parts[1])
			instruction = strings.TrimSpace(parts[2])
		} else if len(parts) == 2 {
			instruction = strings.TrimSpace(parts[1])
		}
		if instruction != "" {
			if glob == "" {
				glob = "*"
			}
			AddLearning(repoID, Learning{PathGlob: glob, Instruction: instruction})
		}
	}
}

// BuildLearningsPrompt builds a string with all learnings for inclusion in the system prompt.
func BuildLearningsPrompt(repoID int64) string {
	items := GetLearnings(repoID)
	if len(items) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\nPrevious feedback to consider in this review:\n")
	for _, l := range items {
		b.WriteString("- ")
		if l.PathGlob != "*" {
			b.WriteString("[" + l.PathGlob + "] ")
		}
		b.WriteString(l.Instruction)
		b.WriteString("\n")
	}
	return b.String()
}
