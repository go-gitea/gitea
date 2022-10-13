// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContainsParentDirectorySeparator(t *testing.T) {
	tests := []struct {
		v string
		b bool
	}{
		{
			v: `user2/repo1/info/refs`,
			b: false,
		},
		{
			v: `user2/repo1/HEAD`,
			b: false,
		},
		{
			v: `user2/repo1/some.../strange_file...mp3`,
			b: false,
		},
		{
			v: `user2/repo1/../../custom/conf/app.ini`,
			b: true,
		},
		{
			v: `user2/repo1/objects/info/..\..\..\..\custom\conf\app.ini`,
			b: true,
		},
	}

	for i := range tests {
		assert.EqualValues(t, tests[i].b, containsParentDirectorySeparator(tests[i].v))
	}
}
