// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"context"
	"html/template"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderCommitBody(t *testing.T) {
	type args struct {
		ctx       context.Context
		msg       string
		urlPrefix string
		metas     map[string]string
	}
	tests := []struct {
		name string
		args args
		want template.HTML
	}{
		{
			name: "multiple lines",
			args: args{
				ctx: context.Background(),
				msg: "first line\nsecond line",
			},
			want: "second line",
		},
		{
			name: "multiple lines with leading newlines",
			args: args{
				ctx: context.Background(),
				msg: "\n\n\n\nfirst line\nsecond line",
			},
			want: "second line",
		},
		{
			name: "multiple lines with trailing newlines",
			args: args{
				ctx: context.Background(),
				msg: "first line\nsecond line\n\n\n",
			},
			want: "second line",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, RenderCommitBody(tt.args.ctx, tt.args.msg, tt.args.urlPrefix, tt.args.metas), "RenderCommitBody(%v, %v, %v, %v)", tt.args.ctx, tt.args.msg, tt.args.urlPrefix, tt.args.metas)
		})
	}
}
