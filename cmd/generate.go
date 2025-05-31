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

var (
	// CmdGenerate represents the available generate sub-command.
	CmdGenerate = &cli.Command{
		Name:  "generate",
		Usage: "Generate Gitea's secrets/keys/tokens",
		Commands: []*cli.Command{
			subcmdSecret,
		},
	}

	subcmdSecret = &cli.Command{
		Name:  "secret",
		Usage: "Generate a secret token",
		Commands: []*cli.Command{
			microcmdGenerateInternalToken,
			microcmdGenerateLfsJwtSecret,
			microcmdGenerateSecretKey,
		},
	}

	microcmdGenerateInternalToken = &cli.Command{
		Name:   "INTERNAL_TOKEN",
		Usage:  "Generate a new INTERNAL_TOKEN",
		Action: runGenerateInternalToken,
	}

	microcmdGenerateLfsJwtSecret = &cli.Command{
		Name:    "JWT_SECRET",
		Aliases: []string{"LFS_JWT_SECRET"},
		Usage:   "Generate a new JWT_SECRET",
		Action:  runGenerateLfsJwtSecret,
	}

	microcmdGenerateSecretKey = &cli.Command{
		Name:   "SECRET_KEY",
		Usage:  "Generate a new SECRET_KEY",
		Action: runGenerateSecretKey,
	}
)

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
	_, jwtSecretBase64, err := generate.NewJwtSecretWithBase64()
	if err != nil {
		return err
	}

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

	fmt.Printf("%s", secretKey)

	if isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Printf("\n")
	}

	return nil
}
