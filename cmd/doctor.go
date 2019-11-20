// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/setting"
	"github.com/urfave/cli"
)

// CmdDoctor represents the available doctor sub-command.
var CmdDoctor = cli.Command{
	Name:        "doctor",
	Usage:       "Diagnose the problems",
	Description: "A command to diagnose the problems of current gitea instance according the given configuration.",
	Action:      runDoctor,
}

type check struct {
	title string
	f     func(ctx *cli.Context) ([]string, error)
}

var checklist = []check{
	{
		title: "Check if openssh authorized_keys file correct",
		f:     runDoctorLocationMoved,
	},
}

func runDoctor(ctx *cli.Context) error {
	if err := initDB(); err != nil {
		return err
	}

	for i, check := range checklist {
		fmt.Println("[", i+1, "]", check.title)
		fmt.Println()
		if messages, err := check.f(ctx); err != nil {
			fmt.Println("Error:", err)
		} else if len(messages) > 0 {
			for _, message := range messages {
				fmt.Println("-", message)
			}
		} else {
			fmt.Println("OK.")
		}
		fmt.Println()
	}
	return nil
}

func exePath() (string, error) {
	file, err := exec.LookPath(os.Args[0])
	if err != nil {
		return "", err
	}
	return filepath.Abs(file)
}

func runDoctorLocationMoved(ctx *cli.Context) ([]string, error) {
	if setting.SSH.StartBuiltinServer {
		return nil, nil
	}

	fPath := filepath.Join(setting.SSH.RootPath, "authorized_keys")
	f, err := os.Open(fPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var firstline string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		firstline = scanner.Text()
		if !strings.HasPrefix(firstline, "#") {
			break
		}
	}

	// command="/Volumes/data/Projects/gitea/gitea/gitea --config
	if len(firstline) > 0 {
		var start, end int
		for i, c := range firstline {
			if c == ' ' {
				end = i
				break
			} else if c == '"' {
				start = i + 1
			}
		}
		if start > 0 && end > 0 {
			p, err := exePath()
			if err != nil {
				return nil, err
			}
			p, err = filepath.Abs(p)
			if err != nil {
				return nil, err
			}

			if firstline[start:end] != p {
				return []string{fmt.Sprintf("Wants %s but %s on %s", p, firstline[start:end], fPath)}, nil
			}
		}
	}

	return nil, nil
}
