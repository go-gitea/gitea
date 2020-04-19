package gitlab

import (
	"fmt"
)

// LicenseTemplate represents a license template.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/templates/licenses.html
type LicenseTemplate struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Nickname    string   `json:"nickname"`
	Featured    bool     `json:"featured"`
	HTMLURL     string   `json:"html_url"`
	SourceURL   string   `json:"source_url"`
	Description string   `json:"description"`
	Conditions  []string `json:"conditions"`
	Permissions []string `json:"permissions"`
	Limitations []string `json:"limitations"`
	Content     string   `json:"content"`
}

// LicenseTemplatesService handles communication with the license templates
// related methods of the GitLab API.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/templates/licenses.html
type LicenseTemplatesService struct {
	client *Client
}

// ListLicenseTemplatesOptions represents the available
// ListLicenseTemplates() options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/templates/licenses.html#list-license-templates
type ListLicenseTemplatesOptions struct {
	ListOptions
	Popular *bool `url:"popular,omitempty" json:"popular,omitempty"`
}

// ListLicenseTemplates get all license templates.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/templates/licenses.html#list-license-templates
func (s *LicenseTemplatesService) ListLicenseTemplates(opt *ListLicenseTemplatesOptions, options ...OptionFunc) ([]*LicenseTemplate, *Response, error) {
	req, err := s.client.NewRequest("GET", "templates/licenses", opt, options)
	if err != nil {
		return nil, nil, err
	}

	var lts []*LicenseTemplate
	resp, err := s.client.Do(req, &lts)
	if err != nil {
		return nil, resp, err
	}

	return lts, resp, err
}

// GetLicenseTemplateOptions represents the available
// GetLicenseTemplate() options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/templates/licenses.html#single-license-template
type GetLicenseTemplateOptions struct {
	Project  *string `url:"project,omitempty" json:"project,omitempty"`
	Fullname *string `url:"fullname,omitempty" json:"fullname,omitempty"`
}

// GetLicenseTemplate get a single license template. You can pass parameters
// to replace the license placeholder.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/templates/licenses.html#single-license-template
func (s *LicenseTemplatesService) GetLicenseTemplate(template string, opt *GetLicenseTemplateOptions, options ...OptionFunc) (*LicenseTemplate, *Response, error) {
	u := fmt.Sprintf("templates/licenses/%s", template)

	req, err := s.client.NewRequest("GET", u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	lt := new(LicenseTemplate)
	resp, err := s.client.Do(req, lt)
	if err != nil {
		return nil, resp, err
	}

	return lt, resp, err
}
