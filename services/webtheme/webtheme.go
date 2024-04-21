// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webtheme

import (
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/public"
	"code.gitea.io/gitea/modules/setting"
)

var (
	availableThemes    []string
	availableThemesSet container.Set[string]
	themeOnce          sync.Once
)

func initThemes() {
	availableThemes = nil
	defer func() {
		availableThemesSet = container.SetOf(availableThemes...)
	}()
	cssFiles, err := public.AssetFS().ListFiles("/assets/css")
	if err != nil {
		log.Error("Failed to list themes: %v", err)
		availableThemes = []string{setting.UI.DefaultTheme}
		return
	}
	var foundThemes []string
	for _, name := range cssFiles {
		name, ok := strings.CutPrefix(name, "theme-")
		if !ok {
			continue
		}
		name, ok = strings.CutSuffix(name, ".css")
		if !ok {
			continue
		}
		foundThemes = append(foundThemes, name)
	}
	if len(setting.UI.Themes) > 0 {
		allowedThemes := container.SetOf(setting.UI.Themes...)
		for _, theme := range foundThemes {
			if allowedThemes.Contains(theme) {
				availableThemes = append(availableThemes, theme)
			}
		}
	} else {
		availableThemes = foundThemes
	}
	if len(availableThemes) == 0 {
		log.Error("No theme candidate, but gitea requires there should be at least one usable theme")
		availableThemes = []string{setting.UI.DefaultTheme}
	}
}

func GetAvailableThemes() []string {
	themeOnce.Do(initThemes)
	return availableThemes
}

func IsThemeAvailable(name string) bool {
	themeOnce.Do(initThemes)
	return availableThemesSet.Contains(name)
}
