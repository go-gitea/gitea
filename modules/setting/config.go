// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting/config"
)

type PictureStruct struct {
	DisableGravatar       *config.Option[bool]
	EnableFederatedAvatar *config.Option[bool]
}

type OpenWithEditorApp struct {
	DisplayName string
	OpenURL     string
}

type OpenWithEditorAppsType []OpenWithEditorApp

// ToTextareaString is only used in templates, for help prompt only
// TODO: OPEN-WITH-EDITOR-APP-JSON: Because there is no "rich UI", a plain text editor is used to manage the list of apps
// Maybe we can use some better formats like Yaml in the future, then a simple textarea can manage the config clearly
func (t OpenWithEditorAppsType) ToTextareaString() string {
	var ret strings.Builder
	for _, app := range t {
		ret.WriteString(app.DisplayName + " = " + app.OpenURL + "\n")
	}
	return ret.String()
}

func openWithEditorAppsDefaultValue() OpenWithEditorAppsType {
	return OpenWithEditorAppsType{
		{
			DisplayName: "VS Code",
			OpenURL:     "vscode://vscode.git/clone?url={url}",
		},
		{
			DisplayName: "VSCodium",
			OpenURL:     "vscodium://vscode.git/clone?url={url}",
		},
		{
			DisplayName: "Intellij IDEA",
			OpenURL:     "jetbrains://idea/checkout/git?idea.required.plugins.id=Git4Idea&checkout.repo={url}",
		},
	}
}

type RepositoryStruct struct {
	OpenWithEditorApps *config.Option[OpenWithEditorAppsType]
	GitGuideRemoteName *config.Option[string]
}

// InstanceBannerType fields are directly used in templates, do remember to update the template if you change the fields
type InstanceBannerType struct {
	DisplayEnabled bool
	ContentMessage string
	StartTimeUnix  int64
	EndTimeUnix    int64
}

func (n InstanceBannerType) ShouldDisplay() bool {
	if !n.DisplayEnabled || n.ContentMessage == "" {
		return false
	}
	now := time.Now().Unix()
	if n.StartTimeUnix > 0 && now < n.StartTimeUnix {
		return false
	}
	if n.EndTimeUnix > 0 && now > n.EndTimeUnix {
		return false
	}
	return true
}

type WebUIStruct struct {
	InstanceBanner *config.Option[InstanceBannerType]
}

type ConfigStruct struct {
	Picture    *PictureStruct
	Repository *RepositoryStruct
	WebUI      *WebUIStruct
}

var (
	defaultConfig     *ConfigStruct
	defaultConfigOnce sync.Once
)

func initDefaultConfig() {
	config.SetCfgSecKeyGetter(&cfgSecKeyGetter{})
	defaultConfig = &ConfigStruct{
		Picture: &PictureStruct{
			DisableGravatar:       config.NewOption[bool]("picture.disable_gravatar").WithFileConfig(config.CfgSecKey{Sec: "picture", Key: "DISABLE_GRAVATAR"}),
			EnableFederatedAvatar: config.NewOption[bool]("picture.enable_federated_avatar").WithFileConfig(config.CfgSecKey{Sec: "picture", Key: "ENABLE_FEDERATED_AVATAR"}),
		},
		Repository: &RepositoryStruct{
			OpenWithEditorApps: config.NewOption[OpenWithEditorAppsType]("repository.open-with.editor-apps").WithEmptyAsDefault().WithDefaultFunc(openWithEditorAppsDefaultValue),
			GitGuideRemoteName: config.NewOption[string]("repository.git-guide-remote-name").WithEmptyAsDefault().WithDefaultSimple("origin"),
		},
		WebUI: &WebUIStruct{
			InstanceBanner: config.NewOption[InstanceBannerType]("web_ui.instance_banner"),
		},
	}
}

func Config() *ConfigStruct {
	defaultConfigOnce.Do(initDefaultConfig)
	return defaultConfig
}

type cfgSecKeyGetter struct{}

func (c cfgSecKeyGetter) GetValue(sec, key string) (v string, has bool) {
	if key == "" {
		return "", false
	}
	cfgSec, err := CfgProvider.GetSection(sec)
	if err != nil {
		log.Error("Unable to get config section: %q", sec)
		return "", false
	}
	cfgKey := ConfigSectionKey(cfgSec, key)
	if cfgKey == nil {
		return "", false
	}
	return cfgKey.Value(), true
}
