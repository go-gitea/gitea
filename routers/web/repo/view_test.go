// Copyright 2017 The Gitea Authors. All rights reserved.
// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMisc_isReadmeFile(t *testing.T) {
	trueTestCases := []string{
		"readme",
		"README",
		"readME.mdown",
		"README.md",
		"readme.i18n.md",
	}
	falseTestCases := []string{
		"test.md",
		"wow.MARKDOWN",
		"LOL.mDoWn",
		"test",
		"abcdefg",
		"abcdefghijklmnopqrstuvwxyz",
		"test.md.test",
		"readmf",
	}

	for _, testCase := range trueTestCases {
		assert.True(t, isReadmeFile(testCase))
	}
	for _, testCase := range falseTestCases {
		assert.False(t, isReadmeFile(testCase))
	}

	type extensionTestcase struct {
		name     string
		expected bool
		idx      int
	}

	exts := []string{".md", ".txt", ""}
	testCasesExtensions := []extensionTestcase{
		{
			name:     "readme",
			expected: true,
			idx:      2,
		},
		{
			name:     "readme.md",
			expected: true,
			idx:      0,
		},
		{
			name:     "README.md",
			expected: true,
			idx:      0,
		},
		{
			name:     "ReAdMe.Md",
			expected: true,
			idx:      0,
		},
		{
			name:     "readme.txt",
			expected: true,
			idx:      1,
		},
		{
			name:     "readme.doc",
			expected: true,
			idx:      3,
		},
		{
			name: "readmee.md",
		},
		{
			name:     "readme..",
			expected: true,
			idx:      3,
		},
	}

	for _, testCase := range testCasesExtensions {
		idx, ok := isReadmeFileExtension(testCase.name, exts...)
		assert.Equal(t, testCase.expected, ok)
		assert.Equal(t, testCase.idx, idx)
	}
}

func Test_localizedExtensions(t *testing.T) {
	tests := []struct {
		name              string
		ext               string
		languageCode      string
		wantLocalizedExts []string
	}{
		{
			name:              "empty language",
			ext:               ".md",
			wantLocalizedExts: []string{".md"},
		},
		{
			name:              "No region - lowercase",
			languageCode:      "en",
			ext:               ".csv",
			wantLocalizedExts: []string{".en.csv", ".csv"},
		},
		{
			name:              "No region - uppercase",
			languageCode:      "FR",
			ext:               ".txt",
			wantLocalizedExts: []string{".fr.txt", ".txt"},
		},
		{
			name:              "With region - lowercase",
			languageCode:      "en-us",
			ext:               ".md",
			wantLocalizedExts: []string{".en-us.md", ".en_us.md", ".en.md", "_en.md", ".md"},
		},
		{
			name:              "With region - uppercase",
			languageCode:      "en-CA",
			ext:               ".MD",
			wantLocalizedExts: []string{".en-ca.MD", ".en_ca.MD", ".en.MD", "_en.MD", ".MD"},
		},
		{
			name:              "With region - all uppercase",
			languageCode:      "ZH-TW",
			ext:               ".md",
			wantLocalizedExts: []string{".zh-tw.md", ".zh_tw.md", ".zh.md", "_zh.md", ".md"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotLocalizedExts := localizedExtensions(tt.ext, tt.languageCode); !reflect.DeepEqual(gotLocalizedExts, tt.wantLocalizedExts) {
				t.Errorf("localizedExtensions() = %v, want %v", gotLocalizedExts, tt.wantLocalizedExts)
			}
		})
	}
}
