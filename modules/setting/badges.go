// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"text/template"
)

// Badges settings
var Badges = struct {
	Enabled                      bool               `ini:"ENABLED"`
	GeneratorURLTemplate         string             `ini:"GENERATOR_URL_TEMPLATE"`
	GeneratorURLTemplateTemplate *template.Template `ini:"-"`
}{
	Enabled:              true,
	GeneratorURLTemplate: "https://img.shields.io/badge/{{.label}}-{{.text}}-{{.color}}",
}

func loadBadgesFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "badges", &Badges)

	Badges.GeneratorURLTemplateTemplate = template.Must(template.New("").Parse(Badges.GeneratorURLTemplate))
}
