// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// checklist represents list for all checks
var checklist = []check{
	{
		title: "Check if OpenSSH authorized_keys file id correct",
		f:     runDoctorLocationMoved,
	},
	// more checks please append here
}

func runDoctor(ctx *cli.Context) error {
	err := initDB()
	fmt.Println("Using app.ini at", setting.CustomConf)
	if err != nil {
		fmt.Println(err)
		fmt.Println("Check if you are using the right config file. You can use a --config directive to specify one.")
		return nil
	}

	for i, check := range checklist {
		fmt.Println("[", i+1, "]", check.title)
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
	if setting.SSH.StartBuiltinServer || !setting.SSH.CreateAuthorizedKeysFile {
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
		firstline = strings.TrimSpace(scanner.Text())
		if len(firstline) == 0 || firstline[0] == '#' {
			continue
		}
		break
	}

	// command="/Volumes/data/Projects/gitea/gitea/gitea --config
	if len(firstline) > 0 {
		exp := regexp.MustCompile(`^[ \t]*(?:command=")([^ ]+) --config='([^']+)' serv key-([^"]+)",(?:[^ ]+) ssh-rsa ([^ ]+) ([^ ]+)[ \t]*$`)

		// command="/home/user/gitea --config='/home/user/etc/app.ini' serv key-999",option-1,option-2,option-n ssh-rsa public-key-value key-name
		res := exp.FindStringSubmatch(firstline)
		if res == nil {
			return nil, errors.New("Unknow authorized_keys format")
		}

		giteaPath := res[1] // => /home/user/gitea
		iniPath := res[2]   // => /home/user/etc/app.ini

		p, err := exePath()
		if err != nil {
			return nil, err
		}
		p, err = filepath.Abs(p)
		if err != nil {
			return nil, err
		}

		if len(giteaPath) > 0 && giteaPath != p {
			return []string{fmt.Sprintf("Gitea exe path wants %s but %s on %s", p, giteaPath, fPath)}, nil
		}
		if len(iniPath) > 0 && iniPath != setting.CustomConf {
			return []string{fmt.Sprintf("Gitea config path wants %s but %s on %s", setting.CustomConf, iniPath, fPath)}, nil
		}
	}

	return nil, nil
}
