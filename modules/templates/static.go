// +build bindata

// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package templates

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"path"
	"strings"
	texttmpl "text/template"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"gitea.com/macaron/macaron"
	"github.com/unknwon/com"
)

var (
	subjectTemplates = texttmpl.New("")
	bodyTemplates    = template.New("")
)

type templateFileSystem struct {
	files []macaron.TemplateFile
}

func (templates templateFileSystem) ListFiles() []macaron.TemplateFile {
	return templates.files
}

func (templates templateFileSystem) Get(name string) (io.Reader, error) {
	for i := range templates.files {
		if templates.files[i].Name()+templates.files[i].Ext() == name {
			return bytes.NewReader(templates.files[i].Data()), nil
		}
	}

	return nil, fmt.Errorf("file '%s' not found", name)
}

func NewTemplateFileSystem() templateFileSystem {
	fs := templateFileSystem{}
	fs.files = make([]macaron.TemplateFile, 0, 10)

	for _, assetPath := range AssetNames() {
		if strings.HasPrefix(assetPath, "mail/") {
			continue
		}

		if !strings.HasSuffix(assetPath, ".tmpl") {
			continue
		}

		content, err := Asset(assetPath)

		if err != nil {
			log.Warn("Failed to read embedded %s template. %v", assetPath, err)
			continue
		}

		fs.files = append(fs.files, macaron.NewTplFile(
			strings.TrimSuffix(
				assetPath,
				".tmpl",
			),
			content,
			".tmpl",
		))
	}

	customDir := path.Join(setting.CustomPath, "templates")

	if com.IsDir(customDir) {
		files, err := com.StatDir(customDir)

		if err != nil {
			log.Warn("Failed to read %s templates dir. %v", customDir, err)
		} else {
			for _, filePath := range files {
				if strings.HasPrefix(filePath, "mail/") {
					continue
				}

				if !strings.HasSuffix(filePath, ".tmpl") {
					continue
				}

				content, err := ioutil.ReadFile(path.Join(customDir, filePath))

				if err != nil {
					log.Warn("Failed to read custom %s template. %v", filePath, err)
					continue
				}

				fs.files = append(fs.files, macaron.NewTplFile(
					strings.TrimSuffix(
						filePath,
						".tmpl",
					),
					content,
					".tmpl",
				))
			}
		}
	}

	return fs
}

// HTMLRenderer implements the macaron handler for serving HTML templates.
func HTMLRenderer() macaron.Handler {
	return macaron.Renderer(macaron.RenderOptions{
		Funcs:              NewFuncMap(),
		TemplateFileSystem: NewTemplateFileSystem(),
	})
}

// JSONRenderer implements the macaron handler for serving JSON templates.
func JSONRenderer() macaron.Handler {
	return macaron.Renderer(macaron.RenderOptions{
		Funcs:              NewFuncMap(),
		TemplateFileSystem: NewTemplateFileSystem(),
		HTMLContentType:    "application/json",
	})
}

// JSRenderer implements the macaron handler for serving JS templates.
func JSRenderer() macaron.Handler {
	return macaron.Renderer(macaron.RenderOptions{
		Funcs:              NewFuncMap(),
		TemplateFileSystem: NewTemplateFileSystem(),
		HTMLContentType:    "application/javascript",
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

	for _, assetPath := range AssetNames() {
		if !strings.HasPrefix(assetPath, "mail/") {
			continue
		}

		if !strings.HasSuffix(assetPath, ".tmpl") {
			continue
		}

		content, err := Asset(assetPath)

		if err != nil {
			log.Warn("Failed to read embedded %s template. %v", assetPath, err)
			continue
		}

		buildSubjectBodyTemplate(subjectTemplates,
			bodyTemplates,
			strings.TrimPrefix(
				strings.TrimSuffix(
					assetPath,
					".tmpl",
				),
				"mail/",
			),
			content)
	}

	customDir := path.Join(setting.CustomPath, "templates", "mail")

	if com.IsDir(customDir) {
		files, err := com.StatDir(customDir)

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

				buildSubjectBodyTemplate(subjectTemplates,
					bodyTemplates,
					strings.TrimSuffix(
						filePath,
						".tmpl",
					),
					content)
			}
		}
	}

	return subjectTemplates, bodyTemplates
}

func Asset(name string) ([]byte, error) {
	f, err := Assets.Open("/" + name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ioutil.ReadAll(f)
}

func AssetNames() []string {
	realFS := Assets.(vfsgen۰FS)
	var results = make([]string, 0, len(realFS))
	for k := range realFS {
		results = append(results, k[1:])
	}
	return results
}
