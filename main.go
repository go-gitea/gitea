// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Gitea (git with a cup of tea) is a painless self-hosted Git Service.
package main // import "code.gitea.io/gitea"

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"code.gitea.io/gitea/cmd"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	// register supported doc types
	_ "code.gitea.io/gitea/modules/markup/asciicast"
	_ "code.gitea.io/gitea/modules/markup/console"
	_ "code.gitea.io/gitea/modules/markup/csv"
	_ "code.gitea.io/gitea/modules/markup/markdown"
	_ "code.gitea.io/gitea/modules/markup/orgmode"

	"github.com/urfave/cli"
)

var (
	// Version holds the current Gitea version
	Version = "development"
	// Tags holds the build tags used
	Tags = ""
	// MakeVersion holds the current Make version if built with make
	MakeVersion = ""
)

func init() {
	setting.AppVer = Version
	setting.AppBuiltWith = formatBuiltWith()
	setting.AppStartTime = time.Now().UTC()
}

// cmdHelp is our own help subcommand with more information
// test cases:
// ./gitea help
// ./gitea -h
// ./gitea web help
// ./gitea web -h (due to cli lib limitation, this won't call our cmdHelp, so no extra info)
// ./gitea admin
// ./gitea admin help
// ./gitea admin auth help
// ./gitea -c /tmp/app.ini -h
// ./gitea -c /tmp/app.ini help
// ./gitea help -c /tmp/app.ini
// GITEA_WORK_DIR=/tmp ./gitea help
// GITEA_WORK_DIR=/tmp ./gitea help --work-path /tmp/other
// GITEA_WORK_DIR=/tmp ./gitea help --config /tmp/app-other.ini
var cmdHelp = cli.Command{
	Name:      "help",
	Aliases:   []string{"h"},
	Usage:     "Shows a list of commands or help for one command",
	ArgsUsage: "[command]",
	Action: func(c *cli.Context) (err error) {
		args := c.Args()
		if args.Present() {
			err = cli.ShowCommandHelp(c, args.First())
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

func main() {
	app := cli.NewApp()
	app.Name = "Gitea"
	app.Usage = "A painless self-hosted Git service"
	app.Description = `By default, Gitea will start serving using the web-server with no argument, which can alternatively be run by running the subcommand "web".`
	app.Version = Version + formatBuiltWith()
	app.EnableBashCompletion = true
	app.Commands = []cli.Command{
		cmd.CmdWeb,
		cmd.CmdServ,
		cmd.CmdHook,
		cmd.CmdDump,
		cmd.CmdCert,
		cmd.CmdAdmin,
		cmd.CmdGenerate,
		cmd.CmdMigrate,
		cmd.CmdKeys,
		cmd.CmdConvert,
		cmd.CmdDoctor,
		cmd.CmdManager,
		cmd.Cmdembedded,
		cmd.CmdMigrateStorage,
		cmd.CmdDocs,
		cmd.CmdDumpRepository,
		cmd.CmdRestoreRepository,
		cmd.CmdActions,
	}

	// default configuration flags
	globalFlags := []cli.Flag{
		cli.HelpFlag,
		cli.StringFlag{
			Name:  "custom-path, C",
			Usage: "Set custom path (defaults to '{WorkPath}/custom')",
		},
		cli.StringFlag{
			Name:  "config, c",
			Value: setting.CustomConf,
			Usage: "Set custom config file (defaults to '{WorkPath}/custom/conf/app.ini')",
		},
		cli.StringFlag{
			Name:  "work-path, w",
			Usage: "Set Gitea's working path (defaults to the Gitea's binary directory)",
		},
	}

	// Set the default to be equivalent to cmdWeb and add the default flags
	app.Flags = append(app.Flags, globalFlags...)
	app.Flags = append(app.Flags, cmd.CmdWeb.Flags...)
	app.Action = prepareWorkPathAndCustomConf(cmd.CmdWeb.Action)
	app.HideHelp = true // use our own help action to show helps (with more information like default config)
	app.Commands = append(app.Commands, cmdHelp)
	for i := range app.Commands {
		prepareSubcommands(&app.Commands[i], globalFlags)
	}

	err := app.Run(os.Args)
	if err != nil {
		_, _ = fmt.Fprintf(app.Writer, "\nFailed to run with %s: %v\n", os.Args, err)
	}

	log.GetManager().Close()
}

func prepareSubcommands(command *cli.Command, defaultFlags []cli.Flag) {
	command.Flags = append(command.Flags, defaultFlags...)
	command.Action = prepareWorkPathAndCustomConf(command.Action)
	command.HideHelp = true
	if command.Name != "help" {
		command.Subcommands = append(command.Subcommands, cmdHelp)
	}
	for i := range command.Subcommands {
		prepareSubcommands(&command.Subcommands[i], defaultFlags)
	}
}

// prepareWorkPathAndCustomConf wraps the Action to prepare the work path and custom config
// It can't use "Before", because each level's sub-command's Before will be called one by one, so the "init" would be done multiple times
func prepareWorkPathAndCustomConf(action any) func(ctx *cli.Context) error {
	return func(ctx *cli.Context) error {
		var args setting.ArgWorkPathAndCustomConf
		curCtx := ctx
		for curCtx != nil {
			if curCtx.IsSet("work-path") && args.WorkPath == "" {
				args.WorkPath = curCtx.String("work-path")
			}
			if curCtx.IsSet("custom-path") && args.CustomPath == "" {
				args.CustomPath = curCtx.String("custom-path")
			}
			if curCtx.IsSet("config") && args.CustomConf == "" {
				args.CustomConf = curCtx.String("config")
			}
			curCtx = curCtx.Parent()
		}
		setting.InitWorkPathAndCommonConfig(os.Getenv, args)
		if ctx.Bool("help") || action == nil {
			// the default behavior of "urfave/cli": "nil action" means "show help"
			return cmdHelp.Action.(func(ctx *cli.Context) error)(ctx)
		}
		return action.(func(*cli.Context) error)(ctx)
	}
}

func formatBuiltWith() string {
	version := runtime.Version()
	if len(MakeVersion) > 0 {
		version = MakeVersion + ", " + runtime.Version()
	}
	if len(Tags) == 0 {
		return " built with " + version
	}

	return " built with " + version + " : " + strings.ReplaceAll(Tags, " ", ", ")
}
