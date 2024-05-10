// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/generate"

	"github.com/mattn/go-isatty"
	"github.com/urfave/cli/v2"
)

var (
	// CmdGenerate represents the available generate sub-command.
	CmdGenerate = &cli.Command{
		Name:  "generate",
		Usage: "Generate Gitea's secrets/keys/tokens",
		Subcommands: []*cli.Command{
			subcmdSecret,
		},
	}

	subcmdSecret = &cli.Command{
		Name:  "secret",
		Usage: "Generate a secret token",
		Subcommands: []*cli.Command{
			microcmdGenerateInternalToken,
			microcmdGenerateGeneralWebSecret,
			microcmdGenerateSecretKey,
		},
	}

	microcmdGenerateInternalToken = &cli.Command{
		Name:   "INTERNAL_TOKEN",
		Usage:  "Generate a new INTERNAL_TOKEN",
		Action: runGenerateInternalToken,
	}

	microcmdGenerateSecretKey = &cli.Command{
		Name:   "SECRET_KEY",
		Usage:  "Generate a new SECRET_KEY",
		Action: runGenerateSecretKey,
	}

	microcmdGenerateGeneralWebSecret = &cli.Command{
		Name:   "GENERAL_WEB_SECRET",
		Usage:  "Generate a new GENERAL_WEB_SECRET",
		Action: runGenerateGeneralWebSecret,
	}
)

func runGenerateInternalToken(c *cli.Context) error {
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

func runGenerateGeneralWebSecret(c *cli.Context) error {
	_, webSecretBase64, err := generate.NewGeneralWebSecretWithBase64()
	if err != nil {
		return err
	}
	fmt.Printf("%s", webSecretBase64)
	if isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Printf("\n")
	}
	return nil
}

func runGenerateSecretKey(c *cli.Context) error {
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
