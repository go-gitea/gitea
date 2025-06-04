// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli/v3"
)

// cmdHelp is our own help subcommand with more information
// Keep in mind that the "./gitea help"(subcommand) is different from "./gitea --help"(flag), the flag doesn't parse the config or output "DEFAULT CONFIGURATION:" information
func cmdHelp() *cli.Command {
	c := &cli.Command{
		Name:      "help",
		Aliases:   []string{"h"},
		Usage:     "Shows a list of commands or help for one command",
		ArgsUsage: "[command]",
		Action: func(ctx context.Context, c *cli.Command) (err error) {
			lineage := c.Lineage() // The order is from child to parent: help, doctor, Gitea
			targetCmdIdx := 0
			if c.Name == "help" {
				targetCmdIdx = 1
			}
			if lineage[targetCmdIdx] != lineage[targetCmdIdx].Root() {
				err = cli.ShowCommandHelp(ctx, lineage[targetCmdIdx+1] /* parent cmd */, lineage[targetCmdIdx].Name /* sub cmd */)
			} else {
				err = cli.ShowAppHelp(c)
			}
			_, _ = fmt.Fprintf(c.Root().Writer, `
DEFAULT CONFIGURATION:
   AppPath:    %s
   WorkPath:   %s
   CustomPath: %s
   ConfigFile: %s

`, setting.AppPath, setting.AppWorkPath, setting.CustomPath, setting.CustomConf)
			return err
		},
	}
	return c
}

func appGlobalFlags() []cli.Flag {
	return []cli.Flag{
		// make the builtin flags at the top
		cli.HelpFlag,

		// shared configuration flags, they are for global and for each sub-command at the same time
		// eg: such command is valid: "./gitea --config /tmp/app.ini web --config /tmp/app.ini", while it's discouraged indeed
		// keep in mind that the short flags like "-C", "-c" and "-w" are globally polluted, they can't be used for sub-commands anymore.
		&cli.StringFlag{
			Name:    "custom-path",
			Aliases: []string{"C"},
			Usage:   "Set custom path (defaults to '{WorkPath}/custom')",
		},
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   setting.CustomConf,
			Usage:   "Set custom config file (defaults to '{WorkPath}/custom/conf/app.ini')",
		},
		&cli.StringFlag{
			Name:    "work-path",
			Aliases: []string{"w"},
			Usage:   "Set Gitea's working path (defaults to the Gitea's binary directory)",
		},
	}
}

func prepareSubcommandWithGlobalFlags(command *cli.Command) {
	command.Flags = append(append([]cli.Flag{}, appGlobalFlags()...), command.Flags...)
	command.Action = prepareWorkPathAndCustomConf(command.Action)
	command.HideHelp = true
	if command.Name != "help" {
		command.Commands = append(command.Commands, cmdHelp())
	}
	for i := range command.Commands {
		prepareSubcommandWithGlobalFlags(command.Commands[i])
	}
}

// prepareWorkPathAndCustomConf wraps the Action to prepare the work path and custom config
// It can't use "Before", because each level's sub-command's Before will be called one by one, so the "init" would be done multiple times
func prepareWorkPathAndCustomConf(action cli.ActionFunc) func(context.Context, *cli.Command) error {
	return func(ctx context.Context, cmd *cli.Command) error {
		var args setting.ArgWorkPathAndCustomConf
		// from children to parent, check the global flags
		for _, curCtx := range cmd.Lineage() {
			if curCtx.IsSet("work-path") && args.WorkPath == "" {
				args.WorkPath = curCtx.String("work-path")
			}
			if curCtx.IsSet("custom-path") && args.CustomPath == "" {
				args.CustomPath = curCtx.String("custom-path")
			}
			if curCtx.IsSet("config") && args.CustomConf == "" {
				args.CustomConf = curCtx.String("config")
			}
		}
		setting.InitWorkPathAndCommonConfig(os.Getenv, args)
		if cmd.Bool("help") || action == nil {
			// the default behavior of "urfave/cli": "nil action" means "show help"
			return cmdHelp().Action(ctx, cmd)
		}
		return action(ctx, cmd)
	}
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

	// these sub-commands need to use config file
	subCmdWithConfig := []*cli.Command{
		cmdHelp(), // the "help" sub-command was used to show the more information for "work path" and "custom config"
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

	app.DefaultCommand = CmdWeb.Name

	app.Flags = append(app.Flags, cli.VersionFlag)
	app.Flags = append(app.Flags, appGlobalFlags()...)
	app.HideHelp = true // use our own help action to show helps (with more information like default config)
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
