// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"gitea.dev/modules/generate"
	"gitea.dev/modules/ssh"

	"github.com/mattn/go-isatty"
	"github.com/urfave/cli/v3"
)

func newGenerateCommand() *cli.Command {
	return &cli.Command{
		Name:  "generate",
		Usage: "Generate Gitea's secrets/keys/tokens",
		Commands: []*cli.Command{
			newGenerateSecretCommand(),
			newGenerateSSHCommand(),
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

func newGenerateSSHCommand() *cli.Command {
	return &cli.Command{
		Name:  "ssh",
		Usage: "Generate ssh keys",
		Commands: []*cli.Command{
			newGenerateSSHKeyCommand(),
			newGenerateSSHHostKeysCommand(),
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

func newGenerateSSHKeyCommand() *cli.Command {
	return &cli.Command{
		Name:  "key",
		Usage: "Generate a new ssh key",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "bits", Aliases: []string{"b"}, Usage: "Number of bits in the key, ignored when key is ed25519"},
			&cli.StringFlag{Name: "type", Aliases: []string{"t"}, Value: "ed25519", Usage: "Specifies the type of key to create."},
			&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "Specifies the path or base directory for the key file", Required: true},
		},
		Action: runGenerateKeyPair,
	}
}

func newGenerateSSHHostKeysCommand() *cli.Command {
	return &cli.Command{
		Name:  "host-keys",
		Usage: "Generate host keys of all default key types (rsa, ecdsa, and ed25519) if they do not already exist.",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "dir", Aliases: []string{"d"}, Usage: "Specifies the base directory for the key files", Required: true},
		},
		Action: runGenerateHostKey,
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

func runGenerateHostKey(_ context.Context, c *cli.Command) error {
	file := c.String("dir")
	info, err := os.Stat(file)
	if errors.Is(err, os.ErrNotExist) {
		if err = os.MkdirAll(file, 0o644); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if !info.IsDir() {
		return errors.New("file already exists and is not a directory")
	}
	fmt.Fprintf(c.Writer, "Generating host keys in %s\n", file)
	_, err = ssh.InitDefaultHostKeys(file)
	return err
}

func runGenerateKeyPair(_ context.Context, c *cli.Command) error {
	file := c.String("file")
	keyType := c.String("type")

	fmt.Fprintf(c.Writer, "Generating public/private %s key pair.\n", keyType)

	// Check if file exists to prevent overwriting
	if _, err := os.Stat(file); err == nil {
		if !confirm(c.Reader, c.Writer, "%s already exists.\nOverwrite (y/n)? ", file) {
			fmt.Println("Aborting")
			return nil
		}
	}
	bits := c.Int("bits")
	err := ssh.GenKeyPair(file, generate.SSHKeyType(keyType), bits)
	if err == nil {
		fmt.Printf("Your SSH key has been saved in %s\n", file)
	}
	return err
}
