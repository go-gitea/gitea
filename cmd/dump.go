// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"gitea.com/macaron/session"

	archiver "github.com/mholt/archiver/v3"
	"github.com/unknwon/com"
	"github.com/urfave/cli"
)

func addFile(w archiver.Writer, filePath string, absPath string, verbose bool) error {
	if verbose {
		fmt.Fprintf(os.Stderr, "Adding file %s\n", filePath)
	}
	file, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	return w.Write(archiver.File{
		FileInfo: archiver.FileInfo{
			FileInfo:   fileInfo,
			CustomName: filePath,
		},
		ReadCloser: file,
	})
}

func addRecursive(w archiver.Writer, dirPath string, absPath string, verbose bool) error {
	if verbose {
		fmt.Fprintf(os.Stderr, "Adding dir  %s\n", dirPath)
	}
	dir, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("Could not open directory %s: %s", absPath, err)
	}
	files, err := dir.Readdir(0)
	if err != nil {
		return fmt.Errorf("Unable to list files in %s: %s", absPath, err)
	}

	if err := addFile(w, dirPath, absPath, false); err != nil {
		return err
	}

	for _, fileInfo := range files {
		if fileInfo.IsDir() {
			err = addRecursive(w, filepath.Join(dirPath, fileInfo.Name()), filepath.Join(absPath, fileInfo.Name()), verbose)
		} else {
			err = addFile(w, filepath.Join(dirPath, fileInfo.Name()), filepath.Join(absPath, fileInfo.Name()), verbose)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type outputType struct {
	Enum     []string
	Default  string
	selected string
}

func (o outputType) Join() string {
	return strings.Join(o.Enum, ", ")
}

func (o *outputType) Set(value string) error {
	for _, enum := range o.Enum {
		if enum == value {
			o.selected = value
			return nil
		}
	}

	return fmt.Errorf("allowed values are %s", o.Join())
}

func (o outputType) String() string {
	if o.selected == "" {
		return o.Default
	}
	return o.selected
}

var outputTypeEnum = &outputType{
	Enum:    []string{"zip", "tar", "tar.gz", "tar.xz", "tar.bz2"},
	Default: "zip",
}

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
			Usage: "Name of the dump file which will be created. See type for available types.",
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
		cli.BoolFlag{
			Name:  "skip-log, L",
			Usage: "Skip the log dumping",
		},
		cli.GenericFlag{
			Name:  "type",
			Value: outputTypeEnum,
			Usage: fmt.Sprintf("Dump output format: %s", outputTypeEnum.Join()),
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

	fileName := ctx.String("file")
	file, err := os.Create(fileName)
	if err != nil {
		fatal("Unable to open %s: %s", fileName, err)
	}
	defer file.Close()

	verbose := ctx.Bool("verbose")
	outType := ctx.String("type")
	var iface interface{}
	if fileName == "-" {
		iface, err = archiver.ByExtension(fmt.Sprintf(".%s", outType))
	} else {
		iface, err = archiver.ByExtension(fileName)
	}
	if err != nil {
		fatal("Unable to get archiver for extension: %v", err)
	}

	w, _ := iface.(archiver.Writer)
	if err := w.Create(file); err != nil {
		fatal("Creating archiver.Writer failed: %v", err)
	}
	defer w.Close()

	tmpDir := ctx.String("tempdir")
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		fatal("Path does not exist: %s", tmpDir)
	}
	tmpWorkDir, err := ioutil.TempDir(tmpDir, "gitea-dump-")
	if err != nil {
		fatal("Failed to create tmp work directory: %v", err)
	}
	defer os.RemoveAll(tmpWorkDir)
	log.Info("Creating tmp work dir: %s", tmpWorkDir)

	dbDump := path.Join(tmpWorkDir, "gitea-db.sql")

	if ctx.IsSet("skip-repository") && ctx.Bool("skip-repository") {
		log.Info("Skip dumping local repositories")
	} else {
		log.Info("Dumping local repositories...%s", setting.RepoRootPath)
		if err := addRecursive(w, "repos", setting.RepoRootPath, verbose); err != nil {
			fatal("Failed to include repositories: %v", err)
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

	if err := addFile(w, "gitea-db.sql", dbDump, verbose); err != nil {
		fatal("Failed to include gitea-db.sql: %v", err)
	}

	if len(setting.CustomConf) > 0 {
		log.Info("Adding custom configuration file from %s", setting.CustomConf)
		if err := addFile(w, "app.ini", setting.CustomConf, verbose); err != nil {
			fatal("Failed to include specified app.ini: %v", err)
		}
	}

	customDir, err := os.Stat(setting.CustomPath)
	if err == nil && customDir.IsDir() {
		if err := addRecursive(w, "custom", setting.CustomPath, verbose); err != nil {
			fatal("Failed to include custom: %v", err)
		}
	} else {
		log.Info("Custom dir %s doesn't exist, skipped", setting.CustomPath)
	}

	if com.IsExist(setting.AppDataPath) {
		log.Info("Packing data directory...%s", setting.AppDataPath)

		var excludes []string
		if setting.Cfg.Section("session").Key("PROVIDER").Value() == "file" {
			var opts session.Options
			if err = json.Unmarshal([]byte(setting.SessionConfig.ProviderConfig), &opts); err != nil {
				return err
			}
			excludes = append(excludes, opts.ProviderConfig)
		}

		excludes = append(excludes, setting.RepoRootPath)
		excludes = append(excludes, setting.LFS.ContentPath)
		excludes = append(excludes, setting.LogRootPath)
		if err := addRecursiveExclude(w, "data", setting.AppDataPath, excludes, verbose); err != nil {
			fatal("Failed to include data directory: %v", err)
		}
	}

	// Doesn't check if LogRootPath exists before processing --skip-log intentionally,
	// ensuring that it's clear the dump is skipped whether the directory's initialized
	// yet or not.
	if ctx.IsSet("skip-log") && ctx.Bool("skip-log") {
		log.Info("Skip dumping log files")
	} else if com.IsExist(setting.LogRootPath) {
		if err := addRecursive(w, "log", setting.LogRootPath, verbose); err != nil {
			fatal("Failed to include log: %v", err)
		}
	}

	if err = w.Close(); err != nil {
		_ = os.Remove(fileName)
		fatal("Failed to save %s: %v", fileName, err)
	}

	if err := os.Chmod(fileName, 0600); err != nil {
		log.Info("Can't change file access permissions mask to 0600: %v", err)
	}

	log.Info("Finish dumping in file %s", fileName)

	return nil
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// addRecursiveExclude zips absPath to specified insidePath inside writer excluding excludeAbsPath
func addRecursiveExclude(w archiver.Writer, insidePath, absPath string, excludeAbsPath []string, verbose bool) error {
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return err
	}
	dir, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer dir.Close()

	files, err := dir.Readdir(0)
	if err != nil {
		return err
	}
	for _, file := range files {
		currentAbsPath := path.Join(absPath, file.Name())
		currentInsidePath := path.Join(insidePath, file.Name())
		if file.IsDir() {
			if !contains(excludeAbsPath, currentAbsPath) {
				if err := addFile(w, currentInsidePath, currentAbsPath, false); err != nil {
					return err
				}
				if err = addRecursiveExclude(w, currentInsidePath, currentAbsPath, excludeAbsPath, verbose); err != nil {
					return err
				}
			}
		} else {
			if err = addFile(w, currentInsidePath, currentAbsPath, verbose); err != nil {
				return err
			}
		}
	}
	return nil
}
