// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package config

import (
	"fmt"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"gopkg.in/yaml.v3"
)

var renderConfigKey = parser.NewContextKey()

func GetRenderConfig(pc parser.Context) *RenderConfig {
	return pc.Get(renderConfigKey).(*RenderConfig)
}

func SetRenderConfig(pc parser.Context, rc *RenderConfig) {
	pc.Set(renderConfigKey, rc)
}

// RenderConfig represents rendering configuration for this file
type RenderConfig struct {
	Meta     string
	Icon     string
	TOC      bool
	Lang     string
	Math     *MathConfig
	yamlNode *yaml.Node
}

type MathConfig struct {
	InlineDollar  bool `yaml:"inline_dollar"`
	InlineLatex   bool `yaml:"inline_latex"`
	DisplayDollar bool `yaml:"display_dollar"`
	DisplayLatex  bool `yaml:"display_latex"`
}

// UnmarshalYAML implement yaml.v3 UnmarshalYAML
func (rc *RenderConfig) UnmarshalYAML(value *yaml.Node) error {
	rc.yamlNode = value

	basic := &yamlRenderConfig{}
	err := value.Decode(basic)
	if err != nil {
		return fmt.Errorf("failed to decode basic: %w", err)
	}

	if basic.Lang != "" {
		rc.Lang = basic.Lang
	}

	rc.TOC = basic.TOC

	if basic.Math != nil {
		rc.Math = basic.Math
	}

	if basic.Gitea != nil {
		if basic.Gitea.Meta != nil {
			rc.Meta = *basic.Gitea.Meta
		}
		if basic.Gitea.Icon != nil {
			rc.Icon = *basic.Gitea.Icon
		}
		if basic.Gitea.Lang != nil {
			rc.Lang = *basic.Gitea.Lang
		}
		if basic.Gitea.TOC != nil {
			rc.TOC = *basic.Gitea.TOC
		}
		if basic.Gitea.Math != nil {
			rc.Math = basic.Gitea.Math
		}
	}

	return nil
}

type yamlRenderConfig struct {
	TOC   bool        `yaml:"include_toc"`
	Lang  string      `yaml:"lang"`
	Math  *MathConfig `yaml:"math"`
	Gitea *yamlGitea  `yaml:"gitea"`
}

type yamlGitea struct {
	Meta *string
	Icon *string `yaml:"details_icon"`
	TOC  *bool   `yaml:"include_toc"`
	Lang *string
	Math *MathConfig
}

func (y *yamlGitea) UnmarshalYAML(node *yaml.Node) error {
	var controlString string
	if err := node.Decode(&controlString); err == nil {
		var meta string
		switch strings.TrimSpace(strings.ToLower(controlString)) {
		case "none":
			meta = "none"
		case "table":
			meta = "table"
		default: // "details"
			meta = "details"
		}
		y.Meta = &meta
		return nil
	}

	type yExactType yamlGitea
	yExact := (*yExactType)(y)
	if err := node.Decode(yExact); err != nil {
		return fmt.Errorf("unable to parse yamlGitea: %w", err)
	}

	return nil
}

func (m *MathConfig) UnmarshalYAML(node *yaml.Node) error {
	var controlBool bool
	if err := node.Decode(&controlBool); err == nil {
		m.InlineLatex = controlBool
		m.DisplayLatex = controlBool
		m.DisplayDollar = controlBool
		// Not InlineDollar
		m.InlineDollar = false
		return nil
	}

	var enableMathStrs []string
	if err := node.Decode(&enableMathStrs); err != nil {
		var enableMathStr string
		if err := node.Decode(&enableMathStr); err == nil {
			m.InlineLatex = false
			m.DisplayLatex = false
			m.DisplayDollar = false
			m.InlineDollar = false
			if enableMathStr == "" {
				enableMathStr = "true"
			}
			enableMathStrs = strings.Split(enableMathStr, ",")
		}
	}
	if enableMathStrs != nil {
		for _, value := range enableMathStrs {
			value = strings.TrimSpace(strings.ToLower(value))
			set := true
			if value != "" && value[0] == '!' {
				set = false
				value = value[1:]
			}
			switch strings.TrimSpace(strings.ToLower(value)) {
			case "none":
				fallthrough
			case "false":
				m.InlineLatex = !set
				m.DisplayLatex = !set
				m.DisplayDollar = !set
				m.InlineDollar = !set
			case "all":
				m.InlineLatex = set
				m.DisplayLatex = set
				m.DisplayDollar = set
				m.InlineDollar = set
				return nil
			case "inline_dollar":
				m.InlineDollar = set
			case "inline_latex":
				m.InlineLatex = set
			case "display_dollar":
				m.DisplayDollar = set
			case "display_latex":
				m.DisplayLatex = set
			case "true":
				m.InlineLatex = set
				m.DisplayLatex = set
				m.DisplayDollar = set
			}
		}
		return nil
	}

	type mExactType MathConfig
	mExact := (*mExactType)(m)
	if err := node.Decode(mExact); err != nil {
		return fmt.Errorf("unable to parse MathConfig: %w", err)
	}
	return nil
}

func (rc *RenderConfig) ToMetaNode() ast.Node {
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
