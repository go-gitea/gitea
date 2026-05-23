// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"strings"
	"testing"

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
