// Copyright 2015 Matthew Holt
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

package certmagic

import (
	"net/http"
	"strings"

	"github.com/mholt/acmez/acme"
	"go.uber.org/zap"
)

// HTTPChallengeHandler wraps h in a handler that can solve the ACME
// HTTP challenge. cfg is required, and it must have a certificate
// cache backed by a functional storage facility, since that is where
// the challenge state is stored between initiation and solution.
//
// If a request is not an ACME HTTP challenge, h will be invoked.
func (am *ACMEManager) HTTPChallengeHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if am.HandleHTTPChallenge(w, r) {
			return
		}
		h.ServeHTTP(w, r)
	})
}

// HandleHTTPChallenge uses am to solve challenge requests from an ACME
// server that were initiated by this instance or any other instance in
// this cluster (being, any instances using the same storage am does).
//
// If the HTTP challenge is disabled, this function is a no-op.
//
// If am is nil or if am does not have a certificate cache backed by
// usable storage, solving the HTTP challenge will fail.
//
// It returns true if it handled the request; if so, the response has
// already been written. If false is returned, this call was a no-op and
// the request has not been handled.
func (am *ACMEManager) HandleHTTPChallenge(w http.ResponseWriter, r *http.Request) bool {
	if am == nil {
		return false
	}
	if am.DisableHTTPChallenge {
		return false
	}
	if !LooksLikeHTTPChallenge(r) {
		return false
	}
	return am.distributedHTTPChallengeSolver(w, r)
}

// distributedHTTPChallengeSolver checks to see if this challenge
// request was initiated by this or another instance which uses the
// same storage as am does, and attempts to complete the challenge for
// it. It returns true if the request was handled; false otherwise.
func (am *ACMEManager) distributedHTTPChallengeSolver(w http.ResponseWriter, r *http.Request) bool {
	if am == nil {
		return false
	}
	host := hostOnly(r.Host)
	chalInfo, distributed, err := am.config.getChallengeInfo(host)
	if err != nil {
		if am.Logger != nil {
			am.Logger.Error("looking up info for HTTP challenge",
				zap.String("host", host),
				zap.Error(err))
		}
		return false
	}
	return solveHTTPChallenge(am.Logger, w, r, chalInfo.Challenge, distributed)
}

// solveHTTPChallenge solves the HTTP challenge using the given challenge information.
// If the challenge is being solved in a distributed fahsion, set distributed to true for logging purposes.
// It returns true the properties of the request check out in relation to the HTTP challenge.
// Most of this code borrowed from xenolf's built-in HTTP-01 challenge solver in March 2018.
func solveHTTPChallenge(logger *zap.Logger, w http.ResponseWriter, r *http.Request, challenge acme.Challenge, distributed bool) bool {
	challengeReqPath := challenge.HTTP01ResourcePath()
	if r.URL.Path == challengeReqPath &&
		strings.EqualFold(hostOnly(r.Host), challenge.Identifier.Value) && // mitigate DNS rebinding attacks
		r.Method == "GET" {
		w.Header().Add("Content-Type", "text/plain")
		w.Write([]byte(challenge.KeyAuthorization))
		r.Close = true
		if logger != nil {
			logger.Info("served key authentication",
				zap.String("identifier", challenge.Identifier.Value),
				zap.String("challenge", "http-01"),
				zap.String("remote", r.RemoteAddr),
				zap.Bool("distributed", distributed))
		}
		return true
	}
	return false
}

// SolveHTTPChallenge solves the HTTP challenge. It should be used only on HTTP requests that are
// from ACME servers trying to validate an identifier (i.e. LooksLikeHTTPChallenge() == true). It
// returns true if the request criteria check out and it answered with key authentication, in which
// case no further handling of the request is necessary.
func SolveHTTPChallenge(logger *zap.Logger, w http.ResponseWriter, r *http.Request, challenge acme.Challenge) bool {
	return solveHTTPChallenge(logger, w, r, challenge, false)
}

// LooksLikeHTTPChallenge returns true if r looks like an ACME
// HTTP challenge request from an ACME server.
func LooksLikeHTTPChallenge(r *http.Request) bool {
	return r.Method == "GET" && strings.HasPrefix(r.URL.Path, challengeBasePath)
}

const challengeBasePath = "/.well-known/acme-challenge"
