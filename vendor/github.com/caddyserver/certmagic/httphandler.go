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
	"encoding/json"
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

	tokenKey := distributedSolver{acmeManager: am, caURL: am.CA}.challengeTokensKey(host)
	chalInfoBytes, err := am.config.Storage.Load(tokenKey)
	if err != nil {
		if _, ok := err.(ErrNotExist); !ok {
			if am.Logger != nil {
				am.Logger.Error("opening distributed HTTP challenge token file",
					zap.String("host", host),
					zap.Error(err))
			}
		}
		return false
	}

	var challenge acme.Challenge
	err = json.Unmarshal(chalInfoBytes, &challenge)
	if err != nil {
		if am.Logger != nil {
			am.Logger.Error("decoding HTTP challenge token file (corrupted?)",
				zap.String("host", host),
				zap.String("token_key", tokenKey),
				zap.Error(err))
		}
		return false
	}

	return am.answerHTTPChallenge(w, r, challenge)
}

// answerHTTPChallenge solves the challenge with chalInfo.
// Most of this code borrowed from xenolf's built-in HTTP-01
// challenge solver in March 2018.
func (am *ACMEManager) answerHTTPChallenge(w http.ResponseWriter, r *http.Request, challenge acme.Challenge) bool {
	challengeReqPath := challenge.HTTP01ResourcePath()
	if r.URL.Path == challengeReqPath &&
		strings.EqualFold(hostOnly(r.Host), challenge.Identifier.Value) && // mitigate DNS rebinding attacks
		r.Method == "GET" {
		w.Header().Add("Content-Type", "text/plain")
		w.Write([]byte(challenge.KeyAuthorization))
		r.Close = true
		if am.Logger != nil {
			am.Logger.Info("served key authentication",
				zap.String("identifier", challenge.Identifier.Value),
				zap.String("challenge", "http-01"),
				zap.String("remote", r.RemoteAddr))
		}
		return true
	}
	return false
}

// LooksLikeHTTPChallenge returns true if r looks like an ACME
// HTTP challenge request from an ACME server.
func LooksLikeHTTPChallenge(r *http.Request) bool {
	return r.Method == "GET" && strings.HasPrefix(r.URL.Path, challengeBasePath)
}

const challengeBasePath = "/.well-known/acme-challenge"
