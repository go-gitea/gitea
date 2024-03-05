// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"reflect"
	"testing"
)

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
