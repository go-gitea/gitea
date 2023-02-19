// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

var (
	// Other settings
	ShowFooterBranding         bool
	ShowFooterVersion          bool
	ShowFooterTemplateLoadTime bool
	EnableFeed                 bool
	EnableSitemap              bool
)

func loadOtherFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("other")
	ShowFooterBranding = sec.Key("SHOW_FOOTER_BRANDING").MustBool(false)
	ShowFooterVersion = sec.Key("SHOW_FOOTER_VERSION").MustBool(true)
	ShowFooterTemplateLoadTime = sec.Key("SHOW_FOOTER_TEMPLATE_LOAD_TIME").MustBool(true)
	EnableSitemap = sec.Key("ENABLE_SITEMAP").MustBool(true)
	EnableFeed = sec.Key("ENABLE_FEED").MustBool(true)
}
