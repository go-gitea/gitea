package gitlab

// ValidateService handles communication with the validation related methods of
// the GitLab API.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/lint.html
type ValidateService struct {
	client *Client
}

// LintResult represents the linting results.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/lint.html
type LintResult struct {
	Status string   `json:"status"`
	Errors []string `json:"errors"`
}

// Lint validates .gitlab-ci.yml content.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/lint.html
func (s *ValidateService) Lint(content string, options ...OptionFunc) (*LintResult, *Response, error) {
	var opts struct {
		Content string `url:"content,omitempty" json:"content,omitempty"`
	}
	opts.Content = content

	req, err := s.client.NewRequest("POST", "ci/lint", &opts, options)
	if err != nil {
		return nil, nil, err
	}

	l := new(LintResult)
	resp, err := s.client.Do(req, l)
	if err != nil {
		return nil, resp, err
	}

	return l, resp, nil
}
