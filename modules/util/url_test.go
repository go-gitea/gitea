// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain url is unchanged",
			in:   "https://avatars.githubusercontent.com/u/9919",
			want: "https://avatars.githubusercontent.com/u/9919",
		},
		{
			name: "userinfo is stripped",
			in:   "https://alice:secret@example.com/path/avatar.png",
			want: "https://example.com/path/avatar.png",
		},
		{
			name: "query string is stripped (signed URL credentials)",
			in:   "https://bucket.s3.amazonaws.com/avatar.png?X-Amz-Signature=abc123&X-Amz-Expires=3600",
			want: "https://bucket.s3.amazonaws.com/avatar.png",
		},
		{
			name: "fragment is stripped",
			in:   "https://example.com/avatar.png#token=xyz",
			want: "https://example.com/avatar.png",
		},
		{
			name: "userinfo + query + fragment are all stripped together",
			in:   "https://u:p@host.example.com/p?sig=deadbeef#frag",
			want: "https://host.example.com/p",
		},
		{
			name: "empty url stays empty",
			in:   "",
			want: "",
		},
		{
			name: "unparseable url is replaced with placeholder",
			// %ZZ is not a valid percent-escape; net/url returns a parse error.
			in:   "http://example.com/%ZZ",
			want: "<unparseable url>",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, StripURL(c.in))
		})
	}
}
