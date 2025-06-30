// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

type configurationFile struct {
	Name        string
	Path        string
	IsDirectory bool
	Required    bool
	Writable    bool
}

func checkConfigurationFile(logger log.Logger, autofix bool, fileOpts configurationFile) error {
	logger.Info(`%-26s  %q`, log.NewColoredValue(fileOpts.Name+":", log.Reset), fileOpts.Path)
	fi, err := os.Stat(fileOpts.Path)
	if err != nil {
		if os.IsNotExist(err) && autofix && fileOpts.IsDirectory {
			if err := os.MkdirAll(fileOpts.Path, 0o777); err != nil {
				logger.Error("    Directory does not exist and could not be created. ERROR: %v", err)
				return fmt.Errorf("Configuration directory: \"%q\" does not exist and could not be created. ERROR: %w", fileOpts.Path, err)
			}
			fi, err = os.Stat(fileOpts.Path)
		}
	}
	if err != nil {
		if fileOpts.Required {
			logger.Error("    Is REQUIRED but is not accessible. ERROR: %v", err)
			return fmt.Errorf("Configuration file \"%q\" is not accessible but is required. Error: %w", fileOpts.Path, err)
		}
		logger.Warn("    NOTICE: is not accessible (Error: %v)", err)
		// this is a non-critical error
		return nil
	}

	if fileOpts.IsDirectory && !fi.IsDir() {
		logger.Error("    ERROR: not a directory")
		return fmt.Errorf("Configuration directory \"%q\" is not a directory. Error: %w", fileOpts.Path, err)
	} else if !fileOpts.IsDirectory && !fi.Mode().IsRegular() {
		logger.Error("    ERROR: not a regular file")
		return fmt.Errorf("Configuration file \"%q\" is not a regular file. Error: %w", fileOpts.Path, err)
	} else if fileOpts.Writable {
		if err := isWritableDir(fileOpts.Path); err != nil {
			logger.Error("    ERROR: is required to be writable but is not writable: %v", err)
			return fmt.Errorf("Configuration file \"%q\" is required to be writable but is not. Error: %w", fileOpts.Path, err)
		}
	}
	return nil
}

func checkConfigurationFiles(ctx context.Context, logger log.Logger, autofix bool) error {
	if fi, err := os.Stat(setting.CustomConf); err != nil || !fi.Mode().IsRegular() {
		logger.Error("Failed to find configuration file at '%s'.", setting.CustomConf)
		logger.Error("If you've never ran Gitea yet, this is normal and '%s' will be created for you on first run.", setting.CustomConf)
		logger.Error("Otherwise check that you are running this command from the correct path and/or provide a `--config` parameter.")
		logger.Critical("Cannot proceed without a configuration file")
		return err
	}

	setting.MustInstalled()

	configurationFiles := []configurationFile{
		{"Configuration File Path", setting.CustomConf, false, true, false},
		{"Repository Root Path", setting.RepoRootPath, true, true, true},
		{"Data Root Path", setting.AppDataPath, true, true, true},
		{"Custom File Root Path", setting.CustomPath, true, false, false},
		{"Work directory", setting.AppWorkPath, true, true, false},
		{"Log Root Path", setting.Log.RootPath, true, true, true},
	}

	if !setting.HasBuiltinBindata {
		configurationFiles = append(configurationFiles, configurationFile{"Static File Root Path", setting.StaticRootPath, true, true, false})
	}

	numberOfErrors := 0
	for _, configurationFile := range configurationFiles {
		if err := checkConfigurationFile(logger, autofix, configurationFile); err != nil {
			numberOfErrors++
		}
	}

	if numberOfErrors > 0 {
		logger.Critical("Please check your configuration files and try again.")
		return fmt.Errorf("%d configuration files with errors", numberOfErrors)
	}

	return nil
}

func isWritableDir(path string) error {
	// There's no platform-independent way of checking if a directory is writable
	// https://stackoverflow.com/questions/20026320/how-to-tell-if-folder-exists-and-is-writable
	tmpFile, err := os.CreateTemp(path, "doctors-order")
	if err != nil {
		return err
	}
	if err := os.Remove(tmpFile.Name()); err != nil {
		log.Warn("can't remove temporary file: %q", tmpFile.Name())
	}
	_ = tmpFile.Close()
	return nil
}

func init() {
	Register(&Check{
		Title:                      "Check paths and basic configuration",
		Name:                       "paths",
		IsDefault:                  true,
		Run:                        checkConfigurationFiles,
		AbortIfFailed:              true,
		SkipDatabaseInitialization: true,
		Priority:                   1,
	})
}
