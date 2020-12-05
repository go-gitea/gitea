// +build !bindata

// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package templates

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	texttmpl "text/template"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"gitea.com/macaron/macaron"
)

var (
	subjectTemplates = texttmpl.New("")
	bodyTemplates    = template.New("")
)

func GetAsset(name string) ([]byte, error) {
	fmt.Println("=====2", name)

	bs, err := ioutil.ReadFile(filepath.Join(setting.CustomPath, name))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	} else if err == nil {
		return bs, nil
	}

	return ioutil.ReadFile(filepath.Join(setting.StaticRootPath, name))
}

func GetAssetNames() []string {
	tmpls := getDirAssetNames(filepath.Join(setting.CustomPath, "templates"))
	tmpls2 := getDirAssetNames(filepath.Join(setting.StaticRootPath, "templates"))
	return append(tmpls, tmpls2...)
}

func getDirAssetNames(dir string) []string {
	var tmpls []string
	isDir, err := util.IsDir(dir)
	if err != nil {
		log.Warn("Unable to check if templates dir %s is a directory. Error: %v", dir, err)
		return tmpls
	}
	if !isDir {
		log.Warn("Templates dir %s is a not directory.", dir)
		return tmpls
	}

	files, err := com.StatDir(dir)
	if err != nil {
		log.Warn("Failed to read %s templates dir. %v", dir, err)
		return tmpls
	}
	for _, filePath := range files {
		if strings.HasPrefix(filePath, "mail/") {
			continue
		}

		if !strings.HasSuffix(filePath, ".tmpl") {
			continue
		}

		fmt.Println("=======3333", filePath, filePath)

		tmpls = append(tmpls, "templates/"+filePath)
	}
	return tmpls
}

// HTMLRenderer implements the macaron handler for serving HTML templates.
func HTMLRenderer() macaron.Handler {
	return macaron.Renderer(macaron.RenderOptions{
		Funcs:     NewFuncMap(),
		Directory: path.Join(setting.StaticRootPath, "templates"),
		AppendDirectories: []string{
			path.Join(setting.CustomPath, "templates"),
		},
	})
}

// JSONRenderer implements the macaron handler for serving JSON templates.
func JSONRenderer() macaron.Handler {
	return macaron.Renderer(macaron.RenderOptions{
		Funcs:     NewFuncMap(),
		Directory: path.Join(setting.StaticRootPath, "templates"),
		AppendDirectories: []string{
			path.Join(setting.CustomPath, "templates"),
		},
		HTMLContentType: "application/json",
	})
}

// Mailer provides the templates required for sending notification mails.
func Mailer() (*texttmpl.Template, *template.Template) {
	for _, funcs := range NewTextFuncMap() {
		subjectTemplates.Funcs(funcs)
	}
	for _, funcs := range NewFuncMap() {
		bodyTemplates.Funcs(funcs)
	}

	staticDir := path.Join(setting.StaticRootPath, "templates", "mail")

	isDir, err := util.IsDir(staticDir)
	if err != nil {
		log.Warn("Unable to check if templates dir %s is a directory. Error: %v", staticDir, err)
	}
	if isDir {
		files, err := util.StatDir(staticDir)

		if err != nil {
			log.Warn("Failed to read %s templates dir. %v", staticDir, err)
		} else {
			for _, filePath := range files {
				if !strings.HasSuffix(filePath, ".tmpl") {
					continue
				}

				content, err := ioutil.ReadFile(path.Join(staticDir, filePath))

				if err != nil {
					log.Warn("Failed to read static %s template. %v", filePath, err)
					continue
				}

				buildSubjectBodyTemplate(subjectTemplates, bodyTemplates, strings.TrimSuffix(filePath, ".tmpl"), content)
			}
		}
	}

	customDir := path.Join(setting.CustomPath, "templates", "mail")

	isDir, err = util.IsDir(customDir)
	if err != nil {
		log.Warn("Unable to check if templates dir %s is a directory. Error: %v", customDir, err)
	}
	if isDir {
		files, err := util.StatDir(customDir)

		if err != nil {
			log.Warn("Failed to read %s templates dir. %v", customDir, err)
		} else {
			for _, filePath := range files {
				if !strings.HasSuffix(filePath, ".tmpl") {
					continue
				}

				content, err := ioutil.ReadFile(path.Join(customDir, filePath))

				if err != nil {
					log.Warn("Failed to read custom %s template. %v", filePath, err)
					continue
				}

				buildSubjectBodyTemplate(subjectTemplates, bodyTemplates, strings.TrimSuffix(filePath, ".tmpl"), content)
			}
		}
	}

	return subjectTemplates, bodyTemplates
}
