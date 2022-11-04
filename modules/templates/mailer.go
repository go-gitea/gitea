// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package templates

import (
	"context"
	"html/template"
	"io/fs"
	"os"
	"strings"
	texttmpl "text/template"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/watcher"
)

// Mailer provides the templates required for sending notification mails.
func Mailer(ctx context.Context) (*texttmpl.Template, *template.Template) {
	for _, funcs := range NewTextFuncMap() {
		subjectTemplates.Funcs(funcs)
	}
	for _, funcs := range NewFuncMap() {
		bodyTemplates.Funcs(funcs)
	}

	refreshTemplates := func() {
		for _, assetPath := range BuiltinAssetNames() {
			if !strings.HasPrefix(assetPath, "mail/") {
				continue
			}

			if !strings.HasSuffix(assetPath, ".tmpl") {
				continue
			}

			content, err := BuiltinAsset(assetPath)
			if err != nil {
				log.Warn("Failed to read embedded %s template. %v", assetPath, err)
				continue
			}

			assetName := strings.TrimPrefix(strings.TrimSuffix(assetPath, ".tmpl"), "mail/")

			log.Trace("Adding built-in mailer template for %s", assetName)
			buildSubjectBodyTemplate(subjectTemplates,
				bodyTemplates,
				assetName,
				content)
		}

		if err := walkMailerTemplates(func(path, name string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				log.Warn("Failed to read custom %s template. %v", path, err)
				return nil
			}

			assetName := strings.TrimSuffix(name, ".tmpl")
			log.Trace("Adding mailer template for %s from %q", assetName, path)
			buildSubjectBodyTemplate(subjectTemplates,
				bodyTemplates,
				assetName,
				content)
			return nil
		}); err != nil && !os.IsNotExist(err) {
			log.Warn("Error whilst walking mailer templates directories. %v", err)
		}
	}

	refreshTemplates()

	if !setting.IsProd {
		// Now subjectTemplates and bodyTemplates are both synchronized
		// thus it is safe to call refresh from a different goroutine
		watcher.CreateWatcher(ctx, "Mailer Templates", &watcher.CreateWatcherOpts{
			PathsCallback:   walkMailerTemplates,
			BetweenCallback: refreshTemplates,
		})
	}

	return subjectTemplates, bodyTemplates
}
