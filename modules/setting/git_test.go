// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"
)

func TestGitConfig(t *testing.T) {
	defer test.MockVariableValue(&Git)()
	defer test.MockVariableValue(&GitConfig)()

	// GitConfig.Options is a map, so it is read through a getter rather than addressed directly
	option := func(name string) func() string {
		return func() string { return GitConfig.Options[name] }
	}

	testConfigLoad(t, []any{loadGitFrom}, []configTestCase{
		{
			name: "unrelated option keeps the default diff algorithm",
			ini:  "[git.config]\na.b = 1",
			want: []configCheck{
				fieldOf("a.b", option("a.b"), "1"),
				fieldOf("diff.algorithm", option("diff.algorithm"), "histogram"),
			},
		},
		{
			name: "diff algorithm can be overridden",
			ini:  "[git.config]\ndiff.algorithm = other",
			want: []configCheck{fieldOf("diff.algorithm", option("diff.algorithm"), "other")},
		},
	})
}

func TestGitReflog(t *testing.T) {
	defer test.MockVariableValue(&Git)()
	defer test.MockVariableValue(&GitConfig)()

	option := func(name string) func() string {
		return func() string { return GitConfig.GetOption(name) }
	}

	testConfigLoad(t, []any{loadGitFrom}, []configTestCase{
		{
			name: "default reflog config without legacy options",
			want: []configCheck{
				fieldOf("core.logAllRefUpdates", option("core.logAllRefUpdates"), "true"),
				fieldOf("gc.reflogExpire", option("gc.reflogExpire"), "90"),
			},
		},
		{
			name: "custom reflog config by legacy options",
			ini:  "[git.reflog]\nENABLED = false\nEXPIRATION = 123",
			want: []configCheck{
				fieldOf("core.logAllRefUpdates", option("core.logAllRefUpdates"), "false"),
				fieldOf("gc.reflogExpire", option("gc.reflogExpire"), "123"),
			},
		},
	})
}
