// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"context"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting/config"
)

type PictureStruct struct {
	DisableGravatar       *config.Value[bool]
	EnableFederatedAvatar *config.Value[bool]
}

type OpenWithEditorApp struct {
	DisplayName string
	OpenURL     string
}

type OpenWithEditorAppsType []OpenWithEditorApp

func (t OpenWithEditorAppsType) ToTextareaString() string {
	var ret strings.Builder
	for _, app := range t {
		ret.WriteString(app.DisplayName + " = " + app.OpenURL + "\n")
	}
	return ret.String()
}

func DefaultOpenWithEditorApps() OpenWithEditorAppsType {
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
	OpenWithEditorApps *config.Value[OpenWithEditorAppsType]
	GitGuideRemoteName *config.Value[string]
}

const (
	InstanceNoticeLevelInfo    = "info"
	InstanceNoticeLevelSuccess = "success"
	InstanceNoticeLevelWarning = "warning"
	InstanceNoticeLevelDanger  = "danger"
)

type InstanceNotice struct {
	Enabled bool
	Message string
	Level   string

	StartTime int64
	EndTime   int64
}

func DefaultInstanceNotice() InstanceNotice {
	return InstanceNotice{
		Level: InstanceNoticeLevelInfo,
	}
}

func IsValidInstanceNoticeLevel(level string) bool {
	switch level {
	case InstanceNoticeLevelInfo, InstanceNoticeLevelSuccess, InstanceNoticeLevelWarning, InstanceNoticeLevelDanger:
		return true
	default:
		return false
	}
}

func (n *InstanceNotice) Normalize() {
	if !IsValidInstanceNoticeLevel(n.Level) {
		n.Level = InstanceNoticeLevelInfo
	}
}

func (n *InstanceNotice) IsActive(now int64) bool {
	if !n.Enabled || n.Message == "" {
		return false
	}
	if n.StartTime > 0 && now < n.StartTime {
		return false
	}
	if n.EndTime > 0 && now > n.EndTime {
		return false
	}
	return true
}

func GetInstanceNotice(ctx context.Context) InstanceNotice {
	notice := Config().InstanceNotice.Banner.Value(ctx)
	notice.Normalize()
	return notice
}

type InstanceNoticeStruct struct {
	Banner *config.Value[InstanceNotice]
}

type ConfigStruct struct {
	Picture        *PictureStruct
	Repository     *RepositoryStruct
	InstanceNotice *InstanceNoticeStruct
}

var (
	defaultConfig     *ConfigStruct
	defaultConfigOnce sync.Once
)

func initDefaultConfig() {
	config.SetCfgSecKeyGetter(&cfgSecKeyGetter{})
	defaultConfig = &ConfigStruct{
		Picture: &PictureStruct{
			DisableGravatar:       config.ValueJSON[bool]("picture.disable_gravatar").WithFileConfig(config.CfgSecKey{Sec: "picture", Key: "DISABLE_GRAVATAR"}),
			EnableFederatedAvatar: config.ValueJSON[bool]("picture.enable_federated_avatar").WithFileConfig(config.CfgSecKey{Sec: "picture", Key: "ENABLE_FEDERATED_AVATAR"}),
		},
		Repository: &RepositoryStruct{
			OpenWithEditorApps: config.ValueJSON[OpenWithEditorAppsType]("repository.open-with.editor-apps"),
			GitGuideRemoteName: config.ValueJSON[string]("repository.git-guide-remote-name").WithDefault("origin"),
		},
		InstanceNotice: &InstanceNoticeStruct{
			Banner: config.ValueJSON[InstanceNotice]("instance.notice").WithDefault(DefaultInstanceNotice()),
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
