// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"context"
	"errors"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	ap "github.com/go-ap/activitypub"
)

// Returns the username corresponding to a Person actor IRI
func personIRIToName(personIRI ap.IRI) (string, error) {
	personIRISplit := strings.Split(personIRI.String(), "/")
	if len(personIRISplit) < 3 {
		return "", errors.New("Not a Person actor IRI")
	}

	instance := personIRISplit[2]
	name := personIRISplit[len(personIRISplit)-1]
	if instance == setting.Domain {
		// Local user
		return name, nil
	} else {
		// Remote user
		// Get name in username@instance.com format
		return name + "@" + instance, nil
	}
}

// Returns the user corresponding to a Person actor IRI
func personIRIToUser(ctx context.Context, personIRI ap.IRI) (*user_model.User, error) {
	name, err := personIRIToName(personIRI)
	if err != nil {
		return nil, err
	}

	user, err := user_model.GetUserByName(ctx, name)
	if err != nil || !strings.Contains(name, "@") {
		return user, err
	}
	FederatedUserNew(personIRI)
	return user_model.GetUserByName(ctx, name)
}

// Returns the owner and name corresponding to a Repository actor IRI
func repositoryIRIToName(repoIRI ap.IRI) (string, string, error) {
	repoIRISplit := strings.Split(repoIRI.String(), "/")
	if len(repoIRISplit) < 5 {
		return "", "", errors.New("Not a Repository actor IRI")
	}

	instance := repoIRISplit[2]
	username := repoIRISplit[len(repoIRISplit)-2]
	reponame := repoIRISplit[len(repoIRISplit)-1]
	if instance == setting.Domain {
		// Local repo
		return username, reponame, nil
	} else {
		// Remote repo
		return username + "@" + instance, reponame, nil
	}
}

// Returns the repository corresponding to a Repository actor IRI
func repositoryIRIToRepository(ctx context.Context, repoIRI ap.IRI) (*repo_model.Repository, error) {
	username, reponame, err := repositoryIRIToName(repoIRI)
	if err != nil {
		return nil, err
	}

	return repo_model.GetRepositoryByOwnerAndName(username, reponame)
}
