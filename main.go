// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Gitea (git with a cup of tea) is a painless self-hosted Git Service.
package main // import "code.gitea.io/gitea"

import (
	"os"
	"runtime"
	"strings"

	"code.gitea.io/gitea/cmd"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	// register supported doc types
	_ "code.gitea.io/gitea/modules/markup/csv"
	_ "code.gitea.io/gitea/modules/markup/markdown"
	_ "code.gitea.io/gitea/modules/markup/orgmode"

	"github.com/urfave/cli"
)

var (
	// Version holds the current Gitea version
	Version = "1.9.0-dev"
	// Tags holds the build tags used
	Tags = ""
	// MakeVersion holds the current Make version if built with make
	MakeVersion = ""
)

func init() {
	setting.AppVer = Version
	setting.AppBuiltWith = formatBuiltWith(Tags)
}

func main() {
	app := cli.NewApp()
	app.Name = "Gitea"
	app.Usage = "A painless self-hosted Git service"
	app.Description = `By default, gitea will start serving using the webserver with no
arguments - which can alternatively be run by running the subcommand web.`
	app.Version = Version + formatBuiltWith(Tags)
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
	}
	app.Flags = append(app.Flags, cmd.CmdWeb.Flags...)
	app.Action = cmd.CmdWeb.Action
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal("Failed to run app with %s: %v", os.Args, err)
	}
}

func formatBuiltWith(makeTags string) string {
	var version = runtime.Version()
	if len(MakeVersion) > 0 {
		version = MakeVersion + ", " + runtime.Version()
	}
	if len(Tags) == 0 {
		return " built with " + version
	}

	return " built with " + version + " : " + strings.Replace(tags, " ", ", ", -1)
}
