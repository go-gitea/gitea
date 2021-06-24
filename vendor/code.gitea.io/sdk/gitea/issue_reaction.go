// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// Reaction contain one reaction
type Reaction struct {
	User     *User     `json:"user"`
	Reaction string    `json:"content"`
	Created  time.Time `json:"created_at"`
}

// GetIssueReactions get a list reactions of an issue
func (c *Client) GetIssueReactions(owner, repo string, index int64) ([]*Reaction, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	reactions := make([]*Reaction, 0, 10)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/issues/%d/reactions", owner, repo, index), nil, nil, &reactions)
	return reactions, resp, err
}

// GetIssueCommentReactions get a list of reactions from a comment of an issue
func (c *Client) GetIssueCommentReactions(owner, repo string, commentID int64) ([]*Reaction, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	reactions := make([]*Reaction, 0, 10)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/issues/comments/%d/reactions", owner, repo, commentID), nil, nil, &reactions)
	return reactions, resp, err
}

// editReactionOption contain the reaction type
type editReactionOption struct {
	Reaction string `json:"content"`
}

// PostIssueReaction add a reaction to an issue
func (c *Client) PostIssueReaction(owner, repo string, index int64, reaction string) (*Reaction, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	reactionResponse := new(Reaction)
	body, err := json.Marshal(&editReactionOption{Reaction: reaction})
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.getParsedResponse("POST",
		fmt.Sprintf("/repos/%s/%s/issues/%d/reactions", owner, repo, index),
		jsonHeader, bytes.NewReader(body), reactionResponse)
	return reactionResponse, resp, err
}

// DeleteIssueReaction remove a reaction from an issue
func (c *Client) DeleteIssueReaction(owner, repo string, index int64, reaction string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&editReactionOption{Reaction: reaction})
	if err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/issues/%d/reactions", owner, repo, index), jsonHeader, bytes.NewReader(body))
	return resp, err
}

// PostIssueCommentReaction add a reaction to a comment of an issue
func (c *Client) PostIssueCommentReaction(owner, repo string, commentID int64, reaction string) (*Reaction, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	reactionResponse := new(Reaction)
	body, err := json.Marshal(&editReactionOption{Reaction: reaction})
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.getParsedResponse("POST",
		fmt.Sprintf("/repos/%s/%s/issues/comments/%d/reactions", owner, repo, commentID),
		jsonHeader, bytes.NewReader(body), reactionResponse)
	return reactionResponse, resp, err
}

// DeleteIssueCommentReaction remove a reaction from a comment of an issue
func (c *Client) DeleteIssueCommentReaction(owner, repo string, commentID int64, reaction string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&editReactionOption{Reaction: reaction})
	if err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("DELETE",
		fmt.Sprintf("/repos/%s/%s/issues/comments/%d/reactions", owner, repo, commentID),
		jsonHeader, bytes.NewReader(body))
	return resp, err
}
