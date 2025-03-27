// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"

	"github.com/stretchr/testify/assert"
)

func Test_fixUnitConfig_16961(t *testing.T) {
	tests := []struct {
		name      string
		bs        string
		wantFixed bool
		wantErr   bool
	}{
		{
			name:      "normal: {}",
			bs:        "{}",
			wantFixed: false,
			wantErr:   false,
		},
		{
			name:      "broken but fixable: &{}",
			bs:        "&{}",
			wantFixed: true,
			wantErr:   false,
		},
		{
			name:      "broken but unfixable: &{asdasd}",
			bs:        "&{asdasd}",
			wantFixed: false,
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFixed, err := fixUnitConfig16961([]byte(tt.bs), &repo_model.UnitConfig{})
			if (err != nil) != tt.wantErr {
				t.Errorf("fixUnitConfig_16961() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotFixed != tt.wantFixed {
				t.Errorf("fixUnitConfig_16961() = %v, want %v", gotFixed, tt.wantFixed)
			}
		})
	}
}

func Test_fixExternalWikiConfig_16961(t *testing.T) {
	tests := []struct {
		name      string
		bs        string
		expected  string
		wantFixed bool
		wantErr   bool
	}{
		{
			name:      "normal: {\"ExternalWikiURL\":\"http://someurl\"}",
			bs:        "{\"ExternalWikiURL\":\"http://someurl\"}",
			expected:  "http://someurl",
			wantFixed: false,
			wantErr:   false,
		},
		{
			name:      "broken: &{http://someurl}",
			bs:        "&{http://someurl}",
			expected:  "http://someurl",
			wantFixed: true,
			wantErr:   false,
		},
		{
			name:      "broken but unfixable: http://someurl",
			bs:        "http://someurl",
			wantFixed: false,
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &repo_model.ExternalWikiConfig{}
			gotFixed, err := fixExternalWikiConfig16961([]byte(tt.bs), cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("fixExternalWikiConfig_16961() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotFixed != tt.wantFixed {
				t.Errorf("fixExternalWikiConfig_16961() = %v, want %v", gotFixed, tt.wantFixed)
			}
			if cfg.ExternalWikiURL != tt.expected {
				t.Errorf("fixExternalWikiConfig_16961().ExternalWikiURL = %v, want %v", cfg.ExternalWikiURL, tt.expected)
			}
		})
	}
}

func Test_fixExternalTrackerConfig_16961(t *testing.T) {
	tests := []struct {
		name      string
		bs        string
		expected  repo_model.ExternalTrackerConfig
		wantFixed bool
		wantErr   bool
	}{
		{
			name: "normal",
			bs:   `{"ExternalTrackerURL":"a","ExternalTrackerFormat":"b","ExternalTrackerStyle":"c"}`,
			expected: repo_model.ExternalTrackerConfig{
				ExternalTrackerURL:    "a",
				ExternalTrackerFormat: "b",
				ExternalTrackerStyle:  "c",
			},
			wantFixed: false,
			wantErr:   false,
		},
		{
			name: "broken",
			bs:   "&{a b c}",
			expected: repo_model.ExternalTrackerConfig{
				ExternalTrackerURL:    "a",
				ExternalTrackerFormat: "b",
				ExternalTrackerStyle:  "c",
			},
			wantFixed: true,
			wantErr:   false,
		},
		{
			name:      "broken - too many fields",
			bs:        "&{a b c d}",
			wantFixed: false,
			wantErr:   true,
		},
		{
			name:      "broken - wrong format",
			bs:        "a b c d}",
			wantFixed: false,
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &repo_model.ExternalTrackerConfig{}
			gotFixed, err := fixExternalTrackerConfig16961([]byte(tt.bs), cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("fixExternalTrackerConfig_16961() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotFixed != tt.wantFixed {
				t.Errorf("fixExternalTrackerConfig_16961() = %v, want %v", gotFixed, tt.wantFixed)
			}
			if cfg.ExternalTrackerFormat != tt.expected.ExternalTrackerFormat {
				t.Errorf("fixExternalTrackerConfig_16961().ExternalTrackerFormat = %v, want %v", tt.expected.ExternalTrackerFormat, cfg.ExternalTrackerFormat)
			}
			if cfg.ExternalTrackerStyle != tt.expected.ExternalTrackerStyle {
				t.Errorf("fixExternalTrackerConfig_16961().ExternalTrackerStyle = %v, want %v", tt.expected.ExternalTrackerStyle, cfg.ExternalTrackerStyle)
			}
			if cfg.ExternalTrackerURL != tt.expected.ExternalTrackerURL {
				t.Errorf("fixExternalTrackerConfig_16961().ExternalTrackerURL = %v, want %v", tt.expected.ExternalTrackerURL, cfg.ExternalTrackerURL)
			}
		})
	}
}

