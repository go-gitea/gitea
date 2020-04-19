//
// Copyright 2018, Patrick Webster
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
//

package gitlab

// LicenseService handles communication with the license
// related methods of the GitLab API.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/license.html
type LicenseService struct {
	client *Client
}

// License represents a GitLab license.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/license.html
type License struct {
	StartsAt  *ISOTime `json:"starts_at"`
	ExpiresAt *ISOTime `json:"expires_at"`
	Licensee  struct {
		Name    string `json:"Name"`
		Company string `json:"Company"`
		Email   string `json:"Email"`
	} `json:"licensee"`
	UserLimit   int `json:"user_limit"`
	ActiveUsers int `json:"active_users"`
	AddOns      struct {
		GitLabFileLocks int `json:"GitLabFileLocks"`
	} `json:"add_ons"`
}

func (l License) String() string {
	return Stringify(l)
}

// GetLicense retrieves information about the current license.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/license.html#retrieve-information-about-the-current-license
func (s *LicenseService) GetLicense() (*License, *Response, error) {
	req, err := s.client.NewRequest("GET", "license", nil, nil)
	if err != nil {
		return nil, nil, err
	}

	l := new(License)
	resp, err := s.client.Do(req, l)
	if err != nil {
		return nil, resp, err
	}

	return l, resp, err
}

// AddLicenseOptions represents the available AddLicense() options.
//
// https://docs.gitlab.com/ee/api/license.html#add-a-new-license
type AddLicenseOptions struct {
	License *string `url:"license" json:"license"`
}

// AddLicense adds a new license.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/license.html#add-a-new-license
func (s *LicenseService) AddLicense(opt *AddLicenseOptions, options ...OptionFunc) (*License, *Response, error) {
	req, err := s.client.NewRequest("POST", "license", opt, options)
	if err != nil {
		return nil, nil, err
	}

	l := new(License)
	resp, err := s.client.Do(req, l)
	if err != nil {
		return nil, resp, err
	}

	return l, resp, err
}
