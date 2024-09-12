// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/dump"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"

	"gitea.com/go-chi/session"
	"github.com/mholt/archiver/v3"
	"github.com/urfave/cli/v2"
)

// CmdDump represents the available dump sub-command.
var CmdDump = &cli.Command{
	Name:        "dump",
	Usage:       "Dump Gitea files and database",
	Description: `Dump compresses all related files and database into zip file. It can be used for backup and capture Gitea server image to send to maintainer`,
	Action:      runDump,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "file",
			Aliases: []string{"f"},
			Usage:   `Name of the dump file which will be created, default to "gitea-dump-{time}.zip". Supply '-' for stdout. See type for available types.`,
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"V"},
			Usage:   "Show process details",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Only display warnings and errors",
		},
		&cli.StringFlag{
			Name:    "tempdir",
			Aliases: []string{"t"},
			Value:   os.TempDir(),
			Usage:   "Temporary dir path",
		},
		&cli.StringFlag{
			Name:    "database",
			Aliases: []string{"d"},
			Usage:   "Specify the database SQL syntax: sqlite3, mysql, mssql, postgres",
		},
		&cli.BoolFlag{
			Name:    "skip-repository",
			Aliases: []string{"R"},
			Usage:   "Skip the repository dumping",
		},
		&cli.BoolFlag{
			Name:    "skip-log",
			Aliases: []string{"L"},
			Usage:   "Skip the log dumping",
		},
		&cli.BoolFlag{
			Name:  "skip-custom-dir",
			Usage: "Skip custom directory",
		},
		&cli.BoolFlag{
			Name:  "skip-lfs-data",
			Usage: "Skip LFS data",
		},
		&cli.BoolFlag{
			Name:  "skip-attachment-data",
			Usage: "Skip attachment data",
		},
		&cli.BoolFlag{
			Name:  "skip-package-data",
			Usage: "Skip package data",
		},
		&cli.BoolFlag{
			Name:  "skip-index",
			Usage: "Skip bleve index data",
		},
		&cli.BoolFlag{
			Name:  "skip-db",
			Usage: "Skip database",
		},
		&cli.StringFlag{
			Name:  "type",
			Usage: fmt.Sprintf(`Dump output format, default to "zip", supported types: %s`, strings.Join(dump.SupportedOutputTypes, ", ")),
		},
	},
}

func fatal(format string, args ...any) {
	log.Fatal(format, args...)
}

