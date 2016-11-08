// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"io/ioutil"

	"github.com/Unknwon/cae/zip"
	"github.com/urfave/cli"

	"github.com/go-gitea/gitea/models"
	"github.com/go-gitea/gitea/modules/setting"
)

// CmdDump represents the available dump sub-command.
var CmdDump = cli.Command{
	Name:  "dump",
	Usage: "Dump Gogs files and database",
	Description: `Dump compresses all related files and database into zip file.
It can be used for backup and capture Gogs server image to send to maintainer`,
	Action: runDump,
	Flags: []cli.Flag{
		stringFlag("config, c", "custom/conf/app.ini", "Custom configuration file path"),
		boolFlag("verbose, v", "Show process details"),
		stringFlag("tempdir, t", os.TempDir(), "Temporary dir path"),
	},
}

func runDump(ctx *cli.Context) error {
	if ctx.IsSet("config") {
		setting.CustomConf = ctx.String("config")
	}
	setting.NewContext()
	models.LoadConfigs()
	models.SetEngine()

	tmpDir := ctx.String("tempdir")
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		log.Fatalf("Path does not exist: %s", tmpDir)
	}
	TmpWorkDir, err := ioutil.TempDir(tmpDir, "gogs-dump-")
	if err != nil {
		log.Fatalf("Fail to create tmp work directory: %v", err)
	}
	log.Printf("Creating tmp work dir: %s", TmpWorkDir)

	reposDump := path.Join(TmpWorkDir, "gogs-repo.zip")
	dbDump := path.Join(TmpWorkDir, "gogs-db.sql")

	log.Printf("Dumping local repositories...%s", setting.RepoRootPath)
	zip.Verbose = ctx.Bool("verbose")
	if err := zip.PackTo(setting.RepoRootPath, reposDump, true); err != nil {
		log.Fatalf("Fail to dump local repositories: %v", err)
	}

	log.Printf("Dumping database...")
	if err := models.DumpDatabase(dbDump); err != nil {
		log.Fatalf("Fail to dump database: %v", err)
	}

	fileName := fmt.Sprintf("gogs-dump-%d.zip", time.Now().Unix())
	log.Printf("Packing dump files...")
	z, err := zip.Create(fileName)
	if err != nil {
		os.Remove(fileName)
		log.Fatalf("Fail to create %s: %v", fileName, err)
	}

	if err := z.AddFile("gogs-repo.zip", reposDump); err != nil {
		log.Fatalf("Fail to include gogs-repo.zip: %v", err)
	}
	if err := z.AddFile("gogs-db.sql", dbDump); err != nil {
		log.Fatalf("Fail to include gogs-db.sql: %v", err)
	}
	customDir, err := os.Stat(setting.CustomPath)
	if err == nil && customDir.IsDir() {
		if err := z.AddDir("custom", setting.CustomPath); err != nil {
			log.Fatalf("Fail to include custom: %v", err)
		}
	} else {
		log.Printf("Custom dir %s doesn't exist, skipped", setting.CustomPath)
	}
	if err := z.AddDir("log", setting.LogRootPath); err != nil {
		log.Fatalf("Fail to include log: %v", err)
	}
	// FIXME: SSH key file.
	if err = z.Close(); err != nil {
		os.Remove(fileName)
		log.Fatalf("Fail to save %s: %v", fileName, err)
	}

	if err := os.Chmod(fileName, 0600); err != nil {
		log.Printf("Can't change file access permissions mask to 0600: %v", err)
	}

	log.Printf("Removing tmp work dir: %s", TmpWorkDir)
	os.RemoveAll(TmpWorkDir)
	log.Printf("Finish dumping in file %s", fileName)

	return nil
}
