// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db // it's not db_test, because this file is for testing the private type halfCommitter

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockCommitter struct {
	wants []string
	gots  []string
}

func NewMockCommitter(wants ...string) *MockCommitter {
	return &MockCommitter{
		wants: wants,
	}
}

func (c *MockCommitter) Commit() error {
	c.gots = append(c.gots, "commit")
	return nil
}

func (c *MockCommitter) Close() error {
	c.gots = append(c.gots, "close")
	return nil
}

func (c *MockCommitter) Assert(t *testing.T) {
	assert.Equal(t, c.wants, c.gots, "want operations %v, but got %v", c.wants, c.gots)
}

func Test_halfCommitter(t *testing.T) {
	/*
		Do something like:

		ctx, committer, err := db.TxContext(db.DefaultContext)
		if err != nil {
			return nil
		}
		defer committer.Close()

		// ...

		if err != nil {
			return nil
		}

		// ...

		return committer.Commit()
	*/

	testWithCommitter := func(committer Committer, f func(committer Committer) error) {
		if err := f(&halfCommitter{committer: committer}); err == nil {
			committer.Commit()
		}
		committer.Close()
	}

	t.Run("commit and close", func(t *testing.T) {
		mockCommitter := NewMockCommitter("commit", "close")

		testWithCommitter(mockCommitter, func(committer Committer) error {
			defer committer.Close()
			return committer.Commit()
		})

		mockCommitter.Assert(t)
	})

	t.Run("rollback and close", func(t *testing.T) {
		mockCommitter := NewMockCommitter("close", "close")

		testWithCommitter(mockCommitter, func(committer Committer) error {
			defer committer.Close()
			if true {
				return errors.New("error")
			}
			return committer.Commit()
		})

		mockCommitter.Assert(t)
	})

	t.Run("close and commit", func(t *testing.T) {
		mockCommitter := NewMockCommitter("close", "close")

		testWithCommitter(mockCommitter, func(committer Committer) error {
			committer.Close()
			committer.Commit()
			return errors.New("error")
		})

		mockCommitter.Assert(t)
	})
}
