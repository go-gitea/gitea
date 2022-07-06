// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

// This migration added non-ideal indices to the action table which on larger datasets slowed things down
// it has been superceded by v218.go
