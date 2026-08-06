// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"strings"
	"testing"

	"gitea.dev/actionslib/pkg/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		options []ParseOption
		wantErr bool
	}{
		{
			name:    "multiple_jobs",
			options: nil,
			wantErr: false,
		},
		{
			name:    "multiple_matrix",
			options: nil,
			wantErr: false,
		},
		{
			name:    "has_needs",
			options: nil,
			wantErr: false,
		},
		{
			name:    "has_with",
			options: nil,
			wantErr: false,
		},
		{
			name:    "has_secrets",
			options: nil,
			wantErr: false,
		},
		{
			name:    "empty_step",
			options: nil,
			wantErr: false,
		},
		{
			name:    "job_name_with_matrix",
			options: nil,
			wantErr: false,
		},
		{
			name:    "prefixed_newline",
			options: nil,
			wantErr: false,
		},
		{
			name:    "continue_on_error_expr",
			options: nil,
			wantErr: false,
		},
	}
	invalidFileTests := []struct {
		name string
	}{
		{name: "null_job_implicit"},
		{name: "null_job_explicit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := ReadTestdata(t, tt.name+".in.yaml")
			want := ReadTestdata(t, tt.name+".out.yaml")
			got, err := Parse(content, tt.options...)
			if tt.wantErr {
				require.Error(t, err)
			}
			require.NoError(t, err)

			builder := &strings.Builder{}
			for _, v := range got {
				if builder.Len() > 0 {
					builder.WriteString("---\n")
				}
				encoder := yaml.NewEncoder(builder)
				encoder.SetIndent(2)
				require.NoError(t, encoder.Encode(v))
				id, job := v.Job()
				assert.NotEmpty(t, id)
				assert.NotNil(t, job)
			}
			assert.Equal(t, string(want), builder.String())
		})
	}

	for _, tt := range invalidFileTests {
		t.Run(tt.name, func(t *testing.T) {
			content := ReadTestdata(t, tt.name+".in.yaml")
			require.NotPanics(t, func() {
				_, err := Parse(content)
				require.Error(t, err)
			})
		})
	}
}

func TestParseInterpolatesRunName(t *testing.T) {
	workflow := func(runName string) []byte {
		return []byte("name: t\nrun-name: \"" + runName + "\"\non: push\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps: [{run: echo}]\n")
	}

	for _, tt := range []struct{ name, runName, want string }{
		{"bool", "${{ true }}", "true"},
		{"int", "${{ 1 }}", "1"},
		{"float", "${{ 1.0 }}", "1"},
		{"null", "${{ null }}", ""},
		{"object", `${{ fromJSON('{\"a\":1}') }}`, "Object"},
		{"array", "${{ fromJSON('[1,2]') }}", "Array"},
		{"context", "${{ github }}", "Object"},
		{"surrounding literals", "run ${{ 1 }} now", "run 1 now"},
		{"two expressions", "${{ 1 }}-${{ true }}", "1-true"},
		{"closing brace inside a string", "${{ 'a}}b' }}", "a}}b"},
		{"incomplete expression stays literal", "${{ 1", "${{ 1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(workflow(tt.runName), WithGitContext(&model.GithubContext{EventName: "push"}))
			require.NoError(t, err)
			require.Len(t, result, 1)
			assert.Equal(t, tt.want, result[0].RunName)
		})
	}

	// a malformed part must not restructure the surrounding expression
	for _, runName := range []string{"${{ 1) && (2 }}", "run ${{ 1) && (2 }} now", "${{ 'a' }} ${{ b", "${{ 'a }}"} {
		_, err := Parse(workflow(runName), WithGitContext(&model.GithubContext{EventName: "push"}))
		assert.ErrorContains(t, err, "interpolate run-name")
	}

	// callers such as commit status parse without a git context, leaving `github` a nil pointer
	result, err := Parse(workflow("${{ github }}"))
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Empty(t, result[0].RunName)
}
