// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"
)

func TestLoadRepositoryCreationLimits(t *testing.T) {
	defer test.MockVariableValue(&Repository)()

	testConfigLoad(t, []any{loadRepositoryFrom}, []configTestCase{
		{
			name: "shortcut propagates to both",
			ini:  "[repository]\nMAX_CREATION_LIMIT = 5",
			want: []configCheck{
				field("MAX_CREATION_LIMIT", &Repository.MaxCreationLimit, 5),
				field("USER_MAX_CREATION_LIMIT", &Repository.UserMaxCreationLimit, 5),
				field("ORG_MAX_CREATION_LIMIT", &Repository.OrgMaxCreationLimit, 5),
			},
		},
		{
			name: "per-type keys override the shortcut",
			ini:  "[repository]\nMAX_CREATION_LIMIT = 5\nUSER_MAX_CREATION_LIMIT = 0\nORG_MAX_CREATION_LIMIT = -1",
			want: []configCheck{
				field("USER_MAX_CREATION_LIMIT", &Repository.UserMaxCreationLimit, 0),
				field("ORG_MAX_CREATION_LIMIT", &Repository.OrgMaxCreationLimit, -1),
			},
		},
		{
			name: "partial override, the other inherits the shortcut",
			ini:  "[repository]\nMAX_CREATION_LIMIT = 7\nORG_MAX_CREATION_LIMIT = -1",
			want: []configCheck{
				field("USER_MAX_CREATION_LIMIT", &Repository.UserMaxCreationLimit, 7),
				field("ORG_MAX_CREATION_LIMIT", &Repository.OrgMaxCreationLimit, -1),
			},
		},
		{
			name: "no key means no limit",
			ini:  "[repository]",
			want: []configCheck{
				field("MAX_CREATION_LIMIT", &Repository.MaxCreationLimit, -1),
				field("USER_MAX_CREATION_LIMIT", &Repository.UserMaxCreationLimit, -1),
				field("ORG_MAX_CREATION_LIMIT", &Repository.OrgMaxCreationLimit, -1),
			},
		},
	})
}
