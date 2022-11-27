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
	issue_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/forgefed"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/activitypub"
	issue_service "code.gitea.io/gitea/services/issue"
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

// Create an issue
func createIssue(ctx context.Context, ticket *forgefed.Ticket) error {
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
	issue := &issue_model.Issue{
		ID:       idx,
		RepoID:   repo.ID,
		Repo:     repo,
		Title:    ticket.Summary.String(),
		PosterID: user.ID,
		Poster:   user,
		Content:  ticket.Content.String(),
	}
	return issue_service.NewIssue(repo, issue, nil, nil, nil)
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
	issue, err := issue_model.GetIssueByIndex(repo.ID, idx)
	if err != nil {
		return err
	}
	_, err = issue_model.CreateCommentCtx(ctx, &issue_model.CreateCommentOptions{
		Doer:    actorUser,
		Repo:    repo,
		Issue:   issue,
		Content: note.Content.String(),
	})
	return err
}
