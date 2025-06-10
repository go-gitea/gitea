// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"strings"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/auth/source/smtp"

	"github.com/urfave/cli/v3"
)

func smtpCLIFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  "name",
			Value: "",
			Usage: "Application Name",
		},
		&cli.StringFlag{
			Name:  "auth-type",
			Value: "PLAIN",
			Usage: "SMTP Authentication Type (PLAIN/LOGIN/CRAM-MD5) default PLAIN",
		},
		&cli.StringFlag{
			Name:  "host",
			Value: "",
			Usage: "SMTP Host",
		},
		&cli.IntFlag{
			Name:  "port",
			Usage: "SMTP Port",
		},
		&cli.BoolFlag{
			Name:  "force-smtps",
			Usage: "SMTPS is always used on port 465. Set this to force SMTPS on other ports.",
			Value: true,
		},
		&cli.BoolFlag{
			Name:  "skip-verify",
			Usage: "Skip TLS verify.",
			Value: true,
		},
		&cli.StringFlag{
			Name:  "helo-hostname",
			Value: "",
			Usage: "Hostname sent with HELO. Leave blank to send current hostname",
		},
		&cli.BoolFlag{
			Name:  "disable-helo",
			Usage: "Disable SMTP helo.",
			Value: true,
		},
		&cli.StringFlag{
			Name:  "allowed-domains",
			Value: "",
			Usage: "Leave empty to allow all domains. Separate multiple domains with a comma (',')",
		},
		&cli.BoolFlag{
			Name:  "skip-local-2fa",
			Usage: "Skip 2FA to log on.",
			Value: true,
		},
		&cli.BoolFlag{
			Name:  "active",
			Usage: "This Authentication Source is Activated.",
			Value: true,
		},
	}
}

func microcmdAuthUpdateSMTP() *cli.Command {
	return &cli.Command{
		Name:  "update-smtp",
		Usage: "Update existing SMTP authentication source",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return newAuthService().runUpdateSMTP(ctx, cmd)
		},
		Flags: append(smtpCLIFlags()[:1], append([]cli.Flag{&cli.Int64Flag{
			Name:  "id",
			Usage: "ID of authentication source",
		}}, smtpCLIFlags()[1:]...)...),
	}
}

func microcmdAuthAddSMTP() *cli.Command {
	return &cli.Command{
		Name:  "add-smtp",
		Usage: "Add new SMTP authentication source",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return newAuthService().runAddSMTP(ctx, cmd)
		},
		Flags: smtpCLIFlags(),
	}
}

func parseSMTPConfig(c *cli.Command, conf *smtp.Source) error {
	if c.IsSet("auth-type") {
		conf.Auth = c.String("auth-type")
		validAuthTypes := []string{"PLAIN", "LOGIN", "CRAM-MD5"}
		if !util.SliceContainsString(validAuthTypes, strings.ToUpper(c.String("auth-type"))) {
			return errors.New("Auth must be one of PLAIN/LOGIN/CRAM-MD5")
		}
		conf.Auth = c.String("auth-type")
	}
	if c.IsSet("host") {
		conf.Host = c.String("host")
	}
	if c.IsSet("port") {
		conf.Port = c.Int("port")
	}
	if c.IsSet("allowed-domains") {
		conf.AllowedDomains = c.String("allowed-domains")
	}
	if c.IsSet("force-smtps") {
		conf.ForceSMTPS = c.Bool("force-smtps")
	}
	if c.IsSet("skip-verify") {
		conf.SkipVerify = c.Bool("skip-verify")
	}
	if c.IsSet("helo-hostname") {
		conf.HeloHostname = c.String("helo-hostname")
	}
	if c.IsSet("disable-helo") {
		conf.DisableHelo = c.Bool("disable-helo")
	}
	return nil
}

func (a *authService) runAddSMTP(ctx context.Context, c *cli.Command) error {
	if err := a.initDB(ctx); err != nil {
		return err
	}

	if !c.IsSet("name") || len(c.String("name")) == 0 {
		return errors.New("name must be set")
	}
	if !c.IsSet("host") || len(c.String("host")) == 0 {
		return errors.New("host must be set")
	}
	if !c.IsSet("port") {
		return errors.New("port must be set")
	}
	active := true
	if c.IsSet("active") {
		active = c.Bool("active")
	}

	var smtpConfig smtp.Source
	if err := parseSMTPConfig(c, &smtpConfig); err != nil {
		return err
	}

	// If not set default to PLAIN
	if len(smtpConfig.Auth) == 0 {
		smtpConfig.Auth = "PLAIN"
	}

	return a.createAuthSource(ctx, &auth_model.Source{
		Type:            auth_model.SMTP,
		Name:            c.String("name"),
		IsActive:        active,
		Cfg:             &smtpConfig,
		TwoFactorPolicy: util.Iif(c.Bool("skip-local-2fa"), "skip", ""),
	})
}

func (a *authService) runUpdateSMTP(ctx context.Context, c *cli.Command) error {
	if !c.IsSet("id") {
		return errors.New("--id flag is missing")
	}

	if err := a.initDB(ctx); err != nil {
		return err
	}

	source, err := a.getAuthSourceByID(ctx, c.Int64("id"))
	if err != nil {
		return err
	}

	smtpConfig := source.Cfg.(*smtp.Source)

	if err := parseSMTPConfig(c, smtpConfig); err != nil {
		return err
	}

	if c.IsSet("name") {
		source.Name = c.String("name")
	}

	if c.IsSet("active") {
		source.IsActive = c.Bool("active")
	}

	source.Cfg = smtpConfig
	source.TwoFactorPolicy = util.Iif(c.Bool("skip-local-2fa"), "skip", "")
	return a.updateAuthSource(ctx, source)
}
