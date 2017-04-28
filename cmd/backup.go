// Copyright 2017 the Gitea Authors. All rights reserved.
// Copyright 2017 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime/debug"
	"time"

	"github.com/Unknwon/cae/zip"
	"github.com/Unknwon/com"
	"github.com/urfave/cli"
	"gopkg.in/ini.v1"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// Backup files and database
var Backup = cli.Command{
	Name:  "backup",
	Usage: "Backup files and database",
	Description: `Backup dumps and compresses all related files and database into zip file,
   which can be used for migrating Gitea to another server. The output format is meant to be
   portable among all supported database engines.`,
	Action: runBackup,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Value: "custom/conf/app.ini",
			Usage: "Custom configuration `FILE` path",
		},
		cli.StringFlag{
			Name:  "tempdir, t",
			Value: os.TempDir(),
			Usage: "Temporary directory `PATH`",
		},
		cli.StringFlag{
			Name:  "target",
			Value: "./",
			Usage: "Target directory `PATH` to save backup archive",
		},
		cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "Show process details",
		},
		cli.BoolTFlag{
			Name:  "db",
			Usage: "Backup the database (default: true)",
		},
		cli.BoolTFlag{
			Name:  "repos",
			Usage: "Backup repositories (default: true)",
		},
		cli.BoolTFlag{
			Name:  "data",
			Usage: "Backup attachments and avatars (default: true)",
		},
		cli.BoolTFlag{
			Name:  "custom",
			Usage: "Backup custom files (default: true)",
		},
	},
}

const (
	archiveRootDir = "gitea-backup"
	backupVersion  = 1
)

func runBackup(c *cli.Context) error {
	zip.Verbose = c.Bool("verbose")
	if c.IsSet("config") {
		setting.CustomConf = c.String("config")
	}
	setting.NewContext()
	models.LoadConfigs()
	if err := models.SetEngine(); err != nil {
		return err
	}

	// Setup temp-dir
	tmpDir := c.String("tempdir")
	if !com.IsExist(tmpDir) {
		log.Fatal(0, "'--tempdir' does not exist: %s", tmpDir)
	}
	rootDir, err := ioutil.TempDir(tmpDir, "gitea-backup-")
	if err != nil {
		log.Fatal(0, "Fail to create backup root directory '%s': %v", rootDir, err)
	}
	defer func(rootDir string) {
		os.RemoveAll(rootDir)
	}(rootDir)
	log.Info("Backup root directory: %s", rootDir)

	// Metadata
	metaFile := path.Join(rootDir, "metadata.ini")
	metadata := ini.Empty()
	metadata.Section("").Key("VERSION").SetValue(fmt.Sprintf("%d", backupVersion))
	metadata.Section("").Key("DATE_TIME").SetValue(time.Now().String())
	metadata.Section("").Key("GITEA_VERSION").SetValue(setting.AppVer)
	if err = metadata.SaveTo(metaFile); err != nil {
		log.Fatal(0, "Fail to save metadata '%s': %v", metaFile, err)
	}

	// Create ZIP-file
	archiveName := path.Join(c.String("target"), fmt.Sprintf("gitea-backup-%d.zip", time.Now().Unix()))
	log.Info("Packing backup files to: %s", archiveName)

	z, err := zip.Create(archiveName)
	if err != nil {
		log.Fatal(0, "Fail to create backup archive '%s': %v", archiveName, err)
	}
	defer func(archiveName string) {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("pkg: %v", r)
			}
			debug.PrintStack()
			log.Info("Removing partial backup-file %s\n", archiveName)
			os.Remove(archiveName)
			log.Fatal(9, "%v\n", err)
		}
	}(archiveName)

	// Add metadata-file
	if err = z.AddFile(archiveRootDir+"/metadata.ini", metaFile); err != nil {
		log.Fatal(0, "Fail to include 'metadata.ini': %v", err)
	}

	// Database
	if c.Bool("db") {
		log.Info("Backing up database")
		dbDir := path.Join(rootDir, "db")
		if err = models.DumpDatabase(dbDir); err != nil {
			log.Fatal(0, "Fail to dump database: %v", err)
		}
		if err = z.AddDir(archiveRootDir+"/db", dbDir); err != nil {
			log.Fatal(0, "Fail to include 'db': %v", err)
		}
	}

	// Custom files
	if c.Bool("custom") {
		log.Info("Backing up custom files")
		if err = z.AddDir(archiveRootDir+"/custom", setting.CustomPath); err != nil {
			log.Fatal(0, "Fail to include 'custom': %v", err)
		}
	}

	// Data files
	if c.Bool("data") {
		log.Info("Backing up attachments and avatars")
		for _, dir := range []string{"attachments", "avatars"} {
			dirPath := path.Join(setting.AppDataPath, dir)
			if !com.IsDir(dirPath) {
				continue
			}

			if err = z.AddDir(path.Join(archiveRootDir+"/data", dir), dirPath); err != nil {
				log.Fatal(0, "Fail to include 'data': %v", err)
			}
		}
	}

	// Repositories
	if c.Bool("repos") {
		log.Info("Backing up repositories")
		reposDump := path.Join(rootDir, "repositories.zip")
		log.Info("Dumping repositories in '%s'", setting.RepoRootPath)
		if err = zip.PackTo(setting.RepoRootPath, reposDump, true); err != nil {
			log.Fatal(0, "Fail to dump repositories: %v", err)
		}
		log.Info("Repositories dumped to: %s", reposDump)

		if err = z.AddFile(archiveRootDir+"/repositories.zip", reposDump); err != nil {
			log.Fatal(0, "Fail to include 'repositories.zip': %v", err)
		}
	}

	if err = z.Close(); err != nil {
		log.Fatal(0, "Fail to save backup archive '%s': %v", archiveName, err)
	}

	os.RemoveAll(rootDir)
	log.Info("Backup succeed! Archive is located at: %s", archiveName)
	return nil
}
