// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"runtime"
	"strings"
	"time"

	"gitea.dev/cmd"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"

	// register supported doc types
	_ "gitea.dev/modules/markup/console"
	_ "gitea.dev/modules/markup/csv"
	_ "gitea.dev/modules/markup/jupyter"
	_ "gitea.dev/modules/markup/markdown"
	_ "gitea.dev/modules/markup/orgmode"

	"github.com/urfave/cli/v3"
)

// these flags will be set by the build flags
var (
	Version = "development" // program version for this build
	Tags    = ""            // the Golang build tags
)

func init() {
	setting.AppVer = Version
	setting.AppBuiltWith = formatBuiltWith()
	setting.AppStartTime = time.Now().UTC()
}

func main() {
	cli.OsExiter = func(code int) {
		log.GetManager().Close()
		os.Exit(code)
	}
	app := cmd.NewMainApp(cmd.AppVersion{Version: Version, Extra: formatBuiltWith()})
	_ = cmd.RunMainApp(app, os.Args...) // all errors should have been handled by the RunMainApp
	// flush the queued logs before exiting, it is a MUST, otherwise there will be log loss
	log.GetManager().Close()
}

func formatBuiltWith() string {
	version := runtime.Version()
	if len(Tags) == 0 {
		return " built with " + version
	}

	return " built with " + version + " : " + strings.ReplaceAll(Tags, " ", ", ")
}
