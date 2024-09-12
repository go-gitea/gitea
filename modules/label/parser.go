// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package label

import (
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/options"

	"gopkg.in/yaml.v3"
)

type labelFile struct {
	Labels []*Label `yaml:"labels"`
}

// ErrTemplateLoad represents a "ErrTemplateLoad" kind of error.
type ErrTemplateLoad struct {
	TemplateFile  string
	OriginalError error
}

// IsErrTemplateLoad checks if an error is a ErrTemplateLoad.
func IsErrTemplateLoad(err error) bool {
	_, ok := err.(ErrTemplateLoad)
	return ok
}

func (err ErrTemplateLoad) Error() string {
	return fmt.Sprintf("failed to load label template file %q: %v", err.TemplateFile, err.OriginalError)
}

// LoadTemplateFile loads the label template file by given file name, returns a slice of Label structs.
func LoadTemplateFile(fileName string) ([]*Label, error) {
	data, err := options.Labels(fileName)
	if err != nil {
		return nil, ErrTemplateLoad{fileName, fmt.Errorf("LoadTemplateFile: %w", err)}
	}

	if strings.HasSuffix(fileName, ".yaml") || strings.HasSuffix(fileName, ".yml") {
		return parseYamlFormat(fileName, data)
	}
	return parseLegacyFormat(fileName, data)
}

func parseYamlFormat(fileName string, data []byte) ([]*Label, error) {
	lf := &labelFile{}

	if err := yaml.Unmarshal(data, lf); err != nil {
		return nil, err
	}

	// Validate label data and fix colors
	for _, l := range lf.Labels {
		l.Color = strings.TrimSpace(l.Color)
		if len(l.Name) == 0 || len(l.Color) == 0 {
			return nil, ErrTemplateLoad{fileName, errors.New("label name and color are required fields")}
		}
		color, err := NormalizeColor(l.Color)
		if err != nil {
			return nil, ErrTemplateLoad{fileName, fmt.Errorf("bad HTML color code '%s' in label: %s", l.Color, l.Name)}
		}
		l.Color = color
	}

	return lf.Labels, nil
}

func parseLegacyFormat(fileName string, data []byte) ([]*Label, error) {
	lines := strings.Split(string(data), "\n")
	list := make([]*Label, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}

		parts, description, _ := strings.Cut(line, ";")

		color, labelName, ok := strings.Cut(parts, " ")
		if !ok {
			return nil, ErrTemplateLoad{fileName, fmt.Errorf("line is malformed: %s", line)}
		}

		color, err := NormalizeColor(color)
		if err != nil {
			return nil, ErrTemplateLoad{fileName, fmt.Errorf("bad HTML color code '%s' in line: %s", color, line)}
		}

		list = append(list, &Label{
			Name:        strings.TrimSpace(labelName),
			Color:       color,
			Description: strings.TrimSpace(description),
		})
	}

	return list, nil
}

// LoadTemplateDescription loads the labels from a template file, returns a description string by joining each Label.Name with comma
func LoadTemplateDescription(fileName string) (string, error) {
	var buf strings.Builder
	list, err := LoadTemplateFile(fileName)
	if err != nil {
		return "", err
	}

	for i := 0; i < len(list); i++ {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(list[i].Name)
	}
	return buf.String(), nil
}
