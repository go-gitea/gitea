// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"strings"

	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

var FundingProviders []*api.FundingProvider

func loadBuiltinFundingProviders() {
	FundingProviders = append(FundingProviders, &api.FundingProvider{
		Name: "custom",
		Text: "%s",
		URL:  "%s",
		Icon: "img/svg/octicon-link.svg",
	})

	FundingProviders = append(FundingProviders, &api.FundingProvider{
		Name: "ko_fi",
		Text: "Ko-Fi/%s",
		URL:  "https://ko-fi.com/%s",
		Icon: "img/funding/ko_fi.svg",
	})
}

func loadCustomFundingProvidersFrom(rootCfg ConfigProvider) {
	for _, sec := range rootCfg.Section("funding").ChildSections() {
		name := strings.TrimPrefix(sec.Name(), "funding.")
		if name == "" {
			log.Warn("name is empty, funding " + sec.Name() + "ignored")
			continue
		}

		provider := new(api.FundingProvider)
		provider.Name = name
		provider.Text = sec.Key("Text").MustString("")
		provider.URL = sec.Key("URL").MustString("")
		provider.Icon = sec.Key("Icon").MustString("")

		FundingProviders = append(FundingProviders, provider)
	}
}

func GetFundingProviderByName(name string) *api.FundingProvider {
	for _, provider := range FundingProviders {
		if provider.Name == name {
			return provider
		}
	}

	return nil
}
