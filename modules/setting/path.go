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

	// AppWorkPath is the "working directory" of Gitea. It maps to the: WORK_PATH in app.ini, "--work-path" flag, environment variable GITEA_WORK_DIR.
	// If that is not set it is the default set here by the linker or failing that the directory of AppPath.
	// It is used as the base path for several other paths.
	AppWorkPath string
	CustomPath  string // Custom directory path. Env: GITEA_CUSTOM
	CustomConf  string

	appWorkPathBuiltin string
	customPathBuiltin  string
	customConfBuiltin  string

	AppWorkPathMismatch bool
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

	if AppWorkPath == "" {
		AppWorkPath = filepath.Dir(AppPath)
	}

	appWorkPathBuiltin = AppWorkPath
	customPathBuiltin = CustomPath
	customConfBuiltin = CustomConf
}

type ArgWorkPathAndCustomConf struct {
	WorkPath   string
	CustomPath string
	CustomConf string
}

type stringWithDefault struct {
	Value string
	IsSet bool
}

func (s *stringWithDefault) Set(v string) {
	s.Value = v
	s.IsSet = true
}

// InitWorkPathAndCommonConfig will set AppWorkPath, CustomPath and CustomConf, init default config provider by CustomConf and load common settings,
func InitWorkPathAndCommonConfig(getEnvFn func(name string) string, args ArgWorkPathAndCustomConf) {
	InitWorkPathAndCfgProvider(getEnvFn, args)
	LoadCommonSettings()
}

// InitWorkPathAndCfgProvider will set AppWorkPath, CustomPath and CustomConf, init default config provider by CustomConf
func InitWorkPathAndCfgProvider(getEnvFn func(name string) string, args ArgWorkPathAndCustomConf) {
	tryAbsPath := func(paths ...string) string {
		s := paths[len(paths)-1]
		for i := len(paths) - 2; i >= 0; i-- {
			if filepath.IsAbs(s) {
				break
			}
			s = filepath.Join(paths[i], s)
		}
		return s
	}

	var err error
	tmpWorkPath := stringWithDefault{Value: appWorkPathBuiltin}
	if tmpWorkPath.Value == "" {
		tmpWorkPath.Value = filepath.Dir(AppPath)
	}
	tmpCustomPath := stringWithDefault{Value: customPathBuiltin}
	if tmpCustomPath.Value == "" {
		tmpCustomPath.Value = "custom"
	}
	tmpCustomConf := stringWithDefault{Value: customConfBuiltin}
	if tmpCustomConf.Value == "" {
		tmpCustomConf.Value = "conf/app.ini"
	}

	readFromEnv := func() {
		envWorkPath := getEnvFn("GITEA_WORK_DIR")
		if envWorkPath != "" {
			tmpWorkPath.Set(envWorkPath)
			if !filepath.IsAbs(tmpWorkPath.Value) {
				log.Fatal("GITEA_WORK_DIR (work path) must be absolute path")
			}
		}

		envCustomPath := getEnvFn("GITEA_CUSTOM")
		if envCustomPath != "" {
			tmpCustomPath.Set(envCustomPath)
			if !filepath.IsAbs(tmpCustomPath.Value) {
				log.Fatal("GITEA_CUSTOM (custom path) must be absolute path")
			}
		}
	}

	readFromArgs := func() {
		if args.WorkPath != "" {
			tmpWorkPath.Set(args.WorkPath)
			if !filepath.IsAbs(tmpWorkPath.Value) {
				log.Fatal("--work-path must be absolute path")
			}
		}
		if args.CustomPath != "" {
			tmpCustomPath.Set(args.CustomPath) // if it is not abs, it will be based on work-path, it shouldn't happen
			if !filepath.IsAbs(tmpCustomPath.Value) {
				log.Error("--custom-path must be absolute path")
			}
		}
		if args.CustomConf != "" {
			tmpCustomConf.Set(args.CustomConf)
			if !filepath.IsAbs(tmpCustomConf.Value) {
				// the config path can be relative to the real current working path
				if tmpCustomConf.Value, err = filepath.Abs(tmpCustomConf.Value); err != nil {
					log.Fatal("Failed to get absolute path of config %q: %v", tmpCustomConf.Value, err)
				}
			}
		}
	}

	readFromEnv()
	readFromArgs()

	if !tmpCustomConf.IsSet {
		tmpCustomConf.Set(tryAbsPath(tmpWorkPath.Value, tmpCustomPath.Value, tmpCustomConf.Value))
	}

	// only read the config but do not load/init anything more, because the AppWorkPath and CustomPath are not ready
	InitCfgProvider(tmpCustomConf.Value)
	if HasInstallLock(CfgProvider) {
		ClearEnvConfigKeys() // if the instance has been installed, do not pass the environment variables to sub-processes
	}
	configWorkPath := ConfigSectionKeyString(CfgProvider.Section(""), "WORK_PATH")
	if configWorkPath != "" {
		if !filepath.IsAbs(configWorkPath) {
			log.Fatal("WORK_PATH in %q must be absolute path", configWorkPath)
		}
		configWorkPath = filepath.Clean(configWorkPath)
		if tmpWorkPath.Value != "" && (getEnvFn("GITEA_WORK_DIR") != "" || args.WorkPath != "") {
			fi1, err1 := os.Stat(tmpWorkPath.Value)
			fi2, err2 := os.Stat(configWorkPath)
			if err1 != nil || err2 != nil || !os.SameFile(fi1, fi2) {
				AppWorkPathMismatch = true
			}
		}
		tmpWorkPath.Set(configWorkPath)
	}

	tmpCustomPath.Set(tryAbsPath(tmpWorkPath.Value, tmpCustomPath.Value))

	AppWorkPath = tmpWorkPath.Value
	CustomPath = tmpCustomPath.Value
	CustomConf = tmpCustomConf.Value
}
