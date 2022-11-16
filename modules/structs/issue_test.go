// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
