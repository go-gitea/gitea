// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

type envVars map[string]string

func (e envVars) Getenv(key string) string {
	return e[key]
}

func TestInitWorkPathAndCommonConfig(t *testing.T) {
	testInit := func(defaultWorkPath, defaultCustomPath, defaultCustomConf string) {
		AppWorkPathMismatch = false
		AppWorkPath = defaultWorkPath
		appWorkPathBuiltin = defaultWorkPath
		CustomPath = defaultCustomPath
		customPathBuiltin = defaultCustomPath
		CustomConf = defaultCustomConf
		customConfBuiltin = defaultCustomConf
	}

	fp := filepath.Join

	tmpDir := t.TempDir()
	dirFoo := fp(tmpDir, "foo")
	dirBar := fp(tmpDir, "bar")
	dirXxx := fp(tmpDir, "xxx")
	dirYyy := fp(tmpDir, "yyy")

	t.Run("Default", func(t *testing.T) {
		testInit(dirFoo, "", "")
		InitWorkPathAndCommonConfig(envVars{}.Getenv, ArgWorkPathAndCustomConf{})
		assert.Equal(t, dirFoo, AppWorkPath)
		assert.Equal(t, fp(dirFoo, "custom"), CustomPath)
		assert.Equal(t, fp(dirFoo, "custom/conf/app.ini"), CustomConf)
	})

	t.Run("WorkDir(env)", func(t *testing.T) {
		testInit(dirFoo, "", "")
		InitWorkPathAndCommonConfig(envVars{"GITEA_WORK_DIR": dirBar}.Getenv, ArgWorkPathAndCustomConf{})
		assert.Equal(t, dirBar, AppWorkPath)
		assert.Equal(t, fp(dirBar, "custom"), CustomPath)
		assert.Equal(t, fp(dirBar, "custom/conf/app.ini"), CustomConf)
	})

	t.Run("WorkDir(env,arg)", func(t *testing.T) {
		testInit(dirFoo, "", "")
		InitWorkPathAndCommonConfig(envVars{"GITEA_WORK_DIR": dirBar}.Getenv, ArgWorkPathAndCustomConf{WorkPath: dirXxx})
		assert.Equal(t, dirXxx, AppWorkPath)
		assert.Equal(t, fp(dirXxx, "custom"), CustomPath)
		assert.Equal(t, fp(dirXxx, "custom/conf/app.ini"), CustomConf)
	})

	t.Run("CustomPath(env)", func(t *testing.T) {
		testInit(dirFoo, "", "")
		InitWorkPathAndCommonConfig(envVars{"GITEA_CUSTOM": fp(dirBar, "custom1")}.Getenv, ArgWorkPathAndCustomConf{})
		assert.Equal(t, dirFoo, AppWorkPath)
		assert.Equal(t, fp(dirBar, "custom1"), CustomPath)
		assert.Equal(t, fp(dirBar, "custom1/conf/app.ini"), CustomConf)
	})

	t.Run("CustomPath(env,arg)", func(t *testing.T) {
		testInit(dirFoo, "", "")
		InitWorkPathAndCommonConfig(envVars{"GITEA_CUSTOM": fp(dirBar, "custom1")}.Getenv, ArgWorkPathAndCustomConf{CustomPath: "custom2"})
		assert.Equal(t, dirFoo, AppWorkPath)
		assert.Equal(t, fp(dirFoo, "custom2"), CustomPath)
		assert.Equal(t, fp(dirFoo, "custom2/conf/app.ini"), CustomConf)
	})

	t.Run("CustomConf", func(t *testing.T) {
		testInit(dirFoo, "", "")
		InitWorkPathAndCommonConfig(envVars{}.Getenv, ArgWorkPathAndCustomConf{CustomConf: "app1.ini"})
		assert.Equal(t, dirFoo, AppWorkPath)
		cwd, _ := os.Getwd()
		assert.Equal(t, fp(cwd, "app1.ini"), CustomConf)

		testInit(dirFoo, "", "")
		InitWorkPathAndCommonConfig(envVars{}.Getenv, ArgWorkPathAndCustomConf{CustomConf: fp(dirBar, "app1.ini")})
		assert.Equal(t, dirFoo, AppWorkPath)
		assert.Equal(t, fp(dirBar, "app1.ini"), CustomConf)
	})

	t.Run("CustomConfOverrideWorkPath", func(t *testing.T) {
		iniWorkPath := fp(tmpDir, "app-workpath.ini")
		_ = os.WriteFile(iniWorkPath, []byte("WORK_PATH="+dirXxx), 0o644)

		testInit(dirFoo, "", "")
		InitWorkPathAndCommonConfig(envVars{}.Getenv, ArgWorkPathAndCustomConf{CustomConf: iniWorkPath})
		assert.Equal(t, dirXxx, AppWorkPath)
		assert.Equal(t, fp(dirXxx, "custom"), CustomPath)
		assert.Equal(t, iniWorkPath, CustomConf)
		assert.False(t, AppWorkPathMismatch)

		testInit(dirFoo, "", "")
		InitWorkPathAndCommonConfig(envVars{"GITEA_WORK_DIR": dirBar}.Getenv, ArgWorkPathAndCustomConf{CustomConf: iniWorkPath})
		assert.Equal(t, dirXxx, AppWorkPath)
		assert.Equal(t, fp(dirXxx, "custom"), CustomPath)
		assert.Equal(t, iniWorkPath, CustomConf)
		assert.True(t, AppWorkPathMismatch)

		testInit(dirFoo, "", "")
		InitWorkPathAndCommonConfig(envVars{}.Getenv, ArgWorkPathAndCustomConf{WorkPath: dirBar, CustomConf: iniWorkPath})
		assert.Equal(t, dirXxx, AppWorkPath)
		assert.Equal(t, fp(dirXxx, "custom"), CustomPath)
		assert.Equal(t, iniWorkPath, CustomConf)
		assert.True(t, AppWorkPathMismatch)
	})

	t.Run("Builtin", func(t *testing.T) {
		testInit(dirFoo, dirBar, dirXxx)
		InitWorkPathAndCommonConfig(envVars{}.Getenv, ArgWorkPathAndCustomConf{})
		assert.Equal(t, dirFoo, AppWorkPath)
		assert.Equal(t, dirBar, CustomPath)
		assert.Equal(t, dirXxx, CustomConf)

		testInit(dirFoo, "custom1", "cfg.ini")
		InitWorkPathAndCommonConfig(envVars{}.Getenv, ArgWorkPathAndCustomConf{})
		assert.Equal(t, dirFoo, AppWorkPath)
		assert.Equal(t, fp(dirFoo, "custom1"), CustomPath)
		assert.Equal(t, fp(dirFoo, "custom1/cfg.ini"), CustomConf)

		testInit(dirFoo, "custom1", "cfg.ini")
		InitWorkPathAndCommonConfig(envVars{"GITEA_WORK_DIR": dirYyy}.Getenv, ArgWorkPathAndCustomConf{})
		assert.Equal(t, dirYyy, AppWorkPath)
		assert.Equal(t, fp(dirYyy, "custom1"), CustomPath)
		assert.Equal(t, fp(dirYyy, "custom1/cfg.ini"), CustomConf)

		testInit(dirFoo, "custom1", "cfg.ini")
		InitWorkPathAndCommonConfig(envVars{"GITEA_CUSTOM": dirYyy}.Getenv, ArgWorkPathAndCustomConf{})
		assert.Equal(t, dirFoo, AppWorkPath)
		assert.Equal(t, dirYyy, CustomPath)
		assert.Equal(t, fp(dirYyy, "cfg.ini"), CustomConf)

		iniWorkPath := fp(tmpDir, "app-workpath.ini")
		_ = os.WriteFile(iniWorkPath, []byte("WORK_PATH="+dirXxx), 0o644)
		testInit(dirFoo, "custom1", "cfg.ini")
		InitWorkPathAndCommonConfig(envVars{}.Getenv, ArgWorkPathAndCustomConf{CustomConf: iniWorkPath})
		assert.Equal(t, dirXxx, AppWorkPath)
		assert.Equal(t, fp(dirXxx, "custom1"), CustomPath)
		assert.Equal(t, iniWorkPath, CustomConf)
	})
}
