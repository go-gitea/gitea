// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dev

import (
	"html/template"
	"strings"

	"code.gitea.io/gitea/models/system"
)

const KeyDevDefaultEditor = "dev.default_editor"

type Editor struct {
	Name string
	URL  string
	Icon string
}

func (e *Editor) RenderURL(repoURL string) template.URL {
	return template.URL(strings.Replace(e.URL, "${repo_url}", repoURL, -1))
}

var defaultEditors = []Editor{
	{
		Name: "VS Code",
		URL:  "vscode://vscode.git/clone?url=${repo_url}",
		Icon: `gitea-vscode`,
	},
	{
		Name: "VSCodium",
		URL:  "vscodium://vscode.git/clone?url=${repo_url}",
		Icon: `gitea-vscodium`,
	},
}

func GetEditorByName(name string) *Editor {
	for _, editor := range defaultEditors {
		if editor.Name == name {
			return &editor
		}
	}
	return nil
}

// GetEditors returns all editors
func GetEditors() ([]Editor, error) {
	return defaultEditors, nil
}

func DefaultEditorName() string {
	return defaultEditors[0].Name
}

func GetDefaultEditor() (*Editor, error) {
	defaultName, err := system.GetSetting(KeyDevDefaultEditor)
	if err != nil && !system.IsErrSettingIsNotExist(err) {
		return nil, err
	}
	for _, editor := range defaultEditors {
		if editor.Name == defaultName {
			return &editor, nil
		}
	}
	return &defaultEditors[0], nil
}

func SetDefaultEditor(name string) error {
	for _, editor := range defaultEditors {
		if editor.Name == name {
			return system.SetSetting(&system.Setting{
				SettingKey:   KeyDevDefaultEditor,
				SettingValue: name,
			})
		}
	}
	return nil
}
