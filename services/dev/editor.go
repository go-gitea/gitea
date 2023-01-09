// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dev

import (
	"html/template"
	"strings"

	"code.gitea.io/gitea/models/system"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
)

const KeyDevDefaultEditor = "dev.default_editor"

type Editor struct {
	Name string
	URL  string
	Icon string
}

func (e *Editor) RenderURL(repoURL string) template.URL {
	return template.URL(strings.ReplaceAll(e.URL, "${repo_url}", repoURL))
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

type ErrUnknownEditor struct {
	editorName string
}

func (e ErrUnknownEditor) Error() string {
	return "Unknown editor: " + e.editorName
}

func GetUserDefaultEditor(userID int64) (*Editor, error) {
	defaultName, err := user_model.GetSetting(userID, KeyDevDefaultEditor)
	if err != nil {
		return nil, err
	}
	for _, editor := range defaultEditors {
		if editor.Name == defaultName {
			return &editor, nil
		}
	}
	return nil, ErrUnknownEditor{defaultName}
}

func SetUserDefaultEditor(userID int64, name string) error {
	return user_model.SetUserSetting(userID, KeyDevDefaultEditor, name)
}

func GetUserDefaultEditorWithFallback(user *user_model.User) (*Editor, error) {
	if user == nil || user.ID <= 0 {
		return GetDefaultEditor()
	}
	editor, err := GetUserDefaultEditor(user.ID)
	if err == nil {
		return editor, nil
	}

	if theErr, ok := err.(ErrUnknownEditor); ok {
		log.Error("Unknown editor for user %d: %s, fallback to system default", user.ID, theErr.editorName)
		return GetDefaultEditor()
	}
	if user_model.IsErrUserSettingIsNotExist(err) {
		return GetDefaultEditor()
	}
	return nil, err
}
