// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"fmt"
	"strings"

	"github.com/yuin/goldmark/ast"
	"gopkg.in/yaml.v3"
)

// RenderConfig represents rendering configuration for this file
type RenderConfig struct {
	Meta     string
	Icon     string
	TOC      bool
	Lang     string
	yamlNode *yaml.Node
}

// UnmarshalYAML implement yaml.v3 UnmarshalYAML
func (rc *RenderConfig) UnmarshalYAML(value *yaml.Node) error {
	if rc == nil {
		rc = &RenderConfig{
			Meta: "table",
			Icon: "table",
			Lang: "",
		}
	}
	rc.yamlNode = value

	type commonRenderConfig struct {
		TOC  bool   `yaml:"include_toc"`
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
			switch strings.TrimSpace(strings.ToLower(stringBasic.Gitea)) {
			case "none":
				rc.Meta = "none"
			case "table":
				rc.Meta = "table"
			default: // "details"
				rc.Meta = "details"
			}
		}
		return nil
	}

	type giteaControl struct {
		Meta *string `yaml:"meta"`
		Icon *string `yaml:"details_icon"`
		TOC  *bool   `yaml:"include_toc"`
		Lang *string `yaml:"lang"`
	}

	type complexGiteaConfig struct {
		Gitea *giteaControl `yaml:"gitea"`
	}
	var complex complexGiteaConfig
	if err := value.Decode(&complex); err != nil {
		return fmt.Errorf("unable to decode into complexRenderConfig %w", err)
	}

	if complex.Gitea == nil {
		return nil
	}

	if complex.Gitea.Meta != nil {
		switch strings.TrimSpace(strings.ToLower(*complex.Gitea.Meta)) {
		case "none":
			rc.Meta = "none"
		case "table":
			rc.Meta = "table"
		default: // "details"
			rc.Meta = "details"
		}
	}

	if complex.Gitea.Icon != nil {
		rc.Icon = strings.TrimSpace(strings.ToLower(*complex.Gitea.Icon))
	}

	if complex.Gitea.Lang != nil && *complex.Gitea.Lang != "" {
		rc.Lang = *complex.Gitea.Lang
	}

	if complex.Gitea.TOC != nil {
		rc.TOC = *complex.Gitea.TOC
	}

	return nil
}

func (rc *RenderConfig) toMetaNode() ast.Node {
	if rc.yamlNode == nil {
		return nil
	}
	switch rc.Meta {
	case "table":
		return nodeToTable(rc.yamlNode)
	case "details":
		return nodeToDetails(rc.yamlNode, rc.Icon)
	default:
		return nil
	}
}
