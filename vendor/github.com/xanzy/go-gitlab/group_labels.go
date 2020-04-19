package gitlab

import (
	"fmt"
)

// GroupLabelsService handles communication with the label related methods of the
// GitLab API.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/group_labels.html
type GroupLabelsService struct {
	client *Client
}

// GroupLabel represents a GitLab group label.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/group_labels.html
type GroupLabel Label

func (l GroupLabel) String() string {
	return Stringify(l)
}

// ListGroupLabelsOptions represents the available ListGroupLabels() options.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/labels.html#list-labels
type ListGroupLabelsOptions ListOptions

// ListGroupLabels gets all labels for given group.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/group_labels.html#list-group-labels
func (s *GroupLabelsService) ListGroupLabels(gid interface{}, opt *ListGroupLabelsOptions, options ...OptionFunc) ([]*GroupLabel, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/labels", pathEscape(group))

	req, err := s.client.NewRequest("GET", u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	var l []*GroupLabel
	resp, err := s.client.Do(req, &l)
	if err != nil {
		return nil, resp, err
	}

	return l, resp, err
}

// CreateGroupLabelOptions represents the available CreateGroupLabel() options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/group_labels.html#create-a-new-group-label
type CreateGroupLabelOptions CreateLabelOptions

// CreateGroupLabel creates a new label for given group with given name and
// color.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/group_labels.html#create-a-new-group-label
func (s *GroupLabelsService) CreateGroupLabel(gid interface{}, opt *CreateGroupLabelOptions, options ...OptionFunc) (*GroupLabel, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/labels", pathEscape(group))

	req, err := s.client.NewRequest("POST", u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	l := new(GroupLabel)
	resp, err := s.client.Do(req, l)
	if err != nil {
		return nil, resp, err
	}

	return l, resp, err
}

// DeleteGroupLabelOptions represents the available DeleteGroupLabel() options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/group_labels.html#delete-a-group-label
type DeleteGroupLabelOptions DeleteLabelOptions

// DeleteGroupLabel deletes a group label given by its name.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/labels.html#delete-a-label
func (s *GroupLabelsService) DeleteGroupLabel(gid interface{}, opt *DeleteGroupLabelOptions, options ...OptionFunc) (*Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("groups/%s/labels", pathEscape(group))

	req, err := s.client.NewRequest("DELETE", u, opt, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// UpdateGroupLabelOptions represents the available UpdateGroupLabel() options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/group_labels.html#update-a-group-label
type UpdateGroupLabelOptions UpdateLabelOptions

// UpdateGroupLabel updates an existing label with new name or now color. At least
// one parameter is required, to update the label.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/group_labels.html#update-a-group-label
func (s *GroupLabelsService) UpdateGroupLabel(gid interface{}, opt *UpdateGroupLabelOptions, options ...OptionFunc) (*GroupLabel, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/labels", pathEscape(group))

	req, err := s.client.NewRequest("PUT", u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	l := new(GroupLabel)
	resp, err := s.client.Do(req, l)
	if err != nil {
		return nil, resp, err
	}

	return l, resp, err
}

// SubscribeToGroupLabel subscribes the authenticated user to a label to receive
// notifications. If the user is already subscribed to the label, the status
// code 304 is returned.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/group_labels.html#subscribe-to-a-group-label
func (s *GroupLabelsService) SubscribeToGroupLabel(gid interface{}, labelID interface{}, options ...OptionFunc) (*GroupLabel, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	label, err := parseID(labelID)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/labels/%s/subscribe", pathEscape(group), label)

	req, err := s.client.NewRequest("POST", u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	l := new(GroupLabel)
	resp, err := s.client.Do(req, l)
	if err != nil {
		return nil, resp, err
	}

	return l, resp, err
}

// UnsubscribeFromGroupLabel unsubscribes the authenticated user from a label to not
// receive notifications from it. If the user is not subscribed to the label, the
// status code 304 is returned.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/group_labels.html#unsubscribe-from-a-group-label
func (s *GroupLabelsService) UnsubscribeFromGroupLabel(gid interface{}, labelID interface{}, options ...OptionFunc) (*Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, err
	}
	label, err := parseID(labelID)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("groups/%s/labels/%s/unsubscribe", pathEscape(group), label)

	req, err := s.client.NewRequest("POST", u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
