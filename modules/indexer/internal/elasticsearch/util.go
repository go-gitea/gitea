// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"code.gitea.io/gitea/modules/graceful"
	"errors"
	"net"

	"github.com/olivere/elastic/v7"
)

// CheckError checks if the error is a connection error and sets the availability
func (i *Indexer) CheckError(err error) error {
	var opErr *net.OpError
	if !(elastic.IsConnErr(err) || (errors.As(err, &opErr) && (opErr.Op == "dial" || opErr.Op == "read"))) {
		return err
	}

	i.setAvailability(false)

	return err
}

func (i *Indexer) setAvailability(available bool) {
	i.lock.Lock()
	defer i.lock.Unlock()

	if i.available == available {
		return
	}

	i.available = available
}

func (i *Indexer) checkAvailability() {
	if i.Ping() {
		return
	}

	// Request cluster state to check if elastic is available again
	_, err := i.Client.ClusterState().Do(graceful.GetManager().ShutdownContext())
	if err != nil {
		i.setAvailability(false)
		return
	}

	i.setAvailability(true)
}
