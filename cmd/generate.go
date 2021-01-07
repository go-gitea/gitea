// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/base64"
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/generate"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/secrets"

	"github.com/mattn/go-isatty"
	"github.com/urfave/cli"
)

var (
	// CmdGenerate represents the available generate sub-command.
	CmdGenerate = cli.Command{
		Name:  "generate",
		Usage: "Command line interface for running generators",
		Subcommands: []cli.Command{
			subcmdSecret,
		},
	}

	subcmdSecret = cli.Command{
		Name:  "secret",
		Usage: "Generate a secret token",
		Subcommands: []cli.Command{
			microcmdGenerateInternalToken,
			microcmdGenerateLfsJwtSecret,
			microcmdGenerateSecretKey,
			microcmdGenerateMasterKey,
		},
	}

	microcmdGenerateInternalToken = cli.Command{
		Name:   "INTERNAL_TOKEN",
		Usage:  "Generate a new INTERNAL_TOKEN",
		Action: runGenerateInternalToken,
	}

	microcmdGenerateLfsJwtSecret = cli.Command{
		Name:    "JWT_SECRET",
		Aliases: []string{"LFS_JWT_SECRET"},
		Usage:   "Generate a new JWT_SECRET",
		Action:  runGenerateLfsJwtSecret,
	}

	microcmdGenerateSecretKey = cli.Command{
		Name:   "SECRET_KEY",
		Usage:  "Generate a new SECRET_KEY",
		Action: runGenerateSecretKey,
	}

	microcmdGenerateMasterKey = cli.Command{
		Name:   "MASTER_KEY",
		Usage:  "Generate a new MASTER_KEY",
		Action: runGenerateMasterKey,
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

func runGenerateLfsJwtSecret(c *cli.Context) error {
	JWTSecretBase64, err := generate.NewJwtSecretBase64()
	if err != nil {
		return err
	}

	fmt.Printf("%s", JWTSecretBase64)

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

func runGenerateMasterKey(c *cli.Context) error {
	// Silence the console logger
	log.DelNamedLogger("console")
	log.DelNamedLogger(log.DEFAULT)

	// Read configuration file
	setting.LoadFromExisting()

	providerType := secrets.MasterKeyProviderType(setting.MasterKeyProvider)
	if providerType == secrets.MasterKeyProviderTypeNone {
		return fmt.Errorf("configured master key provider does not support key generation")
	}

	if err := secrets.Init(); err != nil {
		return err
	}

	scrts, err := secrets.GenerateMasterKey()
	if err != nil {
		return err
	}

	if len(scrts) > 1 {
		fmt.Println("Unseal secrets:")
		for i, secret := range scrts {
			if i > 0 {
				fmt.Printf("\n")
			}
			fmt.Printf("%s\n", base64.StdEncoding.EncodeToString(secret))
		}
	}

	if providerType == secrets.MasterKeyProviderTypePlain && len(scrts) == 1 {
		fmt.Printf("%s", base64.StdEncoding.EncodeToString(scrts[0]))

		if isatty.IsTerminal(os.Stdout.Fd()) {
			fmt.Printf("\n")
		}
	}

	return nil
}
