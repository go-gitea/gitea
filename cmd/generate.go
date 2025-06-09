// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"bufio"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"code.gitea.io/gitea/modules/generate"

	"github.com/mattn/go-isatty"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"
)

var (
	// CmdGenerate represents the available generate sub-command.
	CmdGenerate = &cli.Command{
		Name:  "generate",
		Usage: "Generate Gitea's secrets/keys/tokens",
		Subcommands: []*cli.Command{
			subcmdSecret,
			subcmdKeygen,
		},
	}

	subcmdSecret = &cli.Command{
		Name:  "secret",
		Usage: "Generate a secret token",
		Subcommands: []*cli.Command{
			microcmdGenerateInternalToken,
			microcmdGenerateLfsJwtSecret,
			microcmdGenerateSecretKey,
		},
	}
	keygenFlags = []cli.Flag{
		&cli.StringFlag{Name: "bits", Aliases: []string{"b"}, Usage: "Number of bits in the key, ignored when key is ed25519"},
		&cli.StringFlag{Name: "type", Aliases: []string{"t"}, Value: "ed25519", Usage: "Keytype to generate"},
		&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "Specifies the filename of the key file", Required: true},
	}
	subcmdKeygen = &cli.Command{
		Name:   "ssh-keygen",
		Usage:  "Generate a ssh keypair",
		Flags:  keygenFlags,
		Action: runGenerateKeyPair,
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

func runGenerateKeyPair(c *cli.Context) error {
	file := c.String("file")

	// Check if file exists to prevent overwrites
	if _, err := os.Stat(file); err == nil {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Printf("%s already exists.\nOverwrite (y/n)? ", file)
		scanner.Scan()
		if strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
			fmt.Println("Aborting")
			return nil
		}
	}
	keytype := c.String("type")
	bits := c.Int("bits")
	// provide defaults for bits, ed25519 ignores bit length so it's omitted
	if bits == 0 {
		if keytype == "rsa" {
			bits = 3072
		} else {
			bits = 256
		}
	}

	pub, priv, err := generate.NewSSHKey(keytype, bits)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	err = pem.Encode(f, priv)
	if err != nil {
		return err
	}
	fmt.Printf("Your identification has been saved in %s\n", file)
	err = os.WriteFile(file+".pub", ssh.MarshalAuthorizedKey(pub), 0o644)
	if err != nil {
		return err
	}
	fmt.Printf("Your public key has been saved in %s", file+".pub")
	return nil
}
