#!/bin/bash
#
# Copyright 2017, Joe Tsai. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE.md file.

# zbench wraps internal/tool/bench and is useful for comparing benchmarks from
# the implementations in this repository relative to other implementations.
#
# See internal/tool/bench/main.go for more details.
cd $(dirname "${BASH_SOURCE[0]}")/internal/tool/bench
go run $(go list -f '{{ join .GoFiles "\n" }}') "$@"
