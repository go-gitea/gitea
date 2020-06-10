#!/bin/bash
#
# Copyright 2017, Joe Tsai. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE.md file.

# zfuzz wraps internal/tool/fuzz and is useful for fuzz testing each of
# the implementations in this repository.
cd $(dirname "${BASH_SOURCE[0]}")/internal/tool/fuzz
./fuzz.sh "$@"
