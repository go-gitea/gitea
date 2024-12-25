package group

import (
	"context"
)

type GroupList []*Group

func (groups GroupList) LoadOwners(ctx context.Context) error {
	for _, g := range groups {
		err := g.LoadOwner(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}
