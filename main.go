// +build go1.4

// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Gogs (Go Git Service) is a painless self-hosted Git Service.
package main

import (
	"os"
	"runtime"

	"github.com/go-gitea/gitea/cmd"
	"github.com/go-gitea/gitea/modules/setting"
	"github.com/urfave/cli"
)

const APP_VER = "0.9.99.0915"

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	setting.AppVer = APP_VER
}

func main() {
	app := cli.NewApp()
	app.Name = "Gogs"
	app.Usage = "Go Git Service: a painless self-hosted Git service"
	app.Version = APP_VER
	app.Commands = []cli.Command{
		cmd.CmdWeb,
		cmd.CmdServ,
		cmd.CmdUpdate,
		cmd.CmdDump,
		cmd.CmdCert,
		cmd.CmdAdmin,
	}
	app.Flags = append(app.Flags, []cli.Flag{}...)
	app.Run(os.Args)
}
