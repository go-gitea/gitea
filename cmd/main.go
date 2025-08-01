// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli/v3"
)

var cliHelpPrinterOld = cli.HelpPrinter

func init() {
	cli.HelpPrinter = cliHelpPrinterNew
}

// cliHelpPrinterNew helps to print "DEFAULT CONFIGURATION" for the following cases ( "-c" can apper in any position):
// * ./gitea -c /dev/null -h
// * ./gitea -c help /dev/null help
// * ./gitea help -c /dev/null
// * ./gitea help -c /dev/null web
// * ./gitea help web -c /dev/null
// * ./gitea web help -c /dev/null
// * ./gitea web -h -c /dev/null
func cliHelpPrinterNew(out io.Writer, templ string, data any) {
	cmd, _ := data.(*cli.Command)
	if cmd != nil {
		prepareWorkPathAndCustomConf(cmd)
	}
	cliHelpPrinterOld(out, templ, data)
	if setting.CustomConf != "" {
		_, _ = fmt.Fprintf(out, `
DEFAULT CONFIGURATION:
   AppPath:    %s
   WorkPath:   %s
   CustomPath: %s
   ConfigFile: %s

`, setting.AppPath, setting.AppWorkPath, setting.CustomPath, setting.CustomConf)
	}
}

func prepareSubcommandWithGlobalFlags(originCmd *cli.Command) {
	originBefore := originCmd.Before
	originCmd.Before = func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
		prepareWorkPathAndCustomConf(cmd)
		if originBefore != nil {
			return originBefore(ctx, cmd)
		}
		return ctx, nil
	}
}

// prepareWorkPathAndCustomConf tries to prepare the work path, custom path and custom config from various inputs:
// command line flags, environment variables, config file
func prepareWorkPathAndCustomConf(cmd *cli.Command) {
	var args setting.ArgWorkPathAndCustomConf
	if cmd.IsSet("work-path") {
		args.WorkPath = cmd.String("work-path")
	}
	if cmd.IsSet("custom-path") {
		args.CustomPath = cmd.String("custom-path")
	}
	if cmd.IsSet("config") {
		args.CustomConf = cmd.String("config")
	}
	setting.InitWorkPathAndCommonConfig(os.Getenv, args)
}

type AppVersion struct {
	Version string
	Extra   string
}

func NewMainApp(appVer AppVersion) *cli.Command {
	app := &cli.Command{}
	app.Name = "gitea" // must be lower-cased because it appears in the "USAGE" section like "gitea doctor [command [command options]]"
	app.Usage = "A painless self-hosted Git service"
	app.Description = `Gitea program contains "web" and other subcommands. If no subcommand is given, it starts the web server by default. Use "web" subcommand for more web server arguments, use other subcommands for other purposes.`
	app.Version = appVer.Version + appVer.Extra
	app.EnableShellCompletion = true
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:      "work-path",
			Aliases:   []string{"w"},
			TakesFile: true,
			Usage:     "Set Gitea's working path (defaults to the Gitea's binary directory)",
		},
		&cli.StringFlag{
			Name:      "config",
			Aliases:   []string{"c"},
			TakesFile: true,
			Value:     setting.CustomConf,
			Usage:     "Set custom config file (defaults to '{WorkPath}/custom/conf/app.ini')",
		},
		&cli.StringFlag{
			Name:      "custom-path",
			Aliases:   []string{"C"},
			TakesFile: true,
			Usage:     "Set custom path (defaults to '{WorkPath}/custom')",
		},
	}
	// these sub-commands need to use a config file
	subCmdWithConfig := []*cli.Command{
		CmdWeb,
		CmdServ,
		CmdHook,
		CmdKeys,
		CmdDump,
		CmdAdmin,
		CmdMigrate,
		CmdDoctor,
		CmdManager,
		CmdEmbedded,
		CmdMigrateStorage,
		CmdDumpRepository,
		CmdRestoreRepository,
		CmdActions,
	}

	// these sub-commands do not need the config file, and they do not depend on any path or environment variable.
	subCmdStandalone := []*cli.Command{
		cmdCert(),
		CmdGenerate,
		CmdDocs,
	}

	// TODO: we should eventually drop the default command,
	// but not sure whether it would break Windows users who used to double-click the EXE to run.
	app.DefaultCommand = CmdWeb.Name

	app.Before = PrepareConsoleLoggerLevel(log.INFO)
	for i := range subCmdWithConfig {
		prepareSubcommandWithGlobalFlags(subCmdWithConfig[i])
	}
	app.Commands = append(app.Commands, subCmdWithConfig...)
	app.Commands = append(app.Commands, subCmdStandalone...)

	setting.InitGiteaEnvVars()
	return app
}

func RunMainApp(app *cli.Command, args ...string) error {
	ctx, cancel := installSignals()
	defer cancel()
	err := app.Run(ctx, args)
	if err == nil {
		return nil
	}
	if strings.HasPrefix(err.Error(), "flag provided but not defined:") {
		// the cli package should already have output the error message, so just exit
		cli.OsExiter(1)
		return err
	}
	_, _ = fmt.Fprintf(app.ErrWriter, "Command error: %v\n", err)
	cli.OsExiter(1)
	return err
}
