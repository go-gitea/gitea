// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"errors"
	"io"

	"gopkg.in/yaml.v2"
)

// Webhook is a custom webhook
type Webhook struct {
	ID   string   `yaml:"id"`
	HTTP string   `yaml:"http"`
	Exec []string `yaml:"exec"`
	Form []Form   `yaml:"form"`
}

// Form is a webhook form
type Form struct {
	ID       string `yaml:"id"`
	Label    string `yaml:"label"`
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
	Default  string `yaml:"default"`
}

func (w *Webhook) validate() error {
	if w.ID == "" {
		return errors.New("webhook id is required")
	}
	if (w.HTTP == "" && len(w.Exec) == 0) || (w.HTTP != "" && len(w.Exec) > 0) {
		return errors.New("webhook requires one of exec or http")
	}
	for _, form := range w.Form {
		if form.ID == "" {
			return errors.New("form id is required")
		}
		if form.Label == "" {
			return errors.New("form label is required")
		}
		if form.Type == "" {
			return errors.New("form type is required")
		}
	}
	return nil
}

// Parse parses a Webhooks from an io.Reader
func Parse(r io.Reader) (*Webhook, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var w Webhook
	if err := yaml.Unmarshal(b, &w); err != nil {
		return nil, err
	}

	if err := w.validate(); err != nil {
		return nil, err
	}

	return &w, nil
}
