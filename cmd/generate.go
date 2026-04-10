// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/generate"

	"github.com/mattn/go-isatty"
	"github.com/urfave/cli/v3"
)

func newGenerateCommand() *cli.Command {
	return &cli.Command{
		Name:  "generate",
		Usage: "Generate Gitea's secrets/keys/tokens",
		Commands: []*cli.Command{
			newGenerateSecretCommand(),
		},
	}
}

func newGenerateSecretCommand() *cli.Command {
	return &cli.Command{
		Name:  "secret",
		Usage: "Generate a secret token",
		Commands: []*cli.Command{
			newGenerateInternalTokenCommand(),
			newGenerateLfsJWTSecretCommand(),
			newGenerateSecretKeyCommand(),
		},
	}
}

func newGenerateInternalTokenCommand() *cli.Command {
	return &cli.Command{
		Name:   "INTERNAL_TOKEN",
		Usage:  "Generate a new INTERNAL_TOKEN",
		Action: runGenerateInternalToken,
	}
}

func newGenerateLfsJWTSecretCommand() *cli.Command {
	return &cli.Command{
		Name:    "JWT_SECRET",
		Aliases: []string{"LFS_JWT_SECRET"},
		Usage:   "Generate a new JWT_SECRET",
		Action:  runGenerateLfsJwtSecret,
	}
}

func newGenerateSecretKeyCommand() *cli.Command {
	return &cli.Command{
		Name:   "SECRET_KEY",
		Usage:  "Generate a new SECRET_KEY",
		Action: runGenerateSecretKey,
	}
}

func runGenerateInternalToken(_ context.Context, c *cli.Command) error {
	internalToken, err := generate.NewInternalToken()
	if err != nil {
		return err
	}

	fmt.Printf("%s", internalToken)

	if isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Printf("\n")
	}

	return nil
}

func runGenerateLfsJwtSecret(_ context.Context, c *cli.Command) error {
	_, jwtSecretBase64 := generate.NewJwtSecretWithBase64()
	fmt.Printf("%s", jwtSecretBase64)

	if isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Printf("\n")
	}

	return nil
}

func runGenerateSecretKey(_ context.Context, c *cli.Command) error {
	secretKey, err := generate.NewSecretKey()
	if err != nil {
		return err
	}

	// codeql[disable-next-line=go/clear-text-logging]
	fmt.Printf("%s", secretKey)

	if isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Printf("\n")
	}

	return nil
}
