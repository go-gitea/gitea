// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markdown

import (
	"strings"

	"code.gitea.io/gitea/modules/log"
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

	type basicRenderConfig struct {
		Gitea *yaml.Node `yaml:"gitea"`
		TOC   bool       `yaml:"include_toc"`
		Lang  string     `yaml:"lang"`
	}

	var basic basicRenderConfig

	err := value.Decode(&basic)
	if err != nil {
		return err
	}

	if basic.Lang != "" {
		rc.Lang = basic.Lang
	}

	rc.TOC = basic.TOC
	if basic.Gitea == nil {
		return nil
	}

	var control *string
	if err := basic.Gitea.Decode(&control); err == nil && control != nil {
		log.Info("control %v", control)
		switch strings.TrimSpace(strings.ToLower(*control)) {
		case "none":
			rc.Meta = "none"
		case "table":
			rc.Meta = "table"
		default: // "details"
			rc.Meta = "details"
		}
		return nil
	}

	type giteaControl struct {
		Meta string     `yaml:"meta"`
		Icon string     `yaml:"details_icon"`
		TOC  *yaml.Node `yaml:"include_toc"`
		Lang string     `yaml:"lang"`
	}

	var controlStruct *giteaControl
	if err := basic.Gitea.Decode(controlStruct); err != nil || controlStruct == nil {
		return err
	}

	switch strings.TrimSpace(strings.ToLower(controlStruct.Meta)) {
	case "none":
		rc.Meta = "none"
	case "table":
		rc.Meta = "table"
	default: // "details"
		rc.Meta = "details"
	}

	rc.Icon = strings.TrimSpace(strings.ToLower(controlStruct.Icon))

	if controlStruct.Lang != "" {
		rc.Lang = controlStruct.Lang
	}

	var toc bool
	if err := controlStruct.TOC.Decode(&toc); err == nil {
		rc.TOC = toc
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
