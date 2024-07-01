// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

var hookNames = []string{"pre-receive", "update", "post-receive"}

// hookNames contains the hook name (key) linked with the filename of the resulting hook.
func GetHookNames() map[string]string {
	return map[string]string{
		"pre-receive":  setting.GitHookPrereceiveName,
		"update":       setting.GitHookUpdateName,
		"post-receive": setting.GitHookPostreceiveName,
	}
}

// Hook represents a Git hook.
type Hook struct {
	name     string
	IsActive bool   // Indicates whether repository has this hook.
	Content  string // Content of hook if it's active.
	Sample   string // Sample content from Git.
	path     string // Hook file path.
}

// ErrNotValidHook error when a git hook is not valid
var ErrNotValidHook = errors.New("not a valid Git hook")

// IsValidHookName returns true if given name is a valid Git hook.
func IsValidHookName(name string) bool {
	for hn := range GetHookNames() {
		if hn == name {
			return true
		}
	}
	return false
}

// GetHook returns a Git hook by given name and repository.
func GetHook(repoPath, name string) (*Hook, error) {
	if !IsValidHookName(name) {
		return nil, ErrNotValidHook
	}
	h := &Hook{
		name: name,
		path: filepath.Join(repoPath, "hooks", name+".d", GetHookNames()[name]),
	}
	samplePath := filepath.Join(repoPath, "hooks", name+".sample")
	if isFile(h.path) {
		data, err := os.ReadFile(h.path)
		if err != nil {
			return nil, err
		}
		h.IsActive = true
		h.Content = string(data)
	} else if isFile(samplePath) {
		data, err := os.ReadFile(samplePath)
		if err != nil {
			return nil, err
		}
		h.Sample = string(data)
	}
	return h, nil
}

// Name return the name of the hook
func (h *Hook) Name() string {
	return h.name
}

// Update updates hook settings.
func (h *Hook) Update() error {
	if len(strings.TrimSpace(h.Content)) == 0 {
		if isExist(h.path) {
			err := util.Remove(h.path)
			if err != nil {
				return err
			}
		}
		h.IsActive = false
		return nil
	}
	d := filepath.Dir(h.path)
	if err := os.MkdirAll(d, os.ModePerm); err != nil {
		return err
	}

	err := os.WriteFile(h.path, []byte(strings.ReplaceAll(h.Content, "\r", "")), os.ModePerm)
	if err != nil {
		return err
	}
	h.IsActive = true
	return nil
}

// ListHooks returns a list of Git hooks of given repository.
func ListHooks(repoPath string) (_ []*Hook, err error) {
	if !isDir(path.Join(repoPath, "hooks")) {
		return nil, errors.New("hooks path does not exist")
	}

	hooks := make([]*Hook, len(GetHookNames()))
	for i := range hookNames {
		hooks[i], err = GetHook(repoPath, hookNames[i])
		if err != nil {
			return nil, err
		}
	}
	return hooks, nil
}

const (
	// HookPathUpdate hook update path
	HookPathUpdate = "hooks/update"
)

// SetUpdateHook writes given content to update hook of the repository.
func SetUpdateHook(repoPath, content string) (err error) {
	log.Debug("Setting update hook: %s", repoPath)
	hookPath := path.Join(repoPath, HookPathUpdate)
	isExist, err := util.IsExist(hookPath)
	if err != nil {
		log.Debug("Unable to check if %s exists. Error: %v", hookPath, err)
		return err
	}
	if isExist {
		err = util.Remove(hookPath)
	} else {
		err = os.MkdirAll(path.Dir(hookPath), os.ModePerm)
	}
	if err != nil {
		return err
	}
	return os.WriteFile(hookPath, []byte(content), 0o777)
}
