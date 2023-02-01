package migrations

import (
	"context"
	"errors"

	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
)

type UsernameCache struct {
	nameToID map[string]int64
	idToUser map[int64]*user_model.User
}

func NewUsernameCache() *UsernameCache {
	return &UsernameCache{
		nameToID: make(map[string]int64),
		idToUser: make(map[int64]*user_model.User),
	}
}

func (c *UsernameCache) FindUserIDByName(ctx context.Context, username string) (int64, error) {
	if userID, found := c.nameToID[username]; found {
		return userID, nil
	}

	singleUsernameList := []string{username}
	userIDs, err := issues_model.MakeIDsFromAPIAssigneesToAdd(ctx, "", singleUsernameList)

	// Store this username even when erroring. Seeing the error once is enough.
	c.nameToID[username] = 0

	if err != nil {
		return 0, err
	}
	if len(userIDs) == 0 {
		return 0, errors.New("unknown failure in fetching user IDs")
	}

	userID := userIDs[0]
	c.nameToID[username] = userID
	return userID, nil
}

func (c *UsernameCache) GetUserByID(ctx context.Context, id int64) (*user_model.User, error) {
	if user, found := c.idToUser[id]; found {
		return user, nil
	}

	user, err := user_model.GetUserByID(ctx, id)
	c.idToUser[id] = user

	return user, err
}
