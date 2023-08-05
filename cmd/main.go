// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/urfave/cli/v2"
)

// cmdHelp is our own help subcommand with more information
func cmdHelp() *cli.Command {
	c := &cli.Command{
		Name:      "help",
		Aliases:   []string{"h"},
		Usage:     "Shows a list of commands or help for one command",
		ArgsUsage: "[command]",
		Action: func(c *cli.Context) (err error) {
			lineage := c.Lineage() // The order is from child to parent: help, doctor, Gitea, {Command:nil}
			targetCmdIdx := 0
			if c.Command.Name == "help" {
				targetCmdIdx = 1
			}
			if lineage[targetCmdIdx+1].Command != nil {
				err = cli.ShowCommandHelp(lineage[targetCmdIdx+1], lineage[targetCmdIdx].Command.Name)
			} else {
				err = cli.ShowAppHelp(c)
			}
			_, _ = fmt.Fprintf(c.App.Writer, `
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

var helpFlag = cli.HelpFlag

func init() {
	// cli.HelpFlag = nil TODO: after https://github.com/urfave/cli/issues/1794 we can use this
}

func appGlobalFlags() []cli.Flag {
	return []cli.Flag{
		// make the builtin flags at the top
		helpFlag,

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

func prepareSubcommandWithConfig(command *cli.Command, globalFlags []cli.Flag) {
	command.Flags = append(append([]cli.Flag{}, globalFlags...), command.Flags...)
	command.Action = prepareWorkPathAndCustomConf(command.Action)
	command.HideHelp = true
	if command.Name != "help" {
		command.Subcommands = append(command.Subcommands, cmdHelp())
	}
	for i := range command.Subcommands {
		prepareSubcommandWithConfig(command.Subcommands[i], globalFlags)
	}
}

// prepareWorkPathAndCustomConf wraps the Action to prepare the work path and custom config
// It can't use "Before", because each level's sub-command's Before will be called one by one, so the "init" would be done multiple times
func prepareWorkPathAndCustomConf(action cli.ActionFunc) func(ctx *cli.Context) error {
	return func(ctx *cli.Context) error {
		var args setting.ArgWorkPathAndCustomConf
		// from children to parent, check the global flags
		for _, curCtx := range ctx.Lineage() {
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
		if ctx.Bool("help") || action == nil {
			// the default behavior of "urfave/cli": "nil action" means "show help"
			return cmdHelp().Action(ctx)
		}
		return action(ctx)
	}
}

func NewMainApp(version, versionExtra string) *cli.App {
	app := cli.NewApp()
	app.Name = "Gitea"
	app.Usage = "A painless self-hosted Git service"
	app.Description = `By default, Gitea will start serving using the web-server with no argument, which can alternatively be run by running the subcommand "web".`
	app.Version = version + versionExtra
	app.EnableBashCompletion = true

	// these sub-commands need to use config file
	subCmdWithConfig := []*cli.Command{
		CmdWeb,
		CmdServ,
		CmdHook,
		CmdDump,
		CmdAdmin,
		CmdMigrate,
		CmdKeys,
		CmdDoctor,
		CmdManager,
		CmdEmbedded,
		CmdMigrateStorage,
		CmdDumpRepository,
		CmdRestoreRepository,
		CmdActions,
		cmdHelp(), // the "help" sub-command was used to show the more information for "work path" and "custom config"
	}

	cmdConvert := util.ToPointer(*cmdDoctorConvert)
	cmdConvert.Hidden = true // still support the legacy "./gitea doctor" by the hidden sub-command, remove it in next release
	subCmdWithConfig = append(subCmdWithConfig, cmdConvert)

	// these sub-commands do not need the config file, and they do not depend on any path or environment variable.
	subCmdStandalone := []*cli.Command{
		CmdCert,
		CmdGenerate,
		CmdDocs,
	}

	app.DefaultCommand = CmdWeb.Name

	globalFlags := appGlobalFlags()
	app.Flags = append(app.Flags, cli.VersionFlag)
	app.Flags = append(app.Flags, globalFlags...)
	app.HideHelp = true // use our own help action to show helps (with more information like default config)
	app.Before = PrepareConsoleLoggerLevel(log.INFO)
	for i := range subCmdWithConfig {
		prepareSubcommandWithConfig(subCmdWithConfig[i], globalFlags)
	}
	app.Commands = append(app.Commands, subCmdWithConfig...)
	app.Commands = append(app.Commands, subCmdStandalone...)

	return app
}

func RunMainApp(app *cli.App, args ...string) error {
	err := app.Run(args)
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
