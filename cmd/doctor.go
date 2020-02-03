// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli"
)

// CmdDoctor represents the available doctor sub-command.
var CmdDoctor = cli.Command{
	Name:        "doctor",
	Usage:       "Diagnose problems",
	Description: "A command to diagnose problems with the current Gitea instance according to the given configuration.",
	Action:      runDoctor,
}

type check struct {
	title            string
	f                func(ctx *cli.Context) ([]string, error)
	abortIfFailed    bool
	skipDatabaseInit bool
}

// checklist represents list for all checks
var checklist = []check{
	{
		// NOTE: this check should be the first in the list
		title:            "Check paths and basic configuration",
		f:                runDoctorPathInfo,
		abortIfFailed:    true,
		skipDatabaseInit: true,
	},
	{
		title: "Check if OpenSSH authorized_keys file id correct",
		f:     runDoctorLocationMoved,
	},
	// more checks please append here
}

func runDoctor(ctx *cli.Context) error {

	// Silence the console logger
	// TODO: Redirect all logs into `doctor.log` ignoring any other log configuration
	log.DelNamedLogger("console")
	log.DelNamedLogger(log.DEFAULT)

	dbIsInit := false

	for i, check := range checklist {
		if !dbIsInit && !check.skipDatabaseInit {
			// Only open database after the most basic configuration check
			if err := initDB(); err != nil {
				fmt.Println(err)
				fmt.Println("Check if you are using the right config file. You can use a --config directive to specify one.")
				return nil
			}
			dbIsInit = true
		}
		fmt.Println("[", i+1, "]", check.title)
		messages, err := check.f(ctx)
		for _, message := range messages {
			fmt.Println("-", message)
		}
		if err != nil {
			fmt.Println("Error:", err)
			if check.abortIfFailed {
				return nil
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

func runDoctorPathInfo(ctx *cli.Context) ([]string, error) {

	res := make([]string, 0, 10)

	if fi, err := os.Stat(setting.CustomConf); err != nil || !fi.Mode().IsRegular() {
		res = append(res, fmt.Sprintf("Failed to find configuration file at '%s'.", setting.CustomConf))
		res = append(res, fmt.Sprintf("If you've never ran Gitea yet, this is normal and '%s' will be created for you on first run.", setting.CustomConf))
		res = append(res, "Otherwise check that you are running this command from the correct path and/or provide a `--config` parameter.")
		return res, fmt.Errorf("can't proceed without a configuration file")
	}

	setting.NewContext()

	fail := false

	check := func(name, path string, is_dir, required, is_write bool) {
		res = append(res, fmt.Sprintf("%-25s  '%s'", name+":", path))
		if fi, err := os.Stat(path); err != nil {
			if required {
				res = append(res, fmt.Sprintf("    ERROR: %v", err))
				fail = true
			} else {
				res = append(res, fmt.Sprintf("    NOTICE: not accessible (%v)", err))
			}
		} else if is_dir && !fi.IsDir() {
			res = append(res, "    ERROR: not a directory")
			fail = true
		} else if !is_dir && !fi.Mode().IsRegular() {
			res = append(res, "    ERROR: not a regular file")
			fail = true
		} else if is_write {
			if err := runDoctorWritableDir(path); err != nil {
				res = append(res, fmt.Sprintf("    ERROR: not writable: %v", err))
				fail = true
			}
		}
	}

	// Note print paths inside quotes to make any leading/trailing spaces evident
	check("Configuration File Path", setting.CustomConf, false, true, false)
	check("Repository Root Path", setting.RepoRootPath, true, true, true)
	check("Data Root Path", setting.AppDataPath, true, true, true)
	check("Custom File Root Path", setting.CustomPath, true, false, false)
	check("Work directory", setting.AppWorkPath, true, true, false)
	check("Log Root Path", setting.LogRootPath, true, true, true)

	if options.IsDynamic() {
		// Do not check/report on StaticRootPath if data is embedded in Gitea (-tags bindata)
		check("Static File Root Path", setting.StaticRootPath, true, true, false)
	}

	if fail {
		return res, fmt.Errorf("please check your configuration file and try again")
	}

	return res, nil
}

func runDoctorWritableDir(path string) error {
	// There's no platform-independent way of checking if a directory is writable
	// https://stackoverflow.com/questions/20026320/how-to-tell-if-folder-exists-and-is-writable

	tmpFile, err := ioutil.TempFile(path, "doctors-order")
	if err != nil {
		return err
	}
	if err := os.Remove(tmpFile.Name()); err != nil {
		fmt.Printf("Warning: can't remove temporary file: '%s'\n", tmpFile.Name())
	}
	tmpFile.Close()
	return nil
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
