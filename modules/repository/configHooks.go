/*
Copyright (c) Huawei Technologies Co., Ltd. 2024. All rights reserved
*/

// Package repository for config hook list

package repository

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/util/sets"

	"code.gitea.io/gitea/modules/util"
)

var validHookType sets.String

func init() {
	hookTypeList := []string{"pre-receive", "update", "post-receive"}
	validHookType = sets.NewString()
	for _, v := range hookTypeList {
		validHookType.Insert(v)
	}
}

// 定义一个 Hook 接口
type Hook interface {
	GetHookName() string
	GetHookContent() string
}

type HookRegistry struct {
	hooks map[string]Hook
}

func NewHookRegistry() (*HookRegistry, error) {
	return &HookRegistry{
		hooks: make(map[string]Hook),
	}, nil
}

func (r *HookRegistry) RegisterHook(name string, hook Hook) {
	r.hooks[name] = hook
}

// CreateConfigHook is create a config hook
func (r *HookRegistry) RunCreateConfigHooks(hookDir, hookName string) error {
	baseHookPath := filepath.Join(hookDir, hookName+".d")
	for hookType, hook := range r.hooks {
		if hookType != hookName {
			continue
		}
		content := hook.GetHookContent()
		newHookPath := filepath.Join(baseHookPath, hook.GetHookName())
		if err := util.Remove(newHookPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("unable to pre-remove new hook file '%s' prior to rewriting: %w", newHookPath, err)
		}
		if err := os.WriteFile(newHookPath, []byte(content), 0o777); err != nil {
			return fmt.Errorf("write new check license hook file '%s': %w", newHookPath, err)
		}
		if err := ensureExecutable(newHookPath); err != nil {
			return fmt.Errorf("Unable to set %s executable. Error %w", newHookPath, err)
		}
	}
	return nil
}

// CreateConfigHook is create a config hook
func (r *HookRegistry) RunCheckConfigPathHooks(results []string, hookDir, hookName string) bool {
	baseHookPath := filepath.Join(hookDir, hookName+".d")
	for hookType, hook := range r.hooks {
		if hookType != hookName {
			continue
		}
		newHookPath := filepath.Join(baseHookPath, hook.GetHookName())
		isExist, err := util.IsExist(newHookPath)
		if err != nil {
			results = append(results, fmt.Sprintf("unable to check if %s exists. Error: %v", newHookPath, err))
		}
		if err == nil && !isExist {
			results = append(results, fmt.Sprintf("new hook file %s does not exist", newHookPath))
			return true
		}
	}
	return false
}

// CheckConfigHook is check config hook
func (r *HookRegistry) RunCheckConfigHooks(results []string, hookDir, hookName string) error {
	baseHookPath := filepath.Join(hookDir, hookName+".d")
	for hookType, hook := range r.hooks {
		if hookType != hookName {
			continue
		}
		newHookPath := filepath.Join(baseHookPath, hook.GetHookName())
		contents, err := os.ReadFile(newHookPath)
		if err != nil {
			return err
		}
		if string(contents) != hook.GetHookContent() {
			results = append(results, fmt.Sprintf("new hook file %s is out of date", newHookPath))
		}
		if !checkExecutable(newHookPath) {
			results = append(results, fmt.Sprintf("new hook file %s is not executable", newHookPath))
		}
	}
	return nil
}
