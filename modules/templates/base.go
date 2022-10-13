// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package templates

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// Vars represents variables to be render in golang templates
type Vars map[string]interface{}

// Merge merges another vars to the current, another Vars will override the current
func (vars Vars) Merge(another map[string]interface{}) Vars {
	for k, v := range another {
		vars[k] = v
	}
	return vars
}

// BaseVars returns all basic vars
func BaseVars() Vars {
	startTime := time.Now()
	return map[string]interface{}{
		"IsLandingPageHome":          setting.LandingPageURL == setting.LandingPageHome,
		"IsLandingPageExplore":       setting.LandingPageURL == setting.LandingPageExplore,
		"IsLandingPageOrganizations": setting.LandingPageURL == setting.LandingPageOrganizations,

		"ShowRegistrationButton":        setting.Service.ShowRegistrationButton,
		"ShowMilestonesDashboardPage":   setting.Service.ShowMilestonesDashboardPage,
		"ShowFooterBranding":            setting.ShowFooterBranding,
		"ShowFooterVersion":             setting.ShowFooterVersion,
		"DisableDownloadSourceArchives": setting.Repository.DisableDownloadSourceArchives,

		"EnableSwagger":      setting.API.EnableSwagger,
		"EnableOpenIDSignIn": setting.Service.EnableOpenIDSignIn,
		"PageStartTime":      startTime,
	}
}

func getDirTemplateAssetNames(dir string) []string {
	return getDirAssetNames(dir, false)
}

func getDirAssetNames(dir string, mailer bool) []string {
	var tmpls []string

	if mailer {
		dir += filepath.Join(dir, "mail")
	}
	f, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return tmpls
		}
		log.Warn("Unable to check if templates dir %s is a directory. Error: %v", dir, err)
		return tmpls
	}
	if !f.IsDir() {
		log.Warn("Templates dir %s is a not directory.", dir)
		return tmpls
	}

	files, err := util.StatDir(dir)
	if err != nil {
		log.Warn("Failed to read %s templates dir. %v", dir, err)
		return tmpls
	}

	prefix := "templates/"
	if mailer {
		prefix += "mail/"
	}
	for _, filePath := range files {
		if !mailer && strings.HasPrefix(filePath, "mail/") {
			continue
		}

		if !strings.HasSuffix(filePath, ".tmpl") {
			continue
		}

		tmpls = append(tmpls, prefix+filePath)
	}
	return tmpls
}

func walkAssetDir(root string, skipMail bool, callback func(path, name string, d fs.DirEntry, err error) error) error {
	mailRoot := filepath.Join(root, "mail")
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		name := path[len(root):]
		if len(name) > 0 && name[0] == '/' {
			name = name[1:]
		}
		if err != nil {
			if os.IsNotExist(err) {
				return callback(path, name, d, err)
			}
			return err
		}
		if skipMail && path == mailRoot && d.IsDir() {
			return fs.SkipDir
		}
		if util.CommonSkip(d.Name()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".tmpl") || d.IsDir() {
			return callback(path, name, d, err)
		}
		return nil
	}); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unable to get files for template assets in %s: %w", root, err)
	}
	return nil
}
