// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckAttributeFile(t *testing.T) {
	testContent := `*.txt text eol=lf
/vendor/** -text -eol linguist-vendored
/tools/**/*.py linguist-vendored
`
	r := strings.NewReader(testContent)
	checker, err := LoadAttrbutCheckerFromReader(r)
	if !assert.NoError(t, err) {
		return
	}
	if !assert.NotEmpty(t, checker) {
		return
	}

	tests := []struct {
		want        *AttrCheckResult
		content     string
		requestAttr string
		path        string
	}{
		{
			want: &AttrCheckResult{
				typ:  AttrCheckResultTypeValue,
				data: "lf",
			},
			path:        "aa.txt",
			requestAttr: "eol",
		},
		{
			want: &AttrCheckResult{
				typ:  AttrCheckResultTypeUnset,
				data: "",
			},
			path:        "vendor/aa.txt",
			requestAttr: "eol",
		},
		{
			want: &AttrCheckResult{
				typ:  AttrCheckResultTypeUnspecified,
				data: "",
			},
			path:        "aa.png",
			requestAttr: "text",
		},
		{
			want: &AttrCheckResult{
				typ:  AttrCheckResultTypeSet,
				data: "",
			},
			path:        "vendor/bbb/aa.json",
			requestAttr: "linguist-vendored",
		},
		// TODO: glob tools/**/*.py should match it, but can't ...
		// {
		// 	want: &AttrCheckResult{
		// 		typ:  AttrCheckResultTypeSet,
		// 		data: "",
		// 	},
		// 	path:        "tools/aa.py",
		// 	requestAttr: "linguist-vendored",
		// },
	}
	for _, tt := range tests {
		got := checker.Check(tt.requestAttr, tt.path)
		assert.Equal(t, tt.want, got)
	}
}
