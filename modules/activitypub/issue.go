// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"context"
	"fmt"
	"strconv"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/forgefed"
	issue_service "code.gitea.io/gitea/services/issue"

	ap "github.com/go-ap/activitypub"
)

// Create an issue
func ReceiveIssue(ctx context.Context, ticket *forgefed.Ticket) error {
	// Construct issue
	user, err := PersonIRIToUser(ctx, ap.IRI(ticket.AttributedTo.GetLink().String()))
	if err != nil {
		return err
	}
	repo, err := RepositoryIRIToRepository(ctx, ap.IRI(ticket.Context.GetLink().String()))
	if err != nil {
		return err
	}
	fmt.Println(ticket)
	fmt.Println(ticket.Name.String())
	idx, err := strconv.ParseInt(ticket.Name.String()[1:], 10, 64)
	if err != nil {
		return err
	}
	issue := &issues_model.Issue{
		ID:       idx,
		RepoID:   repo.ID,
		Repo:     repo,
		Title:    ticket.Summary.String(),
		PosterID: user.ID,
		Poster:   user,
		Content:  ticket.Content.String(),
	}
	fmt.Println(issue)
	return issue_service.NewIssue(repo, issue, nil, nil, nil)
}
