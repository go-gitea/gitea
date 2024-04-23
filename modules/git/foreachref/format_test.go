// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package foreachref_test

import (
	"testing"

	"code.gitea.io/gitea/modules/git/foreachref"

	"github.com/stretchr/testify/require"
)

func TestFormat_Flag(t *testing.T) {
	tests := []struct {
		name string

		givenFormat foreachref.Format

		wantFlag string
	}{
		{
			name: "references are delimited by dual null chars",

			// no reference fields requested
			givenFormat: foreachref.NewFormat(),

			// only a reference delimiter field in --format
			wantFlag: "%00%00",
		},

		{
			name: "a field is a space-separated key-value pair",

			givenFormat: foreachref.NewFormat("refname:short"),

			// only a reference delimiter field
			wantFlag: "refname:short %(refname:short)%00%00",
		},

		{
			name: "fields are separated by a null char field-delimiter",

			givenFormat: foreachref.NewFormat("refname:short", "author"),

			wantFlag: "refname:short %(refname:short)%00author %(author)%00%00",
		},

		{
			name: "multiple fields",

			givenFormat: foreachref.NewFormat("refname:lstrip=2", "objecttype", "objectname"),

			wantFlag: "refname:lstrip=2 %(refname:lstrip=2)%00objecttype %(objecttype)%00objectname %(objectname)%00%00",
		},
	}

	for _, test := range tests {
		tc := test // don't close over loop variable
		t.Run(tc.name, func(t *testing.T) {
			gotFlag := tc.givenFormat.Flag()

			require.Equal(t, tc.wantFlag, gotFlag, "unexpected for-each-ref --format string. wanted: '%s', got: '%s'", tc.wantFlag, gotFlag)
		})
	}
}
