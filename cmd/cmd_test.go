// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

func TestDefaultCommand(t *testing.T) {
	test := func(t *testing.T, args []string, expectedRetName string, expectedRetValid bool) {
		called := false
		cmd := &cli.Command{
			DefaultCommand: "test",
			Commands: []*cli.Command{
				{
					Name: "test",
					Action: func(ctx context.Context, command *cli.Command) error {
						retName, retValid := isValidDefaultSubCommand(command)
						assert.Equal(t, expectedRetName, retName)
						assert.Equal(t, expectedRetValid, retValid)
						called = true
						return nil
					},
				},
			},
		}
		assert.NoError(t, cmd.Run(t.Context(), args))
		assert.True(t, called)
	}
	test(t, []string{"./gitea"}, "", true)
	test(t, []string{"./gitea", "test"}, "", true)
	test(t, []string{"./gitea", "other"}, "other", false)
}
