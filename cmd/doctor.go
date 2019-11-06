// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/urfave/cli"

	"code.gitea.io/gitea/modules/setting"
)

// CmdDoctor represents the available doctor sub-command.
var CmdDoctor = cli.Command{
	Name:        "doctor",
	Usage:       "Diagnose the problems",
	Description: "A command to diagnose the problems of current gitea instance according the given configuration.",
	Action:      runDoctor,
}

func runDoctor(ctx *cli.Context) error {
	if err := initDB(); err != nil {
		return err
	}

	runDoctorLocationMoved(ctx)
}

func runDoctorLocationMoved(ctx *cliContext) {
	setting.RepoRootPath
}
