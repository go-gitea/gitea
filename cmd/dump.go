// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/unknwon/cae/zip"
	"github.com/unknwon/com"
	"github.com/urfave/cli"
)

// CmdDump represents the available dump sub-command.
var CmdDump = cli.Command{
	Name:  "dump",
	Usage: "Dump Gitea files and database",
	Description: `Dump compresses all related files and database into zip file.
It can be used for backup and capture Gitea server image to send to maintainer`,
	Action: runDump,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "file, f",
			Value: fmt.Sprintf("gitea-dump-%d.zip", time.Now().Unix()),
			Usage: "Name of the dump file which will be created.",
		},
		cli.BoolFlag{
			Name:  "verbose, V",
			Usage: "Show process details",
		},
		cli.StringFlag{
			Name:  "tempdir, t",
			Value: os.TempDir(),
			Usage: "Temporary dir path",
		},
		cli.StringFlag{
			Name:  "database, d",
			Usage: "Specify the database SQL syntax",
		},
		cli.BoolFlag{
			Name:  "skip-repository, R",
			Usage: "Skip the repository dumping",
		},
	},
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	log.Fatal(format, args...)
}

func runDump(ctx *cli.Context) error {
	setting.NewContext()
	setting.NewServices() // cannot access session settings otherwise

	err := models.SetEngine()
	if err != nil {
		return err
	}

	tmpDir := ctx.String("tempdir")
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		fatal("Path does not exist: %s", tmpDir)
	}
	tmpWorkDir, err := ioutil.TempDir(tmpDir, "gitea-dump-")
	if err != nil {
		fatal("Failed to create tmp work directory: %v", err)
	}
	log.Info("Creating tmp work dir: %s", tmpWorkDir)

	// work-around #1103
	if os.Getenv("TMPDIR") == "" {
		os.Setenv("TMPDIR", tmpWorkDir)
	}

	dbDump := path.Join(tmpWorkDir, "gitea-db.sql")

	fileName := ctx.String("file")
	log.Info("Packing dump files...")
	z, err := zip.Create(fileName)
	if err != nil {
		fatal("Failed to create %s: %v", fileName, err)
	}

	zip.Verbose = ctx.Bool("verbose")

	if ctx.IsSet("skip-repository") {
		log.Info("Skip dumping local repositories")
	} else {
		log.Info("Dumping local repositories...%s", setting.RepoRootPath)
		reposDump := path.Join(tmpWorkDir, "gitea-repo.zip")
		if err := zip.PackTo(setting.RepoRootPath, reposDump, true); err != nil {
			fatal("Failed to dump local repositories: %v", err)
		}
		if err := z.AddFile("gitea-repo.zip", reposDump); err != nil {
			fatal("Failed to include gitea-repo.zip: %v", err)
		}
	}

	targetDBType := ctx.String("database")
	if len(targetDBType) > 0 && targetDBType != setting.Database.Type {
		log.Info("Dumping database %s => %s...", setting.Database.Type, targetDBType)
	} else {
		log.Info("Dumping database...")
	}

	if err := models.DumpDatabase(dbDump, targetDBType); err != nil {
		fatal("Failed to dump database: %v", err)
	}

	if err := z.AddFile("gitea-db.sql", dbDump); err != nil {
		fatal("Failed to include gitea-db.sql: %v", err)
	}

	if len(setting.CustomConf) > 0 {
		log.Info("Adding custom configuration file from %s", setting.CustomConf)
		if err := z.AddFile("app.ini", setting.CustomConf); err != nil {
			fatal("Failed to include specified app.ini: %v", err)
		}
	}

	customDir, err := os.Stat(setting.CustomPath)
	if err == nil && customDir.IsDir() {
		if err := z.AddDir("custom", setting.CustomPath); err != nil {
			fatal("Failed to include custom: %v", err)
		}
	} else {
		log.Info("Custom dir %s doesn't exist, skipped", setting.CustomPath)
	}

	if com.IsExist(setting.AppDataPath) {
		log.Info("Packing data directory...%s", setting.AppDataPath)

		var sessionAbsPath string
		if setting.SessionConfig.Provider == "file" {
			sessionAbsPath = setting.SessionConfig.ProviderConfig
		}
		if err := zipAddDirectoryExclude(z, "data", setting.AppDataPath, sessionAbsPath); err != nil {
			fatal("Failed to include data directory: %v", err)
		}
	}

	if com.IsExist(setting.LogRootPath) {
		if err := z.AddDir("log", setting.LogRootPath); err != nil {
			fatal("Failed to include log: %v", err)
		}
	}

	if err = z.Close(); err != nil {
		_ = os.Remove(fileName)
		fatal("Failed to save %s: %v", fileName, err)
	}

	if err := os.Chmod(fileName, 0600); err != nil {
		log.Info("Can't change file access permissions mask to 0600: %v", err)
	}

	log.Info("Removing tmp work dir: %s", tmpWorkDir)

	if err := os.RemoveAll(tmpWorkDir); err != nil {
		fatal("Failed to remove %s: %v", tmpWorkDir, err)
	}
	log.Info("Finish dumping in file %s", fileName)

	return nil
}

// zipAddDirectoryExclude zips absPath to specified zipPath inside z excluding excludeAbsPath
func zipAddDirectoryExclude(zip *zip.ZipArchive, zipPath, absPath string, excludeAbsPath string) error {
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return err
	}
	dir, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer dir.Close()

	zip.AddEmptyDir(zipPath)

	files, err := dir.Readdir(0)
	if err != nil {
		return err
	}
	for _, file := range files {
		currentAbsPath := path.Join(absPath, file.Name())
		currentZipPath := path.Join(zipPath, file.Name())
		if file.IsDir() {
			if currentAbsPath != excludeAbsPath {
				if err = zipAddDirectoryExclude(zip, currentZipPath, currentAbsPath, excludeAbsPath); err != nil {
					return err
				}
			}

		} else {
			if err = zip.AddFile(currentZipPath, currentAbsPath); err != nil {
				return err
			}
		}
	}
	return nil
}