func runDump(ctx *cli.Context) error {
	setting.MustInstalled()

	quite := ctx.Bool("quiet")
	verbose := ctx.Bool("verbose")
	if verbose && quite {
		fatal("Option --quiet and --verbose cannot both be set")
	}

	// outFileName is either "-" or a file name (will be made absolute)
	outFileName, outType := dump.PrepareFileNameAndType(ctx.String("file"), ctx.String("type"))
	if outType == "" {
		fatal("Invalid output type")
	}

	outFile := os.Stdout
	if outFileName != "-" {
		var err error
		if outFileName, err = filepath.Abs(outFileName); err != nil {
			fatal("Unable to get absolute path of dump file: %v", err)
		}
		if exist, _ := util.IsExist(outFileName); exist {
			fatal("Dump file %q exists", outFileName)
		}
		if outFile, err = os.Create(outFileName); err != nil {
			fatal("Unable to create dump file %q: %v", outFileName, err)
		}
		defer outFile.Close()
	}

	setupConsoleLogger(util.Iif(quite, log.WARN, log.INFO), log.CanColorStderr, os.Stderr)

	setting.DisableLoggerInit()
	setting.LoadSettings() // cannot access session settings otherwise

	stdCtx, cancel := installSignals()
	defer cancel()

	err := db.InitEngine(stdCtx)
	if err != nil {
		return err
	}

	if err = storage.Init(); err != nil {
		return err
	}

	archiverGeneric, err := archiver.ByExtension("." + outType)
	if err != nil {
		fatal("Unable to get archiver for extension: %v", err)
	}

	archiverWriter := archiverGeneric.(archiver.Writer)
	if err := archiverWriter.Create(outFile); err != nil {
		fatal("Creating archiver.Writer failed: %v", err)
	}
	defer archiverWriter.Close()

	dumper := &dump.Dumper{
		Writer:  archiverWriter,
		Verbose: verbose,
	}
	dumper.GlobalExcludeAbsPath(outFileName)

	if ctx.IsSet("skip-repository") && ctx.Bool("skip-repository") {
		log.Info("Skip dumping local repositories")
	} else {
		log.Info("Dumping local repositories... %s", setting.RepoRootPath)
		if err := dumper.AddRecursiveExclude("repos", setting.RepoRootPath, nil); err != nil {
			fatal("Failed to include repositories: %v", err)
		}

		if ctx.IsSet("skip-lfs-data") && ctx.Bool("skip-lfs-data") {
			log.Info("Skip dumping LFS data")
		} else if !setting.LFS.StartServer {
			log.Info("LFS isn't enabled. Skip dumping LFS data")
		} else if err := storage.LFS.IterateObjects("", func(objPath string, object storage.Object) error {
			info, err := object.Stat()
			if err != nil {
				return err
			}
			return dumper.AddReader(object, info, path.Join("data", "lfs", objPath))
		}); err != nil {
			fatal("Failed to dump LFS objects: %v", err)
		}
	}

	if ctx.Bool("skip-db") {
		// Ensure that we don't dump the database file that may reside in setting.AppDataPath or elsewhere.
		dumper.GlobalExcludeAbsPath(setting.Database.Path)
		log.Info("Skipping database")
	} else {
		tmpDir := ctx.String("tempdir")
		if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
			fatal("Path does not exist: %s", tmpDir)
		}

		dbDump, err := os.CreateTemp(tmpDir, "gitea-db.sql")
		if err != nil {
			fatal("Failed to create tmp file: %v", err)
		}
		defer func() {
			_ = dbDump.Close()
			if err := util.Remove(dbDump.Name()); err != nil {
				log.Warn("Unable to remove temporary file: %s: Error: %v", dbDump.Name(), err)
			}
		}()

		targetDBType := ctx.String("database")
		if len(targetDBType) > 0 && targetDBType != setting.Database.Type.String() {
			log.Info("Dumping database %s => %s...", setting.Database.Type, targetDBType)
		} else {
			log.Info("Dumping database...")
		}

		if err := db.DumpDatabase(dbDump.Name(), targetDBType); err != nil {
			fatal("Failed to dump database: %v", err)
		}

		if err = dumper.AddFile("gitea-db.sql", dbDump.Name()); err != nil {
			fatal("Failed to include gitea-db.sql: %v", err)
		}
	}

	log.Info("Adding custom configuration file from %s", setting.CustomConf)
	if err = dumper.AddFile("app.ini", setting.CustomConf); err != nil {
		fatal("Failed to include specified app.ini: %v", err)
	}

	if ctx.IsSet("skip-custom-dir") && ctx.Bool("skip-custom-dir") {
		log.Info("Skipping custom directory")
	} else {
		customDir, err := os.Stat(setting.CustomPath)
		if err == nil && customDir.IsDir() {
			if is, _ := dump.IsSubdir(setting.AppDataPath, setting.CustomPath); !is {
				if err := dumper.AddRecursiveExclude("custom", setting.CustomPath, nil); err != nil {
					fatal("Failed to include custom: %v", err)
				}
			} else {
				log.Info("Custom dir %s is inside data dir %s, skipped", setting.CustomPath, setting.AppDataPath)
			}
		} else {
			log.Info("Custom dir %s doesn't exist, skipped", setting.CustomPath)
		}
	}

	isExist, err := util.IsExist(setting.AppDataPath)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", setting.AppDataPath, err)
	}
	if isExist {
		log.Info("Packing data directory...%s", setting.AppDataPath)

		var excludes []string
		if setting.SessionConfig.OriginalProvider == "file" {
			var opts session.Options
			if err = json.Unmarshal([]byte(setting.SessionConfig.ProviderConfig), &opts); err != nil {
				return err
			}
			excludes = append(excludes, opts.ProviderConfig)
		}

		if ctx.IsSet("skip-index") && ctx.Bool("skip-index") {
			excludes = append(excludes, setting.Indexer.RepoPath)
			excludes = append(excludes, setting.Indexer.IssuePath)
		}

		excludes = append(excludes, setting.RepoRootPath)
		excludes = append(excludes, setting.LFS.Storage.Path)
		excludes = append(excludes, setting.Attachment.Storage.Path)
		excludes = append(excludes, setting.Packages.Storage.Path)
		excludes = append(excludes, setting.Log.RootPath)
		if err := dumper.AddRecursiveExclude("data", setting.AppDataPath, excludes); err != nil {
			fatal("Failed to include data directory: %v", err)
		}
	}

	if ctx.IsSet("skip-attachment-data") && ctx.Bool("skip-attachment-data") {
		log.Info("Skip dumping attachment data")
	} else if err := storage.Attachments.IterateObjects("", func(objPath string, object storage.Object) error {
		info, err := object.Stat()
		if err != nil {
			return err
		}
		return dumper.AddReader(object, info, path.Join("data", "attachments", objPath))
	}); err != nil {
		fatal("Failed to dump attachments: %v", err)
	}

	if ctx.IsSet("skip-package-data") && ctx.Bool("skip-package-data") {
		log.Info("Skip dumping package data")
	} else if !setting.Packages.Enabled {
		log.Info("Packages isn't enabled. Skip dumping package data")
	} else if err := storage.Packages.IterateObjects("", func(objPath string, object storage.Object) error {
		info, err := object.Stat()
		if err != nil {
			return err
		}
		return dumper.AddReader(object, info, path.Join("data", "packages", objPath))
	}); err != nil {
		fatal("Failed to dump packages: %v", err)
	}

	// Doesn't check if LogRootPath exists before processing --skip-log intentionally,
	// ensuring that it's clear the dump is skipped whether the directory's initialized
	// yet or not.
	if ctx.IsSet("skip-log") && ctx.Bool("skip-log") {
		log.Info("Skip dumping log files")
	} else {
		isExist, err := util.IsExist(setting.Log.RootPath)
		if err != nil {
			log.Error("Unable to check if %s exists. Error: %v", setting.Log.RootPath, err)
		}
		if isExist {
			if err := dumper.AddRecursiveExclude("log", setting.Log.RootPath, nil); err != nil {
				fatal("Failed to include log: %v", err)
			}
		}
	}

	if outFileName == "-" {
		log.Info("Finish dumping to stdout")
	} else {
		if err = archiverWriter.Close(); err != nil {
			_ = os.Remove(outFileName)
			fatal("Failed to save %q: %v", outFileName, err)
		}
		if err = os.Chmod(outFileName, 0o600); err != nil {
			log.Info("Can't change file access permissions mask to 0600: %v", err)
		}
		log.Info("Finish dumping in file %s", outFileName)
	}
	return nil
}
