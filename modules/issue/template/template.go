// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package template

import "html/template"

// Field represents a field interface which could be a component in the issue create UI
// TODO: Do we need it?
type Field interface {
	Name() string
	Description() string
	Render() template.HTML
}

type CheckBox struct{}

type Markdown struct{}

type Input struct{}

type TextArea struct{}

type Dropdown struct{}
