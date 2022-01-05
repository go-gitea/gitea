// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// CreateRepoFromTemplateOption options when creating repository using a template
type CreateRepoFromTemplateOption struct {
	// Owner is the organization or person who will own the new repository
	Owner string `json:"owner"`
	// Name of the repository to create
	Name string `json:"name"`
	// Description of the repository to create
	Description string `json:"description"`
	// Private is whether the repository is private
	Private bool `json:"private"`
	// GitContent include git content of default branch in template repo
	GitContent bool `json:"git_content"`
	// Topics include topics of template repo
	Topics bool `json:"topics"`
	// GitHooks include git hooks of template repo
	GitHooks bool `json:"git_hooks"`
	// Webhooks include webhooks of template repo
	Webhooks bool `json:"webhooks"`
	// Avatar include avatar of the template repo
	Avatar bool `json:"avatar"`
	// Labels include labels of template repo
	Labels bool `json:"labels"`
}

// Validate validates CreateRepoFromTemplateOption
func (opt CreateRepoFromTemplateOption) Validate() error {
	if len(opt.Owner) == 0 {
		return fmt.Errorf("field Owner is required")
	}
	if len(opt.Name) == 0 {
		return fmt.Errorf("field Name is required")
	}
	return nil
}

// CreateRepoFromTemplate create a repository using a template
func (c *Client) CreateRepoFromTemplate(templateOwner, templateRepo string, opt CreateRepoFromTemplateOption) (*Repository, *Response, error) {
	if err := escapeValidatePathSegments(&templateOwner, &templateRepo); err != nil {
		return nil, nil, err
	}

	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}

	repo := new(Repository)
	resp, err := c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/generate", templateOwner, templateRepo), jsonHeader, bytes.NewReader(body), &repo)
	return repo, resp, err
}
