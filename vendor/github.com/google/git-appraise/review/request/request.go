/*
Copyright 2015 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package request defines the internal representation of a review request.
package request

import (
	"encoding/json"
	"github.com/google/git-appraise/repository"
	"strconv"
	"time"
)

// Ref defines the git-notes ref that we expect to contain review requests.
const Ref = "refs/notes/devtools/reviews"

// FormatVersion defines the latest version of the request format supported by the tool.
const FormatVersion = 0

// Request represents an initial request for a code review.
//
// Every field except for TargetRef is optional.
type Request struct {
	// Timestamp and Requester are optimizations that allows us to display reviews
	// without having to run git-blame over the notes object. This is done because
	// git-blame will become more and more expensive as the number of reviews grows.
	Timestamp   string   `json:"timestamp,omitempty"`
	ReviewRef   string   `json:"reviewRef,omitempty"`
	TargetRef   string   `json:"targetRef"`
	Requester   string   `json:"requester,omitempty"`
	Reviewers   []string `json:"reviewers,omitempty"`
	Description string   `json:"description,omitempty"`
	// Version represents the version of the metadata format.
	Version int `json:"v,omitempty"`
	// BaseCommit stores the commit ID of the target ref at the time the review was requested.
	// This is optional, and only used for submitted reviews which were anchored at a merge commit.
	// This allows someone viewing that submitted review to find the diff against which the
	// code was reviewed.
	BaseCommit string `json:"baseCommit,omitempty"`
	// Alias stores a post-rebase commit ID for the review. This allows the tool
	// to track the history of a review even if the commit history changes.
	Alias string `json:"alias,omitempty"`
}

// New returns a new request.
//
// The Timestamp and Requester fields are automatically filled in with the current time and user.
func New(requester string, reviewers []string, reviewRef, targetRef, description string) Request {
	return Request{
		Timestamp:   strconv.FormatInt(time.Now().Unix(), 10),
		Requester:   requester,
		Reviewers:   reviewers,
		ReviewRef:   reviewRef,
		TargetRef:   targetRef,
		Description: description,
	}
}

// Parse parses a review request from a git note.
func Parse(note repository.Note) (Request, error) {
	bytes := []byte(note)
	var request Request
	err := json.Unmarshal(bytes, &request)
	// TODO(ojarjur): If "requester" is not set, then use git-blame to fill it in.
	return request, err
}

// ParseAllValid takes collection of git notes and tries to parse a review
// request from each one. Any notes that are not valid review requests get
// ignored, as we expect the git notes to be a heterogenous list, with only
// some of them being review requests.
func ParseAllValid(notes []repository.Note) []Request {
	var requests []Request
	for _, note := range notes {
		request, err := Parse(note)
		if err == nil && request.Version == FormatVersion && request.TargetRef != "" {
			requests = append(requests, request)
		}
	}
	return requests
}

// Write writes a review request as a JSON-formatted git note.
func (request *Request) Write() (repository.Note, error) {
	bytes, err := json.Marshal(request)
	return repository.Note(bytes), err
}
