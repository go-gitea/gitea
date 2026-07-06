// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"
	"strconv"
	"strings"
)

// AIRreview settings
var AIRreview = struct {
	Enabled         bool
	Provider        string
	APIURL          string
	APIToken        string
	Model           string
	MaxTokens       int
	Temperature     float64
	TriggerOnOpen   bool
	TriggerOnUpdate bool
	MaxPatchSize    int
	Timeout         int
	ExcludePaths    []string
	SystemPrompt    string
}{
	Enabled:         false,
	Provider:        "openrouter",
	APIURL:          "https://openrouter.ai/api/v1",
	Model:           "openai/gpt-4o",
	MaxTokens:       4096,
	Temperature:     0.3,
	TriggerOnOpen:   true,
	TriggerOnUpdate: false,
	MaxPatchSize:    80000,
	Timeout:         120,
	ExcludePaths:    nil,
}

func loadAIReviewFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("ai_review")
	AIRreview.Enabled = sec.Key("ENABLED").MustBool(false)
	AIRreview.Provider = sec.Key("PROVIDER").MustString("openrouter")
	AIRreview.APIURL = sec.Key("API_URL").MustString("https://openrouter.ai/api/v1")
	AIRreview.APIToken = sec.Key("API_TOKEN").MustString("")
	AIRreview.Model = sec.Key("MODEL").MustString("openai/gpt-4o")
	AIRreview.MaxTokens = sec.Key("MAX_TOKENS").MustInt(4096)
	if v, err := strconv.ParseFloat(sec.Key("TEMPERATURE").String(), 64); err == nil {
		AIRreview.Temperature = v
	} else {
		AIRreview.Temperature = 0.3
	}
	AIRreview.TriggerOnOpen = sec.Key("TRIGGER_ON_OPEN").MustBool(true)
	AIRreview.TriggerOnUpdate = sec.Key("TRIGGER_ON_UPDATE").MustBool(false)
	AIRreview.MaxPatchSize = sec.Key("MAX_PATCH_SIZE").MustInt(80000)
	AIRreview.Timeout = sec.Key("TIMEOUT").MustInt(120)
	AIRreview.SystemPrompt = sec.Key("SYSTEM_PROMPT").MustString("")
	raw := sec.Key("EXCLUDE_PATHS").MustString("")
	if raw != "" {
		for p := range strings.SplitSeq(raw, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				AIRreview.ExcludePaths = append(AIRreview.ExcludePaths, p)
			}
		}
	}
}

// IsExcludedPath checks if a file path matches any exclusion pattern.
func IsExcludedPath(path string) bool {
	for _, pattern := range AIRreview.ExcludePaths {
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}
	return false
}
