// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package consts

const (
	AsymKeyMinBitsRsa = 3071 // 3072-1 to tolerate the leading zero
	AsymKeyMinBitsEC  = 256

	AsymKeyDefaultBitsRsa   = 4096 // ssh-keygen command defaults to 3072
	AsymKeyDefaultBitsEcdsa = 256
)
