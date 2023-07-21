// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package main

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
)

// these flags will be set by the build flags
var (
	Version     = "development" // program version for this build
	Tags        = ""            // the Golang build tags
	MakeVersion = ""            // "make" program version if built with make
)

func init() {
	setting.AppVer = Version
	setting.AppBuiltWith = formatBuiltWith()
	setting.AppStartTime = time.Now().UTC()
}

func main() {
	app := cmd.NewMainApp()
	app.Name = "Gitea"
	app.Usage = "A painless self-hosted Git service"
	app.Description = `By default, Gitea will start serving using the web-server with no argument, which can alternatively be run by running the subcommand "web".`
	app.Version = Version + formatBuiltWith()

	err := app.Run(os.Args)
	if err != nil {
		_, _ = fmt.Fprintf(app.Writer, "\nFailed to run with %s: %v\n", os.Args, err)
	}

	log.GetManager().Close()
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
