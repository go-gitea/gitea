// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webtheme

import (
	"regexp"
	"sort"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/public"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

var (
	availableThemes   []*ThemeMetaInfo
	availableThemeMap map[string]*ThemeMetaInfo
	themeOnce         sync.Once
)

const (
	fileNamePrefix = "theme-"
	fileNameSuffix = ".css"
)

type ThemeMetaInfo struct {
	FileName       string
	InternalName   string
	DisplayName    string
	ColorblindType string
	ColorScheme    string
}

func (info *ThemeMetaInfo) GetDescription() string {
	if info.ColorblindType == "red-green" {
		return "Red-green colorblind friendly"
	}
	if info.ColorblindType == "blue-yellow" {
		return "Blue-yellow colorblind friendly"
	}
	return ""
}

func (info *ThemeMetaInfo) GetExtraIconName() string {
	if info.ColorblindType == "red-green" {
		return "gitea-colorblind-redgreen"
	}
	if info.ColorblindType == "blue-yellow" {
		return "gitea-colorblind-blueyellow"
	}
	return ""
}

func parseThemeMetaInfoToMap(cssContent string) map[string]string {
	/*
		The theme meta info is stored in the CSS file's variables of `gitea-theme-meta-info` element,
		which is a privately defined and is only used by backend to extract the meta info.
		Not using ":root" because it is difficult to parse various ":root" blocks when importing other files,
		it is difficult to control the overriding, and it's difficult to avoid user's customized overridden styles.
	*/
	metaInfoContent := cssContent
	if pos := strings.LastIndex(metaInfoContent, "gitea-theme-meta-info"); pos >= 0 {
		metaInfoContent = metaInfoContent[pos:]
	}

	reMetaInfoItem := `
(
\s*(--[-\w]+)
\s*:
\s*(
("(\\"|[^"])*")
|('(\\'|[^'])*')
|([^'";]+)
)
\s*;?
\s*
)
`
	reMetaInfoItem = strings.ReplaceAll(reMetaInfoItem, "\n", "")
	reMetaInfoBlock := `\bgitea-theme-meta-info\s*\{(` + reMetaInfoItem + `+)\}`
	re := regexp.MustCompile(reMetaInfoBlock)
	matchedMetaInfoBlock := re.FindAllStringSubmatch(metaInfoContent, -1)
	if len(matchedMetaInfoBlock) == 0 {
		return nil
	}
	re = regexp.MustCompile(strings.ReplaceAll(reMetaInfoItem, "\n", ""))
	matchedItems := re.FindAllStringSubmatch(matchedMetaInfoBlock[0][1], -1)
	m := map[string]string{}
	for _, item := range matchedItems {
		v := item[3]
		if after, ok := strings.CutPrefix(v, `"`); ok {
			v = strings.TrimSuffix(after, `"`)
			v = strings.ReplaceAll(v, `\"`, `"`)
		} else if after, ok := strings.CutPrefix(v, `'`); ok {
			v = strings.TrimSuffix(after, `'`)
			v = strings.ReplaceAll(v, `\'`, `'`)
		}
		m[item[2]] = v
	}
	return m
}

func defaultThemeMetaInfoByFileName(fileName string) *ThemeMetaInfo {
	themeInfo := &ThemeMetaInfo{
		FileName:     fileName,
		InternalName: strings.TrimSuffix(strings.TrimPrefix(fileName, fileNamePrefix), fileNameSuffix),
	}
	themeInfo.DisplayName = themeInfo.InternalName
	return themeInfo
}

func defaultThemeMetaInfoByInternalName(fileName string) *ThemeMetaInfo {
	return defaultThemeMetaInfoByFileName(fileNamePrefix + fileName + fileNameSuffix)
}

func parseThemeMetaInfo(fileName, cssContent string) *ThemeMetaInfo {
	themeInfo := defaultThemeMetaInfoByFileName(fileName)
	m := parseThemeMetaInfoToMap(cssContent)
	if m == nil {
		return themeInfo
	}
	themeInfo.DisplayName = m["--theme-display-name"]
	themeInfo.ColorblindType = m["--theme-colorblind-type"]
	themeInfo.ColorScheme = m["--theme-color-scheme"]
	return themeInfo
}

func initThemes() {
	availableThemes = nil
	defer func() {
		availableThemeMap = map[string]*ThemeMetaInfo{}
		for _, theme := range availableThemes {
			availableThemeMap[theme.InternalName] = theme
		}
		if availableThemeMap[setting.UI.DefaultTheme] == nil {
			setting.LogStartupProblem(1, log.ERROR, "Default theme %q is not available, please correct the '[ui].DEFAULT_THEME' setting in the config file", setting.UI.DefaultTheme)
		}
	}()
	cssFiles, err := public.AssetFS().ListFiles("/assets/css")
	if err != nil {
		log.Error("Failed to list themes: %v", err)
		availableThemes = []*ThemeMetaInfo{defaultThemeMetaInfoByInternalName(setting.UI.DefaultTheme)}
		return
	}
	var foundThemes []*ThemeMetaInfo
	for _, fileName := range cssFiles {
		if strings.HasPrefix(fileName, fileNamePrefix) && strings.HasSuffix(fileName, fileNameSuffix) {
			content, err := public.AssetFS().ReadFile("/assets/css/" + fileName)
			if err != nil {
				log.Error("Failed to read theme file %q: %v", fileName, err)
				continue
			}
			foundThemes = append(foundThemes, parseThemeMetaInfo(fileName, util.UnsafeBytesToString(content)))
		}
	}
	if len(setting.UI.Themes) > 0 {
		allowedThemes := container.SetOf(setting.UI.Themes...)
		for _, theme := range foundThemes {
			if allowedThemes.Contains(theme.InternalName) {
				availableThemes = append(availableThemes, theme)
			}
		}
	} else {
		availableThemes = foundThemes
	}
	sort.Slice(availableThemes, func(i, j int) bool {
		if availableThemes[i].InternalName == setting.UI.DefaultTheme {
			return true
		}
		if availableThemes[i].ColorblindType != availableThemes[j].ColorblindType {
			return availableThemes[i].ColorblindType < availableThemes[j].ColorblindType
		}
		return availableThemes[i].DisplayName < availableThemes[j].DisplayName
	})
	if len(availableThemes) == 0 {
		setting.LogStartupProblem(1, log.ERROR, "No theme candidate in asset files, but Gitea requires there should be at least one usable theme")
		availableThemes = []*ThemeMetaInfo{defaultThemeMetaInfoByInternalName(setting.UI.DefaultTheme)}
	}
}

func GetAvailableThemes() []*ThemeMetaInfo {
	themeOnce.Do(initThemes)
	return availableThemes
}

func GetThemeMetaInfo(internalName string) *ThemeMetaInfo {
	themeOnce.Do(initThemes)
	return availableThemeMap[internalName]
}

// GuaranteeGetThemeMetaInfo guarantees to return a non-nil ThemeMetaInfo,
// to simplify the caller's logic, especially for templates.
// There are already enough warnings messages if the default theme is not available.
func GuaranteeGetThemeMetaInfo(internalName string) *ThemeMetaInfo {
	info := GetThemeMetaInfo(internalName)
	if info == nil {
		info = GetThemeMetaInfo(setting.UI.DefaultTheme)
	}
	if info == nil {
		info = &ThemeMetaInfo{DisplayName: "unavailable", InternalName: "unavailable", FileName: "unavailable"}
	}
	return info
}
