// Copyright 2020 Matthew Holt
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package acmez

import (
	"context"

	"github.com/mholt/acmez/acme"
)

// Solver is a type that can solve ACME challenges. All
// implementations MUST honor context cancellation.
type Solver interface {
	// Present is called just before a challenge is initiated.
	// The implementation MUST prepare anything that is necessary
	// for completing the challenge; for example, provisioning
	// an HTTP resource, TLS certificate, or a DNS record.
	//
	// It MUST return quickly. If presenting the challenge token
	// will take time, then the implementation MUST do the
	// minimum amount of work required in this method, and
	// SHOULD additionally implement the Waiter interface.
	// For example, a DNS challenge solver might make a quick
	// HTTP request to a provider's API to create a new DNS
	// record, but it might be several minutes or hours before
	// the DNS record propagates. The API request should be
	// done in Present(), and waiting for propagation should
	// be done in Wait().
	Present(context.Context, acme.Challenge) error

	// CleanUp is called after a challenge is finished, whether
	// successful or not. It MUST free/remove any resources it
	// allocated/created during Present. It SHOULD NOT require
	// that Present ran successfully. It MUST return quickly.
	CleanUp(context.Context, acme.Challenge) error
}

// Waiter is an optional interface for Solvers to implement. Its
// primary purpose is to help ensure the challenge can be solved
// before the server gives up trying to verify the challenge.
//
// If implemented, it will be called after Present() but just
// before the challenge is initiated with the server. It blocks
// until the challenge is ready to be solved. (For example,
// waiting on a DNS record to propagate.) This allows challenges
// to succeed that would normally fail because they take too long
// to set up (i.e. the ACME server would give up polling DNS or
// the client would timeout its polling). By separating Present()
// from Wait(), it allows the slow part of all solvers to begin
// up front, rather than waiting on each solver one at a time.
//
// It MUST NOT do anything exclusive of Present() that is required
// for the challenge to succeed. In other words, if Present() is
// called but Wait() is not, then the challenge should still be able
// to succeed assuming infinite time.
//
// Implementations MUST honor context cancellation.
type Waiter interface {
	Wait(context.Context, acme.Challenge) error
}
