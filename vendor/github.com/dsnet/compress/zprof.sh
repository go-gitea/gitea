#!/bin/bash
#
# Copyright 2017, Joe Tsai. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE.md file.

if [ $# == 0 ]; then
	echo "Usage: $0 PKG_PATH TEST_ARGS..."
	echo ""
	echo "Runs coverage and performance benchmarks for a given package."
	echo "The results are stored in the _zprof_ directory."
	echo ""
	echo "Example:"
	echo "	$0 flate -test.bench=Decode/Twain/Default"
	exit 1
fi

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PKG_PATH=$1
PKG_NAME=$(basename $PKG_PATH)
shift

TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR $PKG_PATH/$PKG_NAME.test" SIGINT SIGTERM EXIT

(
	cd $DIR/$PKG_PATH

	# Print the go version.
	go version

	# Perform coverage profiling.
	go test github.com/dsnet/compress/$PKG_PATH -coverprofile $TMPDIR/cover.profile
	if [ $? != 0 ]; then exit 1; fi
	go tool cover -html $TMPDIR/cover.profile -o cover.html

	# Perform performance profiling.
	if [ $# != 0 ]; then
		go test -c github.com/dsnet/compress/$PKG_PATH
		if [ $? != 0 ]; then exit 1; fi
		./$PKG_NAME.test -test.cpuprofile $TMPDIR/cpu.profile -test.memprofile $TMPDIR/mem.profile -test.run - "$@"
		PPROF="go tool pprof"
		$PPROF -output=cpu.svg          -web                      $PKG_NAME.test $TMPDIR/cpu.profile 2> /dev/null
		$PPROF -output=cpu.html         -weblist=.                $PKG_NAME.test $TMPDIR/cpu.profile 2> /dev/null
		$PPROF -output=mem_objects.svg  -alloc_objects -web       $PKG_NAME.test $TMPDIR/mem.profile 2> /dev/null
		$PPROF -output=mem_objects.html -alloc_objects -weblist=. $PKG_NAME.test $TMPDIR/mem.profile 2> /dev/null
		$PPROF -output=mem_space.svg    -alloc_space   -web       $PKG_NAME.test $TMPDIR/mem.profile 2> /dev/null
		$PPROF -output=mem_space.html   -alloc_space   -weblist=. $PKG_NAME.test $TMPDIR/mem.profile 2> /dev/null
	fi

	rm -rf $DIR/_zprof_/$PKG_NAME
	mkdir -p $DIR/_zprof_/$PKG_NAME
	mv *.html *.svg $DIR/_zprof_/$PKG_NAME 2> /dev/null
)
