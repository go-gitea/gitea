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

type themeCollection struct {
	themeList []*ThemeMetaInfo
	themeMap  map[string]*ThemeMetaInfo
}

var (
	themeMu         sync.RWMutex
	availableThemes *themeCollection
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

func loadThemesFromAssets() (themeList []*ThemeMetaInfo, themeMap map[string]*ThemeMetaInfo) {
	cssFiles, err := public.AssetFS().ListFiles("assets/css")
	if err != nil {
		log.Error("Failed to list themes: %v", err)
		return nil, nil
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

	themeList = foundThemes
	if len(setting.UI.Themes) > 0 {
		themeList = nil // only allow the themes specified in the setting
		allowedThemes := container.SetOf(setting.UI.Themes...)
		for _, theme := range foundThemes {
			if allowedThemes.Contains(theme.InternalName) {
				themeList = append(themeList, theme)
			}
		}
	}

	sort.Slice(themeList, func(i, j int) bool {
		if themeList[i].InternalName == setting.UI.DefaultTheme {
			return true
		}
		if themeList[i].ColorblindType != themeList[j].ColorblindType {
			return themeList[i].ColorblindType < themeList[j].ColorblindType
		}
		return themeList[i].DisplayName < themeList[j].DisplayName
	})

	themeMap = map[string]*ThemeMetaInfo{}
	for _, theme := range themeList {
		themeMap[theme.InternalName] = theme
	}
	return themeList, themeMap
}

func getAvailableThemes() (themeList []*ThemeMetaInfo, themeMap map[string]*ThemeMetaInfo) {
	themeMu.RLock()
	if availableThemes != nil {
		themeList, themeMap = availableThemes.themeList, availableThemes.themeMap
	}
	themeMu.RUnlock()
	if len(themeList) != 0 {
		return themeList, themeMap
	}

	themeMu.Lock()
	defer themeMu.Unlock()
	// no need to double-check "availableThemes.themeList" since the loading isn't really slow, to keep code simple
	themeList, themeMap = loadThemesFromAssets()
	hasAvailableThemes := len(themeList) > 0
	if !hasAvailableThemes {
		defaultTheme := defaultThemeMetaInfoByInternalName(setting.UI.DefaultTheme)
		themeList = []*ThemeMetaInfo{defaultTheme}
		themeMap = map[string]*ThemeMetaInfo{setting.UI.DefaultTheme: defaultTheme}
	}

	if setting.IsProd {
		if !hasAvailableThemes {
			setting.LogStartupProblem(1, log.ERROR, "No theme candidate in asset files, but Gitea requires there should be at least one usable theme")
		}
		if themeMap[setting.UI.DefaultTheme] == nil {
			setting.LogStartupProblem(1, log.ERROR, "Default theme %q is not available, please correct the '[ui].DEFAULT_THEME' setting in the config file", setting.UI.DefaultTheme)
		}
		availableThemes = &themeCollection{themeList, themeMap}
		return themeList, themeMap
	}

	// In dev mode, only store the loaded themes if the list is not empty, in case the frontend is still being built.
	// TBH, there still could be a data-race that the themes are only partially built then the list is incomplete for first time loading.
	// Such edge case can be handled by checking whether the loaded themes are the same in a period or there is a flag file, but it is an over-kill, so, no.
	if hasAvailableThemes {
		availableThemes = &themeCollection{themeList, themeMap}
	}
	return themeList, themeMap
}

func GetAvailableThemes() []*ThemeMetaInfo {
	themes, _ := getAvailableThemes()
	return themes
}

func GetThemeMetaInfo(internalName string) *ThemeMetaInfo {
	_, themeMap := getAvailableThemes()
	return themeMap[internalName]
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
