package gitlab

import (
	"fmt"
	"time"
)

// EpicsService handles communication with the epic related methods
// of the GitLab API.
//
// GitLab API docs: https://docs.gitlab.com/ee/api/epics.html
type EpicsService struct {
	client *Client
}

// EpicAuthor represents a author of the epic.
type EpicAuthor struct {
	ID        int    `json:"id"`
	State     string `json:"state"`
	WebURL    string `json:"web_url"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Username  string `json:"username"`
}

// Epic represents a GitLab epic.
//
// GitLab API docs: https://docs.gitlab.com/ee/api/epics.html
type Epic struct {
	ID                      int         `json:"id"`
	IID                     int         `json:"iid"`
	GroupID                 int         `json:"group_id"`
	Author                  *EpicAuthor `json:"author"`
	Description             string      `json:"description"`
	State                   string      `json:"state"`
	Upvotes                 int         `json:"upvotes"`
	Downvotes               int         `json:"downvotes"`
	Labels                  []string    `json:"labels"`
	Title                   string      `json:"title"`
	UpdatedAt               *time.Time  `json:"updated_at"`
	CreatedAt               *time.Time  `json:"created_at"`
	UserNotesCount          int         `json:"user_notes_count"`
	StartDate               *ISOTime    `json:"start_date"`
	StartDateIsFixed        bool        `json:"start_date_is_fixed"`
	StartDateFixed          *ISOTime    `json:"start_date_fixed"`
	StartDateFromMilestones *ISOTime    `json:"start_date_from_milestones"`
	DueDate                 *ISOTime    `json:"due_date"`
	DueDateIsFixed          bool        `json:"due_date_is_fixed"`
	DueDateFixed            *ISOTime    `json:"due_date_fixed"`
	DueDateFromMilestones   *ISOTime    `json:"due_date_from_milestones"`
}

func (e Epic) String() string {
	return Stringify(e)
}

// ListGroupEpicsOptions represents the available ListGroupEpics() options.
//
// GitLab API docs: https://docs.gitlab.com/ee/api/epics.html#list-epics-for-a-group
type ListGroupEpicsOptions struct {
	ListOptions
	State    *string `url:"state,omitempty" json:"state,omitempty"`
	Labels   Labels  `url:"labels,comma,omitempty" json:"labels,omitempty"`
	AuthorID *int    `url:"author_id,omitempty" json:"author_id,omitempty"`
	OrderBy  *string `url:"order_by,omitempty" json:"order_by,omitempty"`
	Sort     *string `url:"sort,omitempty" json:"sort,omitempty"`
	Search   *string `url:"search,omitempty" json:"search,omitempty"`
}

// ListGroupEpics gets a list of group epics. This function accepts pagination
// parameters page and per_page to return the list of group epics.
//
// GitLab API docs: https://docs.gitlab.com/ee/api/epics.html#list-epics-for-a-group
func (s *EpicsService) ListGroupEpics(gid interface{}, opt *ListGroupEpicsOptions, options ...OptionFunc) ([]*Epic, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/epics", pathEscape(group))

	req, err := s.client.NewRequest("GET", u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	var es []*Epic
	resp, err := s.client.Do(req, &es)
	if err != nil {
		return nil, resp, err
	}

	return es, resp, err
}

// GetEpic gets a single group epic.
//
// GitLab API docs: https://docs.gitlab.com/ee/api/epics.html#single-epic
func (s *EpicsService) GetEpic(gid interface{}, epic int, options ...OptionFunc) (*Epic, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/epics/%d", pathEscape(group), epic)

	req, err := s.client.NewRequest("GET", u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	e := new(Epic)
	resp, err := s.client.Do(req, e)
	if err != nil {
		return nil, resp, err
	}

	return e, resp, err
}

// CreateEpicOptions represents the available CreateEpic() options.
//
// GitLab API docs: https://docs.gitlab.com/ee/api/epics.html#new-epic
type CreateEpicOptions struct {
	Title            *string  `url:"title,omitempty" json:"title,omitempty"`
	Description      *string  `url:"description,omitempty" json:"description,omitempty"`
	Labels           Labels   `url:"labels,comma,omitempty" json:"labels,omitempty"`
	StartDateIsFixed *bool    `url:"start_date_is_fixed,omitempty" json:"start_date_is_fixed,omitempty"`
	StartDateFixed   *ISOTime `url:"start_date_fixed,omitempty" json:"start_date_fixed,omitempty"`
	DueDateIsFixed   *bool    `url:"due_date_is_fixed,omitempty" json:"due_date_is_fixed,omitempty"`
	DueDateFixed     *ISOTime `url:"due_date_fixed,omitempty" json:"due_date_fixed,omitempty"`
}

// CreateEpic creates a new group epic.
//
// GitLab API docs: https://docs.gitlab.com/ee/api/epics.html#new-epic
func (s *EpicsService) CreateEpic(gid interface{}, opt *CreateEpicOptions, options ...OptionFunc) (*Epic, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/epics", pathEscape(group))

	req, err := s.client.NewRequest("POST", u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	e := new(Epic)
	resp, err := s.client.Do(req, e)
	if err != nil {
		return nil, resp, err
	}

	return e, resp, err
}

// UpdateEpicOptions represents the available UpdateEpic() options.
//
// GitLab API docs: https://docs.gitlab.com/ee/api/epics.html#update-epic
type UpdateEpicOptions struct {
	Title            *string  `url:"title,omitempty" json:"title,omitempty"`
	Description      *string  `url:"description,omitempty" json:"description,omitempty"`
	Labels           Labels   `url:"labels,comma,omitempty" json:"labels,omitempty"`
	StartDateIsFixed *bool    `url:"start_date_is_fixed,omitempty" json:"start_date_is_fixed,omitempty"`
	StartDateFixed   *ISOTime `url:"start_date_fixed,omitempty" json:"start_date_fixed,omitempty"`
	DueDateIsFixed   *bool    `url:"due_date_is_fixed,omitempty" json:"due_date_is_fixed,omitempty"`
	DueDateFixed     *ISOTime `url:"due_date_fixed,omitempty" json:"due_date_fixed,omitempty"`
	StateEvent       *string  `url:"state_event,omitempty" json:"state_event,omitempty"`
}

// UpdateEpic updates an existing group epic. This function is also used
// to mark an epic as closed.
//
// GitLab API docs: https://docs.gitlab.com/ee/api/epics.html#update-epic
func (s *EpicsService) UpdateEpic(gid interface{}, epic int, opt *UpdateEpicOptions, options ...OptionFunc) (*Epic, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/epics/%d", pathEscape(group), epic)

	req, err := s.client.NewRequest("PUT", u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	e := new(Epic)
	resp, err := s.client.Do(req, e)
	if err != nil {
		return nil, resp, err
	}

	return e, resp, err
}

// DeleteEpic deletes a single group epic.
//
// GitLab API docs: https://docs.gitlab.com/ee/api/epics.html#delete-epic
func (s *EpicsService) DeleteEpic(gid interface{}, epic int, options ...OptionFunc) (*Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("groups/%s/epics/%d", pathEscape(group), epic)

	req, err := s.client.NewRequest("DELETE", u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
