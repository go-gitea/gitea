// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"context"
	"errors"
	"strconv"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	ap "github.com/go-ap/activitypub"
)

// Returns the username corresponding to a Person actor IRI
func PersonIRIToName(personIRI ap.IRI) (string, error) {
	personIRISplit := strings.Split(personIRI.String(), "/")
	if len(personIRISplit) < 4 {
		return "", errors.New("not a Person actor IRI")
	}

	instance := personIRISplit[2]
	name := personIRISplit[len(personIRISplit)-1]
	if instance == setting.Domain {
		// Local user
		return name, nil
	}
	// Remote user
	// Get name in username@instance.com format
	return name + "@" + instance, nil
}

// Returns the user corresponding to a Person actor IRI
func PersonIRIToUser(ctx context.Context, personIRI ap.IRI) (*user_model.User, error) {
	name, err := PersonIRIToName(personIRI)
	if err != nil {
		return nil, err
	}

	user, err := user_model.GetUserByName(ctx, name)
	if err != nil && !strings.Contains(name, "@") {
		return user, err
	}

	return user_model.GetUserByName(ctx, name)
}

// Returns the owner and name corresponding to a Repository actor IRI
func RepositoryIRIToName(repoIRI ap.IRI) (string, string, error) {
	repoIRISplit := strings.Split(repoIRI.String(), "/")
	if len(repoIRISplit) < 5 {
		return "", "", errors.New("not a Repository actor IRI")
	}

	instance := repoIRISplit[2]
	username := repoIRISplit[len(repoIRISplit)-2]
	reponame := repoIRISplit[len(repoIRISplit)-1]
	if instance == setting.Domain {
		// Local repo
		return username, reponame, nil
	}
	// Remote repo
	return username + "@" + instance, reponame, nil
}

// Returns the repository corresponding to a Repository actor IRI
func RepositoryIRIToRepository(ctx context.Context, repoIRI ap.IRI) (*repo_model.Repository, error) {
	username, reponame, err := RepositoryIRIToName(repoIRI)
	if err != nil {
		return nil, err
	}

	// TODO: create remote repo if not exists
	return repo_model.GetRepositoryByOwnerAndName(username, reponame)
}

// Returns the owner, repo name, and idx of a Ticket object IRI
func TicketIRIToName(ticketIRI ap.IRI) (string, string, int64, error) {
	ticketIRISplit := strings.Split(ticketIRI.String(), "/")
	if len(ticketIRISplit) < 5 {
		return "", "", 0, errors.New("not a Ticket actor IRI")
	}

	instance := ticketIRISplit[2]
	username := ticketIRISplit[len(ticketIRISplit)-3]
	reponame := ticketIRISplit[len(ticketIRISplit)-2]
	idx, err := strconv.ParseInt(ticketIRISplit[len(ticketIRISplit)-1], 10, 64)
	if err != nil {
		return "", "", 0, err
	}
	if instance == setting.Domain {
		// Local repo
		return username, reponame, idx, nil
	}
	// Remote repo
	return username + "@" + instance, reponame, idx, nil
}
