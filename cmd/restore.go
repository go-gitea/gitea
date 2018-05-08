// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"

	"github.com/Unknwon/cae/zip"
	"github.com/Unknwon/com"
	"github.com/urfave/cli"
)

// CmdRestore represents the available restore sub-command.
var CmdRestore = cli.Command{
	Name:  "restore",
	Usage: "Restore Gitea files and database",
	Description: `Restore will restore all data from zip file which dumped from gitea. It will use 
the custom config in this dump zip file, this operation will remove all the dest database and repositories.`,
	Action: runRestore,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Value: "custom/conf/app.ini",
			Usage: "Custom configuration file path, if empty will use dumped config file",
		},
		cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "Show process details",
		},
		cli.StringFlag{
			Name:  "tempdir, t",
			Value: os.TempDir(),
			Usage: "Temporary dir path",
		},
	},
}

func runRestore(ctx *cli.Context) error {
	if len(os.Args) < 3 {
		return errors.New("need zip file path")
	}

	tmpDir := ctx.String("tempdir")
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		log.Fatalf("Path does not exist: %s", tmpDir)
	}
	tmpWorkDir, err := ioutil.TempDir(tmpDir, "gitea-restore-")
	if err != nil {
		log.Fatalf("Failed to create tmp work directory: %v", err)
	}
	log.Printf("Creating tmp work dir: %s", tmpWorkDir)

	// work-around #1103
	if os.Getenv("TMPDIR") == "" {
		os.Setenv("TMPDIR", tmpWorkDir)
	}

	srcPath := os.Args[2]

	zip.Verbose = ctx.Bool("verbose")
	log.Printf("Extracting %s to tmp work dir", srcPath)
	err = zip.ExtractTo(srcPath, tmpWorkDir)
	if err != nil {
		log.Fatalf("Failed to extract %s to tmp work directory: %v", srcPath, err)
	}

	verData, err := ioutil.ReadFile(filepath.Join(tmpWorkDir, "VERSION"))
	if err != nil {
		log.Fatalf("Failed to extract %s to tmp work directory: %v", srcPath, err)
	}

	if setting.AppVer != string(verData) {
		log.Fatalf("Expected gitea version to restore is %s, but get %s", string(verData), setting.AppVer)
	}

	if ctx.IsSet("config") {
		setting.CustomConf = ctx.String("config")
	} else {
		setting.CustomConf = filepath.Join(tmpWorkDir, "custom", "conf", "app.ini")
	}
	if !com.IsExist(setting.CustomConf) {
		log.Fatalf("Failed to load ini config file from %s", setting.CustomConf)
	}

	setting.NewContext()
	//setting.CustomPath = filepath.Join(tmpWorkDir, "custom")
	setting.NewXORMLogService(false)
	models.LoadConfigs()

	err = models.SetEngine()
	if err != nil {
		log.Fatalf("Failed to SetEngine: %v", err)
	}

	log.Printf("Restoring repo dir %s ...", setting.RepoRootPath)
	repoPath := filepath.Join(tmpWorkDir, "repositories")
	err = os.RemoveAll(setting.RepoRootPath)
	if err != nil {
		log.Fatalf("Failed to Remove repo root path %s: %v", setting.RepoRootPath, err)
	}

	err = os.Rename(repoPath, setting.RepoRootPath)
	if err != nil {
		log.Fatalf("Failed to move %s to %s: %v", repoPath, setting.RepoRootPath, err)
	}

	log.Printf("Restoring custom dir %s ...", setting.CustomPath)
	customPath := filepath.Join(tmpWorkDir, "custom")
	err = os.RemoveAll(setting.CustomPath)
	if err != nil {
		log.Fatalf("Failed to Remove repo root path %s: %v", setting.CustomPath, err)
	}

	err = os.Rename(customPath, setting.CustomPath)
	if err != nil {
		log.Fatalf("Failed to move %s to %s: %v", customPath, setting.CustomPath, err)
	}

	log.Printf("Restoring data dir %s ...", setting.AppDataPath)
	dataPath := filepath.Join(tmpWorkDir, "data")
	err = os.RemoveAll(setting.AppDataPath)
	if err != nil {
		log.Fatalf("Failed to Remove data root path %s: %v", setting.AppDataPath, err)
	}

	err = os.Rename(dataPath, setting.AppDataPath)
	if err != nil {
		log.Fatalf("Failed to move %s to %s: %v", dataPath, setting.AppDataPath, err)
	}

	log.Printf("Restoring database from ...")
	dbPath := filepath.Join(tmpWorkDir, "database")
	err = models.RestoreDatabaseFixtures(dbPath)
	if err != nil {
		log.Fatalf("Failed to restore database dir %s: %v", dbPath, err)
	}

	return nil
}
