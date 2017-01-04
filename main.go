// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Gitea (git with a cup of tea) is a painless self-hosted Git Service.
package main // import "code.gitea.io/gitea"

import (
	"os"

	"code.gitea.io/gitea/cmd"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"github.com/urfave/cli"
)

// Version holds the current Gitea version
var Version = "1.1.0+dev"

func init() {
	setting.AppVer = Version
}

func main() {
	app := cli.NewApp()
	app.Name = "Gitea"
	app.Usage = "A painless self-hosted Git service"
	app.Version = Version
	app.Commands = []cli.Command{
		cmd.CmdWeb,
		cmd.CmdServ,
		cmd.CmdUpdate,
		cmd.CmdDump,
		cmd.CmdCert,
		cmd.CmdAdmin,
	}
	app.Flags = append(app.Flags, []cli.Flag{}...)
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(4, "Fail to run app with %s: %v", os.Args, err)
	}

}
