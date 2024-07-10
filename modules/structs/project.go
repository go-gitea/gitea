// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// Project represents a project
type Project struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	TemplateType uint8  `json:"template_type"`
	CardType     uint8  `json:"card_type"`
}

type CreateProjectOption struct {
	Title        string `json:"title" binding:"Required;MaxSize(100)"`
	Content      string `json:"content"`
	TemplateType uint8  `json:"template_type"`
	CardType     uint8  `json:"card_type"`
}
