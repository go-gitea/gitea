// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

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
