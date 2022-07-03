// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// swagger:model
type UpsertProjectPayload struct {
	Title       string `json:"title" binding:"Required"`
	Description string `json:"body"`
	BoardType   uint8  `json:"board_type"`
}

type Project struct {
	Title       string `json:"title"`
	Description string `json:"body"`
	BoardType   uint8  `json:"board_type"`
}

type ProjectBoard struct {
	Title   string `json:"title"`
	Default bool   `json:"default"`
	Color   string `json:"color"`
	Sorting int8   `json:"sorting"`
}

// swagger:model
type UpsertProjectBoardPayload struct {
	Title   string `json:"title"`
	Default bool   `json:"default"`
	Color   string `json:"color"`
	Sorting int8   `json:"sorting"`
}
