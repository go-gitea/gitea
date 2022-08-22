// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package grpc

import (
	"net/http"

	grpchealth "github.com/bufbuild/connect-grpchealth-go"
)

func HealthRoute() (string, http.Handler) {
	// grpcHealthCheck
	return grpchealth.NewHandler(
		grpchealth.NewStaticChecker(allServices...),
		compress1KB,
	)
}
