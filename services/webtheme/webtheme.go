// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webtheme

import (
	"io/fs"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"gitea.dev/modules/container"
	"gitea.dev/modules/log"
	"gitea.dev/modules/public"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

type themeCollectionStruct struct {
	lastCheckTime    time.Time
	usingViteDevMode bool

	themeList []*ThemeMetaInfo
	themeMap  map[string]*ThemeMetaInfo
}

var themeCollection atomic.Pointer[themeCollectionStruct]

const (
	fileNamePrefix = "theme-"
	fileNameSuffix = ".css"

	// The built-in themes were renamed from the "gitea-" prefix to the "teabag-" one.
	// The old names are still stored in the user settings row and in the theme cookie of everyone
	// who picked a theme before the rename, and admins may still use them in "[ui].THEMES",
	// so they must keep resolving to the renamed themes instead of silently falling back to the default.
	legacyInternalNamePrefix = "gitea-"
	internalNamePrefix       = "teabag-"
)

// resolveInternalName maps a theme name written before the "gitea-*" to "teabag-*" rename
// to its current name. Names which are already current are returned unchanged.
func resolveInternalName(internalName string) string {
	if suffix, ok := strings.CutPrefix(internalName, legacyInternalNamePrefix); ok {
		return internalNamePrefix + suffix
	}
	return internalName
}

// resolveInternalNameSet expands a configured theme name list into a set which matches both
// the configured names and their post-rename equivalents.
func resolveInternalNameSet(internalNames []string) container.Set[string] {
	set := make(container.Set[string], len(internalNames))
	for _, internalName := range internalNames {
		set.Add(internalName)
		set.Add(resolveInternalName(internalName))
	}
	return set
}

func lookupThemeMetaInfo(themeMap map[string]*ThemeMetaInfo, internalName string) *ThemeMetaInfo {
	if info := themeMap[internalName]; info != nil {
		return info
	}
	// The exact name is tried first, so a custom theme still called "theme-gitea-*.css" keeps winning.
	return themeMap[resolveInternalName(internalName)]
}

type ThemeMetaInfo struct {
	FileName       string
	InternalName   string
	DisplayName    string
	ColorblindType string
	ColorScheme    string
}

func (info *ThemeMetaInfo) PublicAssetURI() string {
	return public.AssetURI("web_src/css/themes/theme-" + url.PathEscape(info.InternalName) + ".css")
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
		return "teabag-colorblind-redgreen"
	}
	if info.ColorblindType == "blue-yellow" {
		return "teabag-colorblind-blueyellow"
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
	if pos := strings.LastIndex(metaInfoContent, "teabag-theme-meta-info"); pos >= 0 {
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
	reMetaInfoBlock := `\bteabag-theme-meta-info\s*\{(` + reMetaInfoItem + `+)\}`
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
	internalName := strings.TrimSuffix(strings.TrimPrefix(fileName, fileNamePrefix), fileNameSuffix)
	// For built-in themes, the manifest knows the unhashed entry name (e.g. "theme-teabag-dark")
	// which lets us correctly strip the content hash without guessing.
	// Custom themes are not in the manifest and never have content hashes.
	if name := public.AssetNameFromHashedPath("css/" + fileName); name != "" {
		internalName = strings.TrimPrefix(name, fileNamePrefix)
	}
	themeInfo := &ThemeMetaInfo{
		FileName:     fileName,
		InternalName: internalName,
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

func collectThemeFiles(dirFS fs.FS, fsPath string) (themes []*ThemeMetaInfo, _ error) {
	files, err := fs.ReadDir(dirFS, fsPath)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		fileName := file.Name()
		if !strings.HasPrefix(fileName, fileNamePrefix) || !strings.HasSuffix(fileName, fileNameSuffix) {
			continue
		}
		content, err := fs.ReadFile(dirFS, path.Join(fsPath, file.Name()))
		if err != nil {
			log.Error("Failed to read theme file %q: %v", fileName, err)
			continue
		}
		themes = append(themes, parseThemeMetaInfo(fileName, util.UnsafeBytesToString(content)))
	}
	return themes, nil
}

func loadThemesFromAssets(isViteDevMode bool) (themeList []*ThemeMetaInfo, themeMap map[string]*ThemeMetaInfo) {
	var themeDir fs.FS
	var themePath string

	if isViteDevMode {
		// In vite dev mode, Vite serves themes directly from source files.
		themeDir, themePath = os.DirFS(setting.StaticRootPath), "web_src/css/themes"
	} else {
		// Without vite dev server, use built assets from AssetFS.
		themeDir, themePath = public.AssetFS(), "assets/css"
	}

	foundThemes, err := collectThemeFiles(themeDir, themePath)
	if err != nil {
		log.Error("Failed to load theme files: %v", err)
		return themeList, themeMap
	}

	defaultThemeName := resolveInternalName(setting.UI.DefaultTheme)

	themeList = foundThemes
	if len(setting.UI.Themes) > 0 {
		themeList = nil // only allow the themes specified in the setting
		allowedThemes := resolveInternalNameSet(setting.UI.Themes)
		for _, theme := range foundThemes {
			if allowedThemes.Contains(theme.InternalName) {
				themeList = append(themeList, theme)
			}
		}
	}

	sort.Slice(themeList, func(i, j int) bool {
		if themeList[i].InternalName == defaultThemeName {
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

func getAvailableThemes() *themeCollectionStruct {
	themes := themeCollection.Load()

	now := time.Now()
	if themes != nil && now.Sub(themes.lastCheckTime) < time.Second {
		return themes
	}

	isViteDevMode := public.IsViteDevMode()
	useLoadedThemes := themes != nil && (setting.IsProd || themes.usingViteDevMode == isViteDevMode)
	if useLoadedThemes && len(themes.themeList) > 0 {
		return themes
	}

	themeList, themeMap := loadThemesFromAssets(isViteDevMode)
	hasAvailableThemes := len(themeList) > 0
	if !hasAvailableThemes {
		defaultTheme := defaultThemeMetaInfoByInternalName(setting.UI.DefaultTheme)
		themeList = []*ThemeMetaInfo{defaultTheme}
		themeMap = map[string]*ThemeMetaInfo{resolveInternalName(setting.UI.DefaultTheme): defaultTheme}
	}

	if setting.IsProd {
		if !hasAvailableThemes {
			setting.LogStartupProblem(1, log.ERROR, "No theme candidate in asset files, but Gitea requires there should be at least one usable theme")
		}
		if lookupThemeMetaInfo(themeMap, setting.UI.DefaultTheme) == nil {
			setting.LogStartupProblem(1, log.ERROR, "Default theme %q is not available, please correct the '[ui].DEFAULT_THEME' setting in the config file", setting.UI.DefaultTheme)
		}
	}

	themes = &themeCollectionStruct{now, isViteDevMode, themeList, themeMap}
	themeCollection.Store(themes)
	return themes
}

func GetAvailableThemes() []*ThemeMetaInfo {
	return getAvailableThemes().themeList
}

func GetThemeMetaInfo(internalName string) *ThemeMetaInfo {
	return lookupThemeMetaInfo(getAvailableThemes().themeMap, internalName)
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
