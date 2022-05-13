// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

var Webhooks = make(map[string]*Webhook)

// Webhook is a custom webhook
type Webhook struct {
	ID    string   `yaml:"id"`
	Label string   `yaml:"label"`
	Docs  string   `yaml:"docs"`
	HTTP  string   `yaml:"http"`
	Exec  []string `yaml:"exec"`
	Form  []Form   `yaml:"form"`
	Path  string   `yaml:"-"`
}

// Image returns a custom webhook image if it exists, else the default image
// Image needs to be CLOSED
func (w *Webhook) Image() (io.ReadCloser, error) {
	img, err := os.Open(filepath.Join(w.Path, "image.png"))
	if err != nil {
		return nil, fmt.Errorf("could not open custom webhook image: %w", err)
	}

	return img, nil
}

// Form is a webhook form
type Form struct {
	ID       string `yaml:"id"`
	Label    string `yaml:"label"`
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
	Default  string `yaml:"default"`
	Pattern  string `yaml:"pattern"`
}

// InputType returns the HTML input type of a Form.Type
func (f Form) InputType() string {
	switch f.Type {
	case "text":
		return "text"
	case "secret":
		return "password"
	case "number":
		return "number"
	case "bool":
		return "checkbox"
	default:
		return "text"
	}
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
		switch form.Type {
		case "text", "secret", "bool", "number":
		default:
			return errors.New("form type is invalid; must be one of text, secret, bool, or number")
		}
	}
	return nil
}

// Parse parses a Webhook from an io.Reader
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

// Init initializes any custom webhooks found in path
func Init(path string) error {
	dir, err := os.ReadDir(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("could not read dir %q: %w", path, err)
	}

	for _, d := range dir {
		if !d.IsDir() {
			continue
		}

		hookPath := filepath.Join(path, d.Name())
		cfg, err := os.Open(filepath.Join(hookPath, "config.yml"))
		if err != nil {
			return fmt.Errorf("could not open custom webhook config: %w", err)
		}

		hook, err := Parse(cfg)
		if err != nil {
			return fmt.Errorf("could not parse custom webhook config: %w", err)
		}
		hook.Path = hookPath

		Webhooks[hook.ID] = hook

		if err := cfg.Close(); err != nil {
			return fmt.Errorf("could not close custom webhook config: %w", err)
		}
	}

	return nil
}
