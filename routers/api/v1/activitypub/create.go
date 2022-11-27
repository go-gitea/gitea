// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/auth"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/forgefed"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/activitypub"
	issue_service "code.gitea.io/gitea/services/issue"
	pull_service "code.gitea.io/gitea/services/pull"
	repo_service "code.gitea.io/gitea/services/repository"
	user_service "code.gitea.io/gitea/services/user"

	ap "github.com/go-ap/activitypub"
)

// Create a new federated user from a Person object
func createPerson(ctx context.Context, person *ap.Person) error {
	name, err := activitypub.PersonIRIToName(person.GetLink())
	if err != nil {
		return err
	}

	exists, err := user_model.IsUserExist(ctx, 0, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	var email string
	if person.Location != nil {
		email = person.Location.GetLink().String()
	} else {
		// This might not even work
		email = strings.ReplaceAll(name, "@", "+") + "@" + setting.Service.NoReplyAddress
	}

	if person.PublicKey.PublicKeyPem == "" {
		return errors.New("person public key not found")
	}

	user := &user_model.User{
		Name:      name,
		FullName:  person.Name.String(), // May not exist!!
		Email:     email,
		LoginType: auth.Federated,
		LoginName: person.GetLink().String(),
	}
	err = user_model.CreateUser(user)
	if err != nil {
		return err
	}

	if person.Icon != nil {
		icon := person.Icon.(*ap.Image)
		iconURL, err := icon.URL.GetLink().URL()
		if err != nil {
			return err
		}

		body, err := activitypub.Fetch(iconURL)
		if err != nil {
			return err
		}

		err = user_service.UploadAvatar(user, body)
		if err != nil {
			return err
		}
	}

	err = user_model.SetUserSetting(user.ID, user_model.UserActivityPubPrivPem, "")
	if err != nil {
		return err
	}
	return user_model.SetUserSetting(user.ID, user_model.UserActivityPubPubPem, person.PublicKey.PublicKeyPem)
}

// Create a new federated repo from a Repository object
func createRepository(ctx context.Context, repository *forgefed.Repository) error {
	ownerURL, err := url.Parse(repository.AttributedTo.GetLink().String())
	if err != nil {
		return err
	}
	// Fetch person object
	resp, err := activitypub.Fetch(ownerURL)
	if err != nil {
		return err
	}
	// Parse person object
	ap.ItemTyperFunc = forgefed.GetItemByType
	ap.JSONItemUnmarshal = forgefed.JSONUnmarshalerFn
	ap.NotEmptyChecker = forgefed.NotEmpty
	object, err := ap.UnmarshalJSON(resp)
	if err != nil {
		return err
	}
	// Create federated user
	err = createPerson(ctx, object.(*ap.Person))
	if err != nil {
		return err
	}

	user, err := activitypub.PersonIRIToUser(ctx, repository.AttributedTo.GetLink())
	if err != nil {
		return err
	}

	// Check if repo exists
	_, err = repo_model.GetRepositoryByOwnerAndNameCtx(ctx, user.Name, repository.Name.String())
	if err == nil {
		return nil
	}

	repo, err := repo_service.CreateRepository(user, user, repo_module.CreateRepoOptions{
		Name: repository.Name.String(),
	})
	if err != nil {
		return err
	}

	if repository.ForkedFrom != nil {
		repo.IsFork = true
		forkedFrom, err := activitypub.RepositoryIRIToRepository(ctx, repository.ForkedFrom.GetLink())
		if err != nil {
			return err
		}
		repo.ForkID = forkedFrom.ID
	}
	return nil
}

// Create a ticket
func createTicket(ctx context.Context, ticket *forgefed.Ticket) error {
	if ticket.Origin != nil {
		return createPullRequest(ctx, ticket)
	}
	return createIssue(ctx, ticket)
}

// Create an issue
func createIssue(ctx context.Context, ticket *forgefed.Ticket) error {
	repoURL, err := url.Parse(ticket.Context.GetLink().String())
	if err != nil {
		return err
	}
	// Fetch repository object
	resp, err := activitypub.Fetch(repoURL)
	if err != nil {
		return err
	}
	// Parse repository object
	ap.ItemTyperFunc = forgefed.GetItemByType
	ap.JSONItemUnmarshal = forgefed.JSONUnmarshalerFn
	ap.NotEmptyChecker = forgefed.NotEmpty
	object, err := ap.UnmarshalJSON(resp)
	if err != nil {
		return err
	}
	// Create federated repo
	err = forgefed.OnRepository(object, func(r *forgefed.Repository) error {
		return createRepository(ctx, r)
	})
	if err != nil {
		return err
	}

	// Construct issue
	user, err := activitypub.PersonIRIToUser(ctx, ap.IRI(ticket.AttributedTo.GetLink().String()))
	if err != nil {
		return err
	}
	repo, err := activitypub.RepositoryIRIToRepository(ctx, ap.IRI(ticket.Context.GetLink().String()))
	if err != nil {
		return err
	}
	idx, err := strconv.ParseInt(ticket.Name.String()[1:], 10, 64)
	if err != nil {
		return err
	}
	issue := &issues_model.Issue{
		Index:    idx,
		RepoID:   repo.ID,
		Repo:     repo,
		Title:    ticket.Summary.String(),
		PosterID: user.ID,
		Poster:   user,
		Content:  ticket.Content.String(),
	}
	return issue_service.NewIssue(repo, issue, nil, nil, nil)
}

// Create a pull request
func createPullRequest(ctx context.Context, ticket *forgefed.Ticket) error {
	// TODO: Clean this up

	actorUser, err := activitypub.PersonIRIToUser(ctx, ticket.AttributedTo.GetLink())
	if err != nil {
		return err
	}

	// TODO: The IRI processing stuff should be moved to iri.go
	originIRI := ticket.Origin.GetLink()
	originIRISplit := strings.Split(originIRI.String(), "/")
	originInstance := originIRISplit[2]
	originUsername := originIRISplit[3]
	originReponame := originIRISplit[4]
	originBranch := originIRISplit[len(originIRISplit)-1]
	originRepo, _ := repo_model.GetRepositoryByOwnerAndName(originUsername+"@"+originInstance, originReponame)

	targetIRI := ticket.Target.GetLink()
	targetIRISplit := strings.Split(targetIRI.String(), "/")
	// targetInstance := targetIRISplit[2]
	targetUsername := targetIRISplit[3]
	targetReponame := targetIRISplit[4]
	targetBranch := targetIRISplit[len(targetIRISplit)-1]

	targetRepo, _ := repo_model.GetRepositoryByOwnerAndName(targetUsername, targetReponame)

	prIssue := &issues_model.Issue{
		RepoID:   targetRepo.ID,
		Title:    "Hello from test.exozy.me!", // Don't hardcode, get the title from the Ticket object
		PosterID: actorUser.ID,
		Poster:   actorUser,
		IsPull:   true,
		Content:  "🎉", // TODO: Get content from Ticket object
	}

	pr := &issues_model.PullRequest{
		HeadRepoID: originRepo.ID,
		BaseRepoID: targetRepo.ID,
		HeadBranch: originBranch,
		BaseBranch: targetBranch,
		HeadRepo:   originRepo,
		BaseRepo:   targetRepo,
		MergeBase:  "",
		Type:       issues_model.PullRequestGitea,
	}

	return pull_service.NewPullRequest(ctx, targetRepo, prIssue, []int64{}, []string{}, pr, []int64{})
}

// Create a comment
func createComment(ctx context.Context, note *ap.Note) error {
	actorUser, err := activitypub.PersonIRIToUser(ctx, note.AttributedTo.GetLink())
	if err != nil {
		return err
	}

	username, reponame, idx, err := activitypub.TicketIRIToName(note.Context.GetLink())
	if err != nil {
		return err
	}
	repo, err := repo_model.GetRepositoryByOwnerAndNameCtx(ctx, username, reponame)
	if err != nil {
		return err
	}
	issue, err := issues_model.GetIssueByIndex(repo.ID, idx)
	if err != nil {
		return err
	}
	_, err = issues_model.CreateCommentCtx(ctx, &issues_model.CreateCommentOptions{
		Doer:    actorUser,
		Repo:    repo,
		Issue:   issue,
		Content: note.Content.String(),
	})
	return err
}
