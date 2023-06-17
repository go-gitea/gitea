// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

var (
	// AppPath represents the path to the gitea binary
	AppPath string

	// AppWorkPath is the "working directory" of Gitea. It maps to the environment variable GITEA_WORK_DIR.
	// If that is not set it is the default set here by the linker or failing that the directory of AppPath.
	// It is used as the base path for several other paths.
	AppWorkPath string

	CustomPath string // Custom directory path. Env: GITEA_CUSTOM
	CustomConf string
)

func getAppPath() (string, error) {
	var appPath string
	var err error
	if IsWindows && filepath.IsAbs(os.Args[0]) {
		appPath = filepath.Clean(os.Args[0])
	} else {
		appPath, err = exec.LookPath(os.Args[0])
	}
	if err != nil {
		if !errors.Is(err, exec.ErrDot) {
			return "", err
		}
		appPath, err = filepath.Abs(os.Args[0])
	}
	if err != nil {
		return "", err
	}
	appPath, err = filepath.Abs(appPath)
	if err != nil {
		return "", err
	}
	// Note: (legacy code) we don't use path.Dir here because it does not handle case which path starts with two "/" in Windows: "//psf/Home/..."
	return strings.ReplaceAll(appPath, "\\", "/"), err
}

func init() {
	var err error
	if AppPath, err = getAppPath(); err != nil {
		log.Fatal("Failed to get app path: %v", err)
	}
	AppWorkPath = filepath.Dir(AppPath)
	CustomPath = filepath.Join(AppWorkPath, "custom")
}

type ArgWorkPathAndCustomConf struct {
	WorkPath   string
	CustomPath string
	CustomConf string
}

// InitWorkPathAndCommonConfig will set AppWorkPath, CustomPath and CustomConf, init default config provider by CustomConf and load common settings,
func InitWorkPathAndCommonConfig(getEnvFn func(name string) string, args ArgWorkPathAndCustomConf) {
	var err error
	var tmpWorkPath, tmpCustomPath, tmpCustomConf string

	readFromEnv := func() {
		// for consistency, prefer to use GITEA_WORK_PATH and GITEA_CUSTOM_PATH
		// the widely used GITEA_WORK_DIR and GITEA_CUSTOM are also supported
		envWorkPath := getEnvFn("GITEA_WORK_PATH")
		if envWorkPath == "" {
			envWorkPath = getEnvFn("GITEA_WORK_DIR")
		}
		if envWorkPath != "" {
			tmpWorkPath = envWorkPath
			if !filepath.IsAbs(tmpWorkPath) {
				log.Fatal("GITEA_WORK_PATH must be absolute path")
			}
		}

		envCustomPath := getEnvFn("GITEA_CUSTOM_PATH")
		if envCustomPath == "" {
			envCustomPath = getEnvFn("GITEA_CUSTOM")
		}
		if envCustomPath != "" {
			tmpCustomPath = envCustomPath
			if !filepath.IsAbs(tmpCustomPath) {
				log.Fatal("GITEA_CUSTOM_PATH must be absolute path")
			}
		}
	}

	readFromArgs := func() {
		if args.WorkPath != "" {
			tmpWorkPath = args.WorkPath
			if !filepath.IsAbs(tmpWorkPath) {
				log.Fatal("--work-path must be absolute path")
			}
		}
		if args.CustomPath != "" {
			tmpCustomPath = args.CustomPath // if it is not abs, it will be based on work-path, it shouldn't happen
			if !filepath.IsAbs(tmpCustomPath) {
				log.Error("--custom-path must be absolute path")
			}
		}
		if args.CustomConf != "" {
			tmpCustomConf = args.CustomConf
			if !filepath.IsAbs(tmpCustomConf) {
				// the config path can be relative to the real current working path
				if tmpCustomConf, err = filepath.Abs(tmpCustomConf); err != nil {
					log.Fatal("Failed to get absolute path of config %q: %v", tmpCustomConf, err)
				}
			}
		}
	}

	readFromEnv()
	readFromArgs()

	if tmpCustomConf == "" {
		// need to guess the config path
		if tmpWorkPath == "" && tmpCustomPath == "" {
			// we don't have any info, so we guess it is in AppWorkPath
			tmpCustomConf = filepath.Join(AppWorkPath, "custom/conf/app.ini")
		} else if tmpWorkPath != "" {
			tmpCustomConf = filepath.Join(tmpWorkPath, "custom/conf/app.ini")
		} else {
			if filepath.IsAbs(tmpCustomPath) {
				tmpCustomConf = filepath.Join(tmpCustomPath, "conf/app.ini")
			} else {
				tmpCustomConf = filepath.Join(AppWorkPath, tmpCustomPath, "conf/app.ini")
			}
		}
	}

	// only read the config but do not load/init anything more, because the AppWorkPath and CustomPath are not ready
	InitCfgProvider(tmpCustomConf)
	configWorkPath := ConfigSectionKeyString(CfgProvider.Section(""), "WORK_PATH")
	if configWorkPath != "" {
		if !filepath.IsAbs(configWorkPath) {
			log.Fatal("WORK_PATH in %q must be absolute path", configWorkPath)
		}
		configWorkPath = filepath.Clean(configWorkPath)
		if tmpWorkPath != "" {
			envWorkPath := filepath.Clean(tmpWorkPath)
			_, err1 := filepath.Rel(configWorkPath, envWorkPath)
			_, err2 := filepath.Rel(envWorkPath, configWorkPath)
			if err1 != nil || err2 != nil {
				log.Error("WORK_PATH from config %q doesn't match other paths from environment variables or command arguments."+
					"Only WORK_PATH in config should be set and used. Please remove the other outdated work paths from environment variables and command arguments", tmpCustomConf)
			}
		}
		tmpWorkPath = configWorkPath
	}

	if tmpWorkPath == "" {
		tmpWorkPath = filepath.Clean(AppWorkPath)
	}

	if tmpCustomPath == "" {
		tmpCustomPath = filepath.Join(tmpWorkPath, "custom")
	} else if !filepath.IsAbs(tmpCustomPath) {
		tmpCustomPath = filepath.Join(tmpWorkPath, tmpCustomPath)
	} else {
		tmpCustomPath = filepath.Clean(tmpCustomPath)
	}

	AppWorkPath = tmpWorkPath
	CustomPath = tmpCustomPath
	CustomConf = tmpCustomConf

	LoadCommonSettings()
}
