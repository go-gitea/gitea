// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"context"
	"html/template"
	"regexp"
	"strings"
	texttmpl "text/template"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

var mailSubjectSplit = regexp.MustCompile(`(?m)^-{3,}\s*$`)

// mailSubjectTextFuncMap returns functions for injecting to text templates, it's only used for mail subject
func mailSubjectTextFuncMap() texttmpl.FuncMap {
	return texttmpl.FuncMap{
		"dict": dict,
		"Eval": Eval,

		"EllipsisString": base.EllipsisString,
		"AppName": func() string {
			return setting.AppName
		},
		"AppDomain": func() string { // documented in mail-templates.md
			return setting.Domain
		},
	}
}

func buildSubjectBodyTemplate(stpl *texttmpl.Template, btpl *template.Template, name string, content []byte) {
	// Split template into subject and body
	var subjectContent []byte
	bodyContent := content
	loc := mailSubjectSplit.FindIndex(content)
	if loc != nil {
		subjectContent = content[0:loc[0]]
		bodyContent = content[loc[1]:]
	}
	if _, err := stpl.New(name).
		Parse(string(subjectContent)); err != nil {
		log.Warn("Failed to parse template [%s/subject]: %v", name, err)
	}
	if _, err := btpl.New(name).
		Parse(string(bodyContent)); err != nil {
		log.Warn("Failed to parse template [%s/body]: %v", name, err)
	}
}

// Mailer provides the templates required for sending notification mails.
func Mailer(ctx context.Context) (*texttmpl.Template, *template.Template) {
	subjectTemplates := texttmpl.New("")
	bodyTemplates := template.New("")

	subjectTemplates.Funcs(mailSubjectTextFuncMap())
	bodyTemplates.Funcs(NewFuncMap())

	assetFS := AssetFS()
	refreshTemplates := func(firstRun bool) {
		if !firstRun {
			log.Trace("Reloading mail templates")
		}
		assetPaths, err := ListMailTemplateAssetNames(assetFS)
		if err != nil {
			log.Error("Failed to list mail templates: %v", err)
			return
		}

		for _, assetPath := range assetPaths {
			content, layerName, err := assetFS.ReadLayeredFile(assetPath)
			if err != nil {
				log.Warn("Failed to read mail template %s by %s: %v", assetPath, layerName, err)
				continue
			}
			tmplName := strings.TrimPrefix(strings.TrimSuffix(assetPath, ".tmpl"), "mail/")
			if firstRun {
				log.Trace("Adding mail template %s: %s by %s", tmplName, assetPath, layerName)
			}
			buildSubjectBodyTemplate(subjectTemplates, bodyTemplates, tmplName, content)
		}
	}

	refreshTemplates(true)

	if !setting.IsProd {
		// Now subjectTemplates and bodyTemplates are both synchronized
		// thus it is safe to call refresh from a different goroutine
		go assetFS.WatchLocalChanges(ctx, func() {
			refreshTemplates(false)
		})
	}

	return subjectTemplates, bodyTemplates
}
