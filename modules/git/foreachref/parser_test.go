// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package foreachref_test

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/git/foreachref"
	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/require"
)

type refSlice = []map[string]string

func TestParser(t *testing.T) {
	tests := []struct {
		name string

		givenFormat foreachref.Format
		givenInput  io.Reader

		wantRefs    refSlice
		wantErr     bool
		expectedErr error
	}{
		// this would, for example, be the result when running `git
		// for-each-ref refs/tags` on a repo without tags.
		{
			name: "no references on empty input",

			givenFormat: foreachref.NewFormat("refname:short"),
			givenInput:  strings.NewReader(``),

			wantRefs: []map[string]string{},
		},

		// note: `git for-each-ref` will add a newline between every
		// reference (in addition to the ref-delimiter we've chosen)
		{
			name: "single field requested, single reference in output",

			givenFormat: foreachref.NewFormat("refname:short"),
			givenInput:  strings.NewReader("refname:short v0.0.1\x00\x00" + "\n"),

			wantRefs: []map[string]string{
				{"refname:short": "v0.0.1"},
			},
		},
		{
			name: "single field requested, multiple references in output",

			givenFormat: foreachref.NewFormat("refname:short"),
			givenInput: strings.NewReader(
				"refname:short v0.0.1\x00\x00" + "\n" +
					"refname:short v0.0.2\x00\x00" + "\n" +
					"refname:short v0.0.3\x00\x00" + "\n"),

			wantRefs: []map[string]string{
				{"refname:short": "v0.0.1"},
				{"refname:short": "v0.0.2"},
				{"refname:short": "v0.0.3"},
			},
		},

		{
			name: "multiple fields requested for each reference",

			givenFormat: foreachref.NewFormat("refname:short", "objecttype", "objectname"),
			givenInput: strings.NewReader(

				"refname:short v0.0.1\x00objecttype commit\x00objectname 7b2c5ac9fc04fc5efafb60700713d4fa609b777b\x00\x00" + "\n" +
					"refname:short v0.0.2\x00objecttype commit\x00objectname a1f051bc3eba734da4772d60e2d677f47cf93ef4\x00\x00" + "\n" +
					"refname:short v0.0.3\x00objecttype commit\x00objectname ef82de70bb3f60c65fb8eebacbb2d122ef517385\x00\x00" + "\n",
			),

			wantRefs: []map[string]string{
				{
					"refname:short": "v0.0.1",
					"objecttype":    "commit",
					"objectname":    "7b2c5ac9fc04fc5efafb60700713d4fa609b777b",
				},
				{
					"refname:short": "v0.0.2",
					"objecttype":    "commit",
					"objectname":    "a1f051bc3eba734da4772d60e2d677f47cf93ef4",
				},
				{
					"refname:short": "v0.0.3",
					"objecttype":    "commit",
					"objectname":    "ef82de70bb3f60c65fb8eebacbb2d122ef517385",
				},
			},
		},

		{
			name: "must handle multi-line fields such as 'content'",

			givenFormat: foreachref.NewFormat("refname:short", "contents", "author"),
			givenInput: strings.NewReader(
				"refname:short v0.0.1\x00contents Create new buffer if not present yet (#549)\n\nFixes a nil dereference when ProcessFoo is used\nwith multiple commands.\x00author Foo Bar <foo@bar.com> 1507832733 +0200\x00\x00" + "\n" +
					"refname:short v0.0.2\x00contents Update CI config (#651)\n\n\x00author John Doe <john.doe@foo.com> 1521643174 +0000\x00\x00" + "\n" +
					"refname:short v0.0.3\x00contents Fixed code sample for bash completion (#687)\n\n\x00author Foo Baz <foo@baz.com> 1524836750 +0200\x00\x00" + "\n",
			),

			wantRefs: []map[string]string{
				{
					"refname:short": "v0.0.1",
					"contents":      "Create new buffer if not present yet (#549)\n\nFixes a nil dereference when ProcessFoo is used\nwith multiple commands.",
					"author":        "Foo Bar <foo@bar.com> 1507832733 +0200",
				},
				{
					"refname:short": "v0.0.2",
					"contents":      "Update CI config (#651)",
					"author":        "John Doe <john.doe@foo.com> 1521643174 +0000",
				},
				{
					"refname:short": "v0.0.3",
					"contents":      "Fixed code sample for bash completion (#687)",
					"author":        "Foo Baz <foo@baz.com> 1524836750 +0200",
				},
			},
		},

		{
			name: "must handle fields without values",

			givenFormat: foreachref.NewFormat("refname:short", "object", "objecttype"),
			givenInput: strings.NewReader(
				"refname:short v0.0.1\x00object \x00objecttype commit\x00\x00" + "\n" +
					"refname:short v0.0.2\x00object \x00objecttype commit\x00\x00" + "\n" +
					"refname:short v0.0.3\x00object \x00objecttype commit\x00\x00" + "\n",
			),

			wantRefs: []map[string]string{
				{
					"refname:short": "v0.0.1",
					"object":        "",
					"objecttype":    "commit",
				},
				{
					"refname:short": "v0.0.2",
					"object":        "",
					"objecttype":    "commit",
				},
				{
					"refname:short": "v0.0.3",
					"object":        "",
					"objecttype":    "commit",
				},
			},
		},

		{
			name: "must fail when the number of fields in the input doesn't match expected format",

			givenFormat: foreachref.NewFormat("refname:short", "objecttype", "objectname"),
			givenInput: strings.NewReader(
				"refname:short v0.0.1\x00objecttype commit\x00\x00" + "\n" +
					"refname:short v0.0.2\x00objecttype commit\x00\x00" + "\n" +
					"refname:short v0.0.3\x00objecttype commit\x00\x00" + "\n",
			),

			wantErr:     true,
			expectedErr: errors.New("unexpected number of reference fields: wanted 2, was 3"),
		},

		{
			name: "must fail input fields don't match expected format",

			givenFormat: foreachref.NewFormat("refname:short", "objectname"),
			givenInput: strings.NewReader(
				"refname:short v0.0.1\x00objecttype commit\x00\x00" + "\n" +
					"refname:short v0.0.2\x00objecttype commit\x00\x00" + "\n" +
					"refname:short v0.0.3\x00objecttype commit\x00\x00" + "\n",
			),

			wantErr:     true,
			expectedErr: errors.New("unexpected field name at position 1: wanted: 'objectname', was: 'objecttype'"),
		},
	}

	for _, test := range tests {
		tc := test // don't close over loop variable
		t.Run(tc.name, func(t *testing.T) {
			parser := tc.givenFormat.Parser(tc.givenInput)

			//
			// parse references from input
			//
			gotRefs := make([]map[string]string, 0)
			for {
				ref := parser.Next()
				if ref == nil {
					break
				}
				gotRefs = append(gotRefs, ref)
			}
			err := parser.Err()

			//
			// verify expectations
			//
			if tc.wantErr {
				require.Error(t, err)
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err, "for-each-ref parser unexpectedly failed with: %v", err)
				require.Equal(t, tc.wantRefs, gotRefs, "for-each-ref parser produced unexpected reference set. wanted: %v, got: %v", pretty(tc.wantRefs), pretty(gotRefs))
			}
		})
	}
}

func pretty(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		// shouldn't happen
		panic(fmt.Sprintf("json-marshalling failed: %v", err))
	}
	return string(data)
}
