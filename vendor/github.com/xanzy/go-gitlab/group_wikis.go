//
// Copyright 2021, Markus Lackner
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gitlab

import (
	"fmt"
	"net/http"
	"net/url"
)

// GroupWikisService handles communication with the group wikis related methods of
// the Gitlab API.
//
// GitLab API docs: https://docs.gitlab.com/ee/api/group_wikis.html
type GroupWikisService struct {
	client *Client
}

// GroupWiki represents a GitLab groups wiki.
//
// GitLab API docs: https://docs.gitlab.com/ee/api/group_wikis.html
type GroupWiki struct {
	Content string          `json:"content"`
	Format  WikiFormatValue `json:"format"`
	Slug    string          `json:"slug"`
	Title   string          `json:"title"`
}

func (w GroupWiki) String() string {
	return Stringify(w)
}

// ListGroupWikisOptions represents the available ListGroupWikis options.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/group_wikis.html#list-wiki-pages
type ListGroupWikisOptions struct {
	WithContent *bool `url:"with_content,omitempty" json:"with_content,omitempty"`
}

// ListGroupWikis lists all pages of the wiki of the given group id.
// When with_content is set, it also returns the content of the pages.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/group_wikis.html#list-wiki-pages
func (s *GroupWikisService) ListGroupWikis(gid interface{}, opt *ListGroupWikisOptions, options ...RequestOptionFunc) ([]*GroupWiki, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/wikis", pathEscape(group))

	req, err := s.client.NewRequest(http.MethodGet, u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	var gws []*GroupWiki
	resp, err := s.client.Do(req, &gws)
	if err != nil {
		return nil, resp, err
	}

	return gws, resp, err
}

// GetGroupWikiPage gets a wiki page for a given group.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/group_wikis.html#get-a-wiki-page
func (s *GroupWikisService) GetGroupWikiPage(gid interface{}, slug string, options ...RequestOptionFunc) (*GroupWiki, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/wikis/%s", pathEscape(group), url.PathEscape(slug))

	req, err := s.client.NewRequest(http.MethodGet, u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	gw := new(GroupWiki)
	resp, err := s.client.Do(req, gw)
	if err != nil {
		return nil, resp, err
	}

	return gw, resp, err
}

// CreateGroupWikiPageOptions represents options to CreateGroupWikiPage.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/group_wikis.html#create-a-new-wiki-page
type CreateGroupWikiPageOptions struct {
	Content *string          `url:"content,omitempty" json:"content,omitempty"`
	Title   *string          `url:"title,omitempty" json:"title,omitempty"`
	Format  *WikiFormatValue `url:"format,omitempty" json:"format,omitempty"`
}

// CreateGroupWikiPage creates a new wiki page for the given group with
// the given title, slug, and content.
//
// GitLab API docs:
// https://docs.gitlab.com/13.8/ee/api/group_wikis.html#create-a-new-wiki-page
func (s *GroupWikisService) CreateGroupWikiPage(gid interface{}, opt *CreateGroupWikiPageOptions, options ...RequestOptionFunc) (*GroupWiki, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/wikis", pathEscape(group))

	req, err := s.client.NewRequest(http.MethodPost, u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	w := new(GroupWiki)
	resp, err := s.client.Do(req, w)
	if err != nil {
		return nil, resp, err
	}

	return w, resp, err
}

// EditGroupWikiPageOptions represents options to EditGroupWikiPage.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/group_wikis.html#edit-an-existing-wiki-page
type EditGroupWikiPageOptions struct {
	Content *string          `url:"content,omitempty" json:"content,omitempty"`
	Title   *string          `url:"title,omitempty" json:"title,omitempty"`
	Format  *WikiFormatValue `url:"format,omitempty" json:"format,omitempty"`
}

// EditGroupWikiPage Updates an existing wiki page. At least one parameter is
// required to update the wiki page.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/group_wikis.html#edit-an-existing-wiki-page
func (s *GroupWikisService) EditGroupWikiPage(gid interface{}, slug string, opt *EditGroupWikiPageOptions, options ...RequestOptionFunc) (*GroupWiki, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/wikis/%s", pathEscape(group), url.PathEscape(slug))

	req, err := s.client.NewRequest(http.MethodPut, u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	w := new(GroupWiki)
	resp, err := s.client.Do(req, w)
	if err != nil {
		return nil, resp, err
	}

	return w, resp, err
}

// DeleteGroupWikiPage deletes a wiki page with a given slug.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/group_wikis.html#delete-a-wiki-page
func (s *GroupWikisService) DeleteGroupWikiPage(gid interface{}, slug string, options ...RequestOptionFunc) (*Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("groups/%s/wikis/%s", pathEscape(group), url.PathEscape(slug))

	req, err := s.client.NewRequest(http.MethodDelete, u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
