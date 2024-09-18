// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestIssueTemplate_Type(t *testing.T) {
	tests := []struct {
		fileName string
		want     IssueTemplateType
	}{
		{
			fileName: ".gitea/ISSUE_TEMPLATE/bug_report.yaml",
			want:     IssueTemplateTypeYaml,
		},
		{
			fileName: ".gitea/ISSUE_TEMPLATE/bug_report.md",
			want:     IssueTemplateTypeMarkdown,
		},
		{
			fileName: ".gitea/ISSUE_TEMPLATE/bug_report.txt",
			want:     "",
		},
		{
			fileName: ".gitea/ISSUE_TEMPLATE/config.yaml",
			want:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.fileName, func(t *testing.T) {
			it := IssueTemplate{
				FileName: tt.fileName,
			}
			assert.Equal(t, tt.want, it.Type())
		})
	}
}

func TestIssueTemplateStringSlice_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		content string
		tmpl    *IssueTemplate
		want    *IssueTemplate
		wantErr string
	}{
		{
			name:    "array",
			content: `labels: ["a", "b", "c"]`,
			tmpl: &IssueTemplate{
				Labels: []string{"should_be_overwrote"},
			},
			want: &IssueTemplate{
				Labels: []string{"a", "b", "c"},
			},
		},
		{
			name:    "string",
			content: `labels: "a,b,c"`,
			tmpl: &IssueTemplate{
				Labels: []string{"should_be_overwrote"},
			},
			want: &IssueTemplate{
				Labels: []string{"a", "b", "c"},
			},
		},
		{
			name:    "empty",
			content: `labels:`,
			tmpl: &IssueTemplate{
				Labels: []string{"should_be_overwrote"},
			},
			want: &IssueTemplate{
				Labels: nil,
			},
		},
		{
			name: "error",
			content: `
labels:
  a: aa
  b: bb
`,
			tmpl:    &IssueTemplate{},
			wantErr: "line 3: cannot unmarshal !!map into IssueTemplateStringSlice",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := yaml.Unmarshal([]byte(tt.content), tt.tmpl)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, tt.tmpl)
			}
		})
	}
}
