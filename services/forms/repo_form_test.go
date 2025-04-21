// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubmitReviewForm_IsEmpty(t *testing.T) {
	cases := []struct {
		form     SubmitReviewForm
		expected bool
	}{
		// Approved PR with a comment shouldn't count as empty
		{SubmitReviewForm{Type: "approve", Content: "Awesome"}, false},

		// Approved PR without a comment shouldn't count as empty
		{SubmitReviewForm{Type: "approve", Content: ""}, false},

		// Rejected PR without a comment should count as empty
		{SubmitReviewForm{Type: "reject", Content: ""}, true},

		// Rejected PR with a comment shouldn't count as empty
		{SubmitReviewForm{Type: "reject", Content: "Awesome"}, false},

		// Comment review on a PR with a comment shouldn't count as empty
		{SubmitReviewForm{Type: "comment", Content: "Awesome"}, false},

		// Comment review on a PR without a comment should count as empty
		{SubmitReviewForm{Type: "comment", Content: ""}, true},
	}

	for _, v := range cases {
		assert.Equal(t, v.expected, v.form.HasEmptyContent())
	}
}
