// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webtheme

import (
	"context"
	"sort"
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

func initThemes(ctx context.Context) {
	availableThemes = nil
	defer func() {
		availableThemesSet = container.SetOf(availableThemes...)
		if !availableThemesSet.Contains(setting.Config().UI.DefaultTheme.Value(ctx)) {
			setting.LogStartupProblem(1, log.ERROR, "Default theme %q is not available, please correct the '[ui].DEFAULT_THEME' setting in the config file", setting.Config().UI.DefaultTheme.Value(ctx))
		}
	}()
	cssFiles, err := public.AssetFS().ListFiles("/assets/css")
	if err != nil {
		log.Error("Failed to list themes: %v", err)
		availableThemes = []string{setting.Config().UI.DefaultTheme.Value(ctx)}
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
	if len(setting.Config().UI.Themes.Value(ctx)) > 0 {
		allowedThemes := container.SetOf(setting.Config().UI.Themes.Value(ctx)...)
		for _, theme := range foundThemes {
			if allowedThemes.Contains(theme) {
				availableThemes = append(availableThemes, theme)
			}
		}
	} else {
		availableThemes = foundThemes
	}
	sort.Strings(availableThemes)
	if len(availableThemes) == 0 {
		setting.LogStartupProblem(1, log.ERROR, "No theme candidate in asset files, but Gitea requires there should be at least one usable theme")
		availableThemes = []string{setting.Config().UI.DefaultTheme.Value(ctx)}
	}
}

func GetAvailableThemes(ctx context.Context) []string {
	themeOnce.Do(func() {
		initThemes(ctx)
	})
	return availableThemes
}

func IsThemeAvailable(ctx context.Context, name string) bool {
	themeOnce.Do(func() {
		initThemes(ctx)
	})
	return availableThemesSet.Contains(name)
}
