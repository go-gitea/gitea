// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"net/url"
	"testing"

	"code.gitea.io/gitea/cmd"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

func Test_CmdKeys(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		tests := []struct {
			name           string
			args           []string
			wantErr        bool
			expectedOutput string
		}{
			{"test_empty_1", []string{"keys", "--username=git", "--type=test", "--content=test"}, true, ""},
			{"test_empty_2", []string{"keys", "-e", "git", "-u", "git", "-t", "test", "-k", "test"}, true, ""},
			{
				"with_key",
				[]string{"keys", "-e", "git", "-u", "git", "-t", "ssh-rsa", "-k", "AAAAB3NzaC1yc2EAAAADAQABAAABgQDWVj0fQ5N8wNc0LVNA41wDLYJ89ZIbejrPfg/avyj3u/ZohAKsQclxG4Ju0VirduBFF9EOiuxoiFBRr3xRpqzpsZtnMPkWVWb+akZwBFAx8p+jKdy4QXR/SZqbVobrGwip2UjSrri1CtBxpJikojRIZfCnDaMOyd9Jp6KkujvniFzUWdLmCPxUE9zhTaPu0JsEP7MW0m6yx7ZUhHyfss+NtqmFTaDO+QlMR7L2QkDliN2Jl3Xa3PhuWnKJfWhdAq1Cw4oraKUOmIgXLkuiuxVQ6mD3AiFupkmfqdHq6h+uHHmyQqv3gU+/sD8GbGAhf6ftqhTsXjnv1Aj4R8NoDf9BS6KRkzkeun5UisSzgtfQzjOMEiJtmrep2ZQrMGahrXa+q4VKr0aKJfm+KlLfwm/JztfsBcqQWNcTURiCFqz+fgZw0Ey/de0eyMzldYTdXXNRYCKjs9bvBK+6SSXRM7AhftfQ0ZuoW5+gtinPrnmoOaSCEJbAiEiTO/BzOHgowiM="},
				false,
				"# gitea public key\ncommand=\"" + setting.AppPath + " --config=" + util.ShellEscape(setting.CustomConf) + " serv key-1\",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty,no-user-rc,restrict ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDWVj0fQ5N8wNc0LVNA41wDLYJ89ZIbejrPfg/avyj3u/ZohAKsQclxG4Ju0VirduBFF9EOiuxoiFBRr3xRpqzpsZtnMPkWVWb+akZwBFAx8p+jKdy4QXR/SZqbVobrGwip2UjSrri1CtBxpJikojRIZfCnDaMOyd9Jp6KkujvniFzUWdLmCPxUE9zhTaPu0JsEP7MW0m6yx7ZUhHyfss+NtqmFTaDO+QlMR7L2QkDliN2Jl3Xa3PhuWnKJfWhdAq1Cw4oraKUOmIgXLkuiuxVQ6mD3AiFupkmfqdHq6h+uHHmyQqv3gU+/sD8GbGAhf6ftqhTsXjnv1Aj4R8NoDf9BS6KRkzkeun5UisSzgtfQzjOMEiJtmrep2ZQrMGahrXa+q4VKr0aKJfm+KlLfwm/JztfsBcqQWNcTURiCFqz+fgZw0Ey/de0eyMzldYTdXXNRYCKjs9bvBK+6SSXRM7AhftfQ0ZuoW5+gtinPrnmoOaSCEJbAiEiTO/BzOHgowiM= user2@localhost\n",
			},
			{"invalid", []string{"keys", "--not-a-flag=git"}, true, "Incorrect Usage: flag provided but not defined: -not-a-flag\n\n"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				app := &cli.Command{
					Writer:    &stdout,
					ErrWriter: &stderr,
					Commands:  []*cli.Command{cmd.CmdKeys},
				}
				cmd.CmdKeys.HideHelp = true
				err := app.Run(t.Context(), append([]string{"prog"}, tt.args...))
				if tt.wantErr {
					assert.Error(t, err)
					assert.Equal(t, tt.expectedOutput, stderr.String())
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.expectedOutput, stdout.String())
				}
			})
		}
	})
}
