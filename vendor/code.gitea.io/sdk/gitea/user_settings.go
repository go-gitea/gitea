// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
)

// UserSettings represents user settings
type UserSettings struct {
	FullName      string `json:"full_name"`
	Website       string `json:"website"`
	Description   string `json:"description"`
	Location      string `json:"location"`
	Language      string `json:"language"`
	Theme         string `json:"theme"`
	DiffViewStyle string `json:"diff_view_style"`
	// Privacy
	HideEmail    bool `json:"hide_email"`
	HideActivity bool `json:"hide_activity"`
}

// UserSettingsOptions represents options to change user settings
type UserSettingsOptions struct {
	FullName      *string `json:"full_name,omitempty"`
	Website       *string `json:"website,omitempty"`
	Description   *string `json:"description,omitempty"`
	Location      *string `json:"location,omitempty"`
	Language      *string `json:"language,omitempty"`
	Theme         *string `json:"theme,omitempty"`
	DiffViewStyle *string `json:"diff_view_style,omitempty"`
	// Privacy
	HideEmail    *bool `json:"hide_email,omitempty"`
	HideActivity *bool `json:"hide_activity,omitempty"`
}

// GetUserSettings returns user settings
func (c *Client) GetUserSettings() (*UserSettings, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_15_0); err != nil {
		return nil, nil, err
	}
	userConfig := new(UserSettings)
	resp, err := c.getParsedResponse("GET", "/user/settings", nil, nil, userConfig)
	return userConfig, resp, err
}

// UpdateUserSettings returns user settings
func (c *Client) UpdateUserSettings(opt UserSettingsOptions) (*UserSettings, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_15_0); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	userConfig := new(UserSettings)
	resp, err := c.getParsedResponse("PATCH", "/user/settings", jsonHeader, bytes.NewReader(body), userConfig)
	return userConfig, resp, err
}
