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

const KeyDevDefaultEditors = "dev.default_editors"

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
	{
		Name: "IDEA",
		URL:  "jetbrains://idea/checkout/git?idea.required.plugins.id=Git4Idea&checkout.repo=${repo-url}",
		Icon: `gitea-idea`,
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

func GetEditorsByNames(names []string) []*Editor {
	editors := make([]*Editor, 0, len(names))
	for _, name := range names {
		if editor := GetEditorByName(name); editor != nil {
			editors = append(editors, editor)
		} else {
			log.Error("Unknown editor: %s", name)
		}
	}
	return editors
}

// GetEditors returns all editors
func GetEditors() ([]Editor, error) {
	return defaultEditors, nil
}

func DefaultEditorsNames() string {
	return defaultEditors[0].Name
}

func GetDefaultEditors() ([]*Editor, error) {
	defaultNames, err := system.GetSetting(KeyDevDefaultEditors)
	if err != nil && !system.IsErrSettingIsNotExist(err) {
		return nil, err
	}
	names := strings.Split(defaultNames, ",")
	return GetEditorsByNames(names), nil
}

func SetDefaultEditors(names []string) error {
	var validateNames []string
	for _, name := range names {
		if editor := GetEditorByName(name); editor != nil {
			validateNames = append(validateNames, name)
		}
	}

	return system.SetSetting(&system.Setting{
		SettingKey:   KeyDevDefaultEditors,
		SettingValue: strings.Join(validateNames, ","),
	})
}

type ErrUnknownEditor struct {
	editorName string
}

func (e ErrUnknownEditor) Error() string {
	return "Unknown editor: " + e.editorName
}

func GetUserDefaultEditors(userID int64) ([]*Editor, error) {
	defaultNames, err := user_model.GetSetting(userID, KeyDevDefaultEditors)
	if err != nil {
		return nil, err
	}
	names := strings.Split(defaultNames, ",")
	return GetEditorsByNames(names), nil
}

func SetUserDefaultEditors(userID int64, names []string) error {
	var validateNames []string
	for _, name := range names {
		if editor := GetEditorByName(name); editor != nil {
			validateNames = append(validateNames, name)
		}
	}
	return user_model.SetUserSetting(userID, KeyDevDefaultEditors, strings.Join(validateNames, ","))
}

func GetUserDefaultEditorsWithFallback(user *user_model.User) ([]*Editor, error) {
	if user == nil || user.ID <= 0 {
		return GetDefaultEditors()
	}
	editor, err := GetUserDefaultEditors(user.ID)
	if err == nil {
		return editor, nil
	}

	if theErr, ok := err.(ErrUnknownEditor); ok {
		log.Error("Unknown editor for user %d: %s, fallback to system default", user.ID, theErr.editorName)
		return GetDefaultEditors()
	}
	if user_model.IsErrUserSettingIsNotExist(err) {
		return GetDefaultEditors()
	}
	return nil, err
}