func Test_fixPullRequestsConfig_16961(t *testing.T) {
	tests := []struct {
		name      string
		bs        string
		expected  repo_model.PullRequestsConfig
		wantFixed bool
		wantErr   bool
	}{
		{
			name: "normal",
			bs:   `{"IgnoreWhitespaceConflicts":false,"AllowMerge":false,"AllowRebase":false,"AllowRebaseMerge":false,"AllowSquash":false,"AllowManualMerge":false,"AutodetectManualMerge":false,"DefaultDeleteBranchAfterMerge":false,"DefaultMergeStyle":""}`,
		},
		{
			name: "broken - 1.14",
			bs:   `&{%!s(bool=false) %!s(bool=true) %!s(bool=true) %!s(bool=true) %!s(bool=true) %!s(bool=false) %!s(bool=false)}`,
			expected: repo_model.PullRequestsConfig{
				IgnoreWhitespaceConflicts: false,
				AllowMerge:                true,
				AllowRebase:               true,
				AllowRebaseMerge:          true,
				AllowSquash:               true,
				AllowManualMerge:          false,
				AutodetectManualMerge:     false,
			},
			wantFixed: true,
		},
		{
			name: "broken - 1.15",
			bs:   `&{%!s(bool=false) %!s(bool=true) %!s(bool=true) %!s(bool=true) %!s(bool=true) %!s(bool=false) %!s(bool=false) %!s(bool=false) merge}`,
			expected: repo_model.PullRequestsConfig{
				AllowMerge:        true,
				AllowRebase:       true,
				AllowRebaseMerge:  true,
				AllowSquash:       true,
				DefaultMergeStyle: repo_model.MergeStyleMerge,
			},
			wantFixed: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &repo_model.PullRequestsConfig{}
			gotFixed, err := fixPullRequestsConfig16961([]byte(tt.bs), cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("fixPullRequestsConfig_16961() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotFixed != tt.wantFixed {
				t.Errorf("fixPullRequestsConfig_16961() = %v, want %v", gotFixed, tt.wantFixed)
			}
			assert.EqualValues(t, &tt.expected, cfg)
		})
	}
}

func Test_fixIssuesConfig_16961(t *testing.T) {
	tests := []struct {
		name      string
		bs        string
		expected  repo_model.IssuesConfig
		wantFixed bool
		wantErr   bool
	}{
		{
			name: "normal",
			bs:   `{"EnableTimetracker":true,"AllowOnlyContributorsToTrackTime":true,"EnableDependencies":true}`,
			expected: repo_model.IssuesConfig{
				EnableTimetracker:                true,
				AllowOnlyContributorsToTrackTime: true,
				EnableDependencies:               true,
			},
		},
		{
			name: "broken",
			bs:   `&{%!s(bool=true) %!s(bool=true) %!s(bool=true)}`,
			expected: repo_model.IssuesConfig{
				EnableTimetracker:                true,
				AllowOnlyContributorsToTrackTime: true,
				EnableDependencies:               true,
			},
			wantFixed: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &repo_model.IssuesConfig{}
			gotFixed, err := fixIssuesConfig16961([]byte(tt.bs), cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("fixIssuesConfig_16961() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotFixed != tt.wantFixed {
				t.Errorf("fixIssuesConfig_16961() = %v, want %v", gotFixed, tt.wantFixed)
			}
			assert.EqualValues(t, &tt.expected, cfg)
		})
	}
}
