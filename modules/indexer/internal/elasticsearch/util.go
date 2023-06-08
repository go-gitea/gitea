// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
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

	i.SetAvailability(false)

	return err
}

func (i *Indexer) SetAvailability(available bool) {
	i.lock.Lock()
	defer i.lock.Unlock()

	if i.available == available {
		return
	}

	i.available = available
}
