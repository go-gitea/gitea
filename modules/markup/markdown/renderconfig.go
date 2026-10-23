// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"fmt"
	"strings"

	"codenest.dev/modules/markup"

	"github.com/yuin/goldmark/ast"
	"go.yaml.in/yaml/v4"
)

// RenderConfig represents rendering configuration for this file
type RenderConfig struct {
	Meta     markup.RenderMetaMode
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
		CodeNest string `yaml:"gitea"`
	}

	var stringBasic controlStringRenderConfig

	if err := value.Decode(&stringBasic); err == nil {
		if stringBasic.CodeNest != "" {
			rc.Meta = renderMetaModeFromString(stringBasic.CodeNest)
		}
		return nil
	}

	type yamlRenderConfig struct {
		Meta *string `yaml:"meta"`
		Icon *string `yaml:"details_icon"` // deprecated, because there is no font icon, so no custom icon
		TOC  *string `yaml:"include_toc"`
		Lang *string `yaml:"lang"`
	}

	type yamlRenderConfigWrapper struct {
		CodeNest *yamlRenderConfig `yaml:"gitea"`
	}

	var cfg yamlRenderConfigWrapper
	if err := value.Decode(&cfg); err != nil {
		return fmt.Errorf("unable to decode into yamlRenderConfigWrapper %w", err)
	}

	if cfg.CodeNest == nil {
		return nil
	}

	if cfg.CodeNest.Meta != nil {
		rc.Meta = renderMetaModeFromString(*cfg.CodeNest.Meta)
	}

	if cfg.CodeNest.Lang != nil && *cfg.CodeNest.Lang != "" {
		rc.Lang = *cfg.CodeNest.Lang
	}

	if cfg.CodeNest.TOC != nil {
		rc.TOC = *cfg.CodeNest.TOC
	}

	return nil
}

func (rc *RenderConfig) toMetaNode(g *ASTTransformer) ast.Node {
	if rc.yamlNode == nil {
		return nil
	}
	switch rc.Meta {
	case markup.RenderMetaAsTable:
		return nodeToTable(rc.yamlNode)
	case markup.RenderMetaAsDetails:
		return nodeToDetails(g, rc.yamlNode)
	default:
		return nil
	}
}
