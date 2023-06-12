// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/markup"

	"github.com/yuin/goldmark/ast"
	"gopkg.in/yaml.v3"
)

// RenderConfig represents rendering configuration for this file
type RenderConfig struct {
	Meta     markup.RenderMetaMode
	Icon     string
	TOC      string // "false": hide,  "side"/empty: in sidebar,  "main"/"true": in main view
	Lang     string
	yamlNode *yaml.Node

	// Used internally.  Cannot be controlled by frontmatter.
	metaLength int
}

func renderMetaModeFromString(s string) markup.RenderMetaMode {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "none":
		return markup.RenderMetaAsNone
	case "table":
		return markup.RenderMetaAsTable
	default: // "details"
		return markup.RenderMetaAsDetails
	}
}

// UnmarshalYAML implement yaml.v3 UnmarshalYAML
func (rc *RenderConfig) UnmarshalYAML(value *yaml.Node) error {
	if rc == nil {
		return nil
	}

	rc.yamlNode = value

	type commonRenderConfig struct {
		TOC  string `yaml:"include_toc"`
		Lang string `yaml:"lang"`
	}
	var basic commonRenderConfig
	if err := value.Decode(&basic); err != nil {
		return fmt.Errorf("unable to decode into commonRenderConfig %w", err)
	}

	if basic.Lang != "" {
		rc.Lang = basic.Lang
	}

	rc.TOC = basic.TOC

	type controlStringRenderConfig struct {
		Gitea string `yaml:"gitea"`
	}

	var stringBasic controlStringRenderConfig

	if err := value.Decode(&stringBasic); err == nil {
		if stringBasic.Gitea != "" {
			rc.Meta = renderMetaModeFromString(stringBasic.Gitea)
		}
		return nil
	}

	type yamlRenderConfig struct {
		Meta *string `yaml:"meta"`
		Icon *string `yaml:"details_icon"`
		TOC  *string `yaml:"include_toc"`
		Lang *string `yaml:"lang"`
	}

	type yamlRenderConfigWrapper struct {
		Gitea *yamlRenderConfig `yaml:"gitea"`
	}

	var cfg yamlRenderConfigWrapper
	if err := value.Decode(&cfg); err != nil {
		return fmt.Errorf("unable to decode into yamlRenderConfigWrapper %w", err)
	}

	if cfg.Gitea == nil {
		return nil
	}

	if cfg.Gitea.Meta != nil {
		rc.Meta = renderMetaModeFromString(*cfg.Gitea.Meta)
	}

	if cfg.Gitea.Icon != nil {
		rc.Icon = strings.TrimSpace(strings.ToLower(*cfg.Gitea.Icon))
	}

	if cfg.Gitea.Lang != nil && *cfg.Gitea.Lang != "" {
		rc.Lang = *cfg.Gitea.Lang
	}

	if cfg.Gitea.TOC != nil {
		rc.TOC = *cfg.Gitea.TOC
	}

	return nil
}

func (rc *RenderConfig) toMetaNode() ast.Node {
	if rc.yamlNode == nil {
		return nil
	}
	switch rc.Meta {
	case markup.RenderMetaAsTable:
		return nodeToTable(rc.yamlNode)
	case markup.RenderMetaAsDetails:
		return nodeToDetails(rc.yamlNode, rc.Icon)
	default:
		return nil
	}
}
