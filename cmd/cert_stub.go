// +build !cert

// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"

	"github.com/urfave/cli"
)

// CmdCert represents the available cert sub-command.
var CmdCert = cli.Command{
	Name:        "cert",
	Usage:       "Generate self-signed certificate",
	Description: `Please use build tags "cert" to rebuild Gogs in order to have this ability`,
	Action:      runCert,
}

func runCert(ctx *cli.Context) error {
	fmt.Println("Command cert not available, please use build tags 'cert' to rebuild.")
	os.Exit(1)

	return nil
}
