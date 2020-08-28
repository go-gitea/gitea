// Copyright 2019+ Klaus Post. All rights reserved.
// License information can be found in the LICENSE file.
// Based on work by Yann Collet, released under BSD License.

package zstd

/*
// encParams are not really used, just here for reference.
type encParams struct {
	// largest match distance : larger == more compression, more memory needed during decompression
	windowLog uint8

	// fully searched segment : larger == more compression, slower, more memory (useless for fast)
	chainLog uint8

	//  dispatch table : larger == faster, more memory
	hashLog uint8

	// < nb of searches : larger == more compression, slower
	searchLog uint8

	// < match length searched : larger == faster decompression, sometimes less compression
	minMatch uint8

	// acceptable match size for optimal parser (only) : larger == more compression, slower
	targetLength uint32

	// see ZSTD_strategy definition above
	strategy strategy
}

// strategy defines the algorithm to use when generating sequences.
type strategy uint8

const (
	// Compression strategies, listed from fastest to strongest
	strategyFast strategy = iota + 1
	strategyDfast
	strategyGreedy
	strategyLazy
	strategyLazy2
	strategyBtlazy2
	strategyBtopt
	strategyBtultra
	strategyBtultra2
	// note : new strategies _might_ be added in the future.
	//   Only the order (from fast to strong) is guaranteed

)

var defEncParams = [4][]encParams{
	{ // "default" - for any srcSize > 256 KB
		// W,  C,  H,  S,  L, TL, strat
		{19, 12, 13, 1, 6, 1, strategyFast},       // base for negative levels
		{19, 13, 14, 1, 7, 0, strategyFast},       // level  1
		{20, 15, 16, 1, 6, 0, strategyFast},       // level  2
		{21, 16, 17, 1, 5, 1, strategyDfast},      // level  3
		{21, 18, 18, 1, 5, 1, strategyDfast},      // level  4
		{21, 18, 19, 2, 5, 2, strategyGreedy},     // level  5
		{21, 19, 19, 3, 5, 4, strategyGreedy},     // level  6
		{21, 19, 19, 3, 5, 8, strategyLazy},       // level  7
		{21, 19, 19, 3, 5, 16, strategyLazy2},     // level  8
		{21, 19, 20, 4, 5, 16, strategyLazy2},     // level  9
		{22, 20, 21, 4, 5, 16, strategyLazy2},     // level 10
		{22, 21, 22, 4, 5, 16, strategyLazy2},     // level 11
		{22, 21, 22, 5, 5, 16, strategyLazy2},     // level 12
		{22, 21, 22, 5, 5, 32, strategyBtlazy2},   // level 13
		{22, 22, 23, 5, 5, 32, strategyBtlazy2},   // level 14
		{22, 23, 23, 6, 5, 32, strategyBtlazy2},   // level 15
		{22, 22, 22, 5, 5, 48, strategyBtopt},     // level 16
		{23, 23, 22, 5, 4, 64, strategyBtopt},     // level 17
		{23, 23, 22, 6, 3, 64, strategyBtultra},   // level 18
		{23, 24, 22, 7, 3, 256, strategyBtultra2}, // level 19
		{25, 25, 23, 7, 3, 256, strategyBtultra2}, // level 20
		{26, 26, 24, 7, 3, 512, strategyBtultra2}, // level 21
		{27, 27, 25, 9, 3, 999, strategyBtultra2}, // level 22
	},
	{ // for srcSize <= 256 KB
		// W,  C,  H,  S,  L,  T, strat
		{18, 12, 13, 1, 5, 1, strategyFast},        // base for negative levels
		{18, 13, 14, 1, 6, 0, strategyFast},        // level  1
		{18, 14, 14, 1, 5, 1, strategyDfast},       // level  2
		{18, 16, 16, 1, 4, 1, strategyDfast},       // level  3
		{18, 16, 17, 2, 5, 2, strategyGreedy},      // level  4.
		{18, 18, 18, 3, 5, 2, strategyGreedy},      // level  5.
		{18, 18, 19, 3, 5, 4, strategyLazy},        // level  6.
		{18, 18, 19, 4, 4, 4, strategyLazy},        // level  7
		{18, 18, 19, 4, 4, 8, strategyLazy2},       // level  8
		{18, 18, 19, 5, 4, 8, strategyLazy2},       // level  9
		{18, 18, 19, 6, 4, 8, strategyLazy2},       // level 10
		{18, 18, 19, 5, 4, 12, strategyBtlazy2},    // level 11.
		{18, 19, 19, 7, 4, 12, strategyBtlazy2},    // level 12.
		{18, 18, 19, 4, 4, 16, strategyBtopt},      // level 13
		{18, 18, 19, 4, 3, 32, strategyBtopt},      // level 14.
		{18, 18, 19, 6, 3, 128, strategyBtopt},     // level 15.
		{18, 19, 19, 6, 3, 128, strategyBtultra},   // level 16.
		{18, 19, 19, 8, 3, 256, strategyBtultra},   // level 17.
		{18, 19, 19, 6, 3, 128, strategyBtultra2},  // level 18.
		{18, 19, 19, 8, 3, 256, strategyBtultra2},  // level 19.
		{18, 19, 19, 10, 3, 512, strategyBtultra2}, // level 20.
		{18, 19, 19, 12, 3, 512, strategyBtultra2}, // level 21.
		{18, 19, 19, 13, 3, 999, strategyBtultra2}, // level 22.
	},
	{ // for srcSize <= 128 KB
		// W,  C,  H,  S,  L,  T, strat
		{17, 12, 12, 1, 5, 1, strategyFast},        // base for negative levels
		{17, 12, 13, 1, 6, 0, strategyFast},        // level  1
		{17, 13, 15, 1, 5, 0, strategyFast},        // level  2
		{17, 15, 16, 2, 5, 1, strategyDfast},       // level  3
		{17, 17, 17, 2, 4, 1, strategyDfast},       // level  4
		{17, 16, 17, 3, 4, 2, strategyGreedy},      // level  5
		{17, 17, 17, 3, 4, 4, strategyLazy},        // level  6
		{17, 17, 17, 3, 4, 8, strategyLazy2},       // level  7
		{17, 17, 17, 4, 4, 8, strategyLazy2},       // level  8
		{17, 17, 17, 5, 4, 8, strategyLazy2},       // level  9
		{17, 17, 17, 6, 4, 8, strategyLazy2},       // level 10
		{17, 17, 17, 5, 4, 8, strategyBtlazy2},     // level 11
		{17, 18, 17, 7, 4, 12, strategyBtlazy2},    // level 12
		{17, 18, 17, 3, 4, 12, strategyBtopt},      // level 13.
		{17, 18, 17, 4, 3, 32, strategyBtopt},      // level 14.
		{17, 18, 17, 6, 3, 256, strategyBtopt},     // level 15.
		{17, 18, 17, 6, 3, 128, strategyBtultra},   // level 16.
		{17, 18, 17, 8, 3, 256, strategyBtultra},   // level 17.
		{17, 18, 17, 10, 3, 512, strategyBtultra},  // level 18.
		{17, 18, 17, 5, 3, 256, strategyBtultra2},  // level 19.
		{17, 18, 17, 7, 3, 512, strategyBtultra2},  // level 20.
		{17, 18, 17, 9, 3, 512, strategyBtultra2},  // level 21.
		{17, 18, 17, 11, 3, 999, strategyBtultra2}, // level 22.
	},
	{ // for srcSize <= 16 KB
		// W,  C,  H,  S,  L,  T, strat
		{14, 12, 13, 1, 5, 1, strategyFast},        // base for negative levels
		{14, 14, 15, 1, 5, 0, strategyFast},        // level  1
		{14, 14, 15, 1, 4, 0, strategyFast},        // level  2
		{14, 14, 15, 2, 4, 1, strategyDfast},       // level  3
		{14, 14, 14, 4, 4, 2, strategyGreedy},      // level  4
		{14, 14, 14, 3, 4, 4, strategyLazy},        // level  5.
		{14, 14, 14, 4, 4, 8, strategyLazy2},       // level  6
		{14, 14, 14, 6, 4, 8, strategyLazy2},       // level  7
		{14, 14, 14, 8, 4, 8, strategyLazy2},       // level  8.
		{14, 15, 14, 5, 4, 8, strategyBtlazy2},     // level  9.
		{14, 15, 14, 9, 4, 8, strategyBtlazy2},     // level 10.
		{14, 15, 14, 3, 4, 12, strategyBtopt},      // level 11.
		{14, 15, 14, 4, 3, 24, strategyBtopt},      // level 12.
		{14, 15, 14, 5, 3, 32, strategyBtultra},    // level 13.
		{14, 15, 15, 6, 3, 64, strategyBtultra},    // level 14.
		{14, 15, 15, 7, 3, 256, strategyBtultra},   // level 15.
		{14, 15, 15, 5, 3, 48, strategyBtultra2},   // level 16.
		{14, 15, 15, 6, 3, 128, strategyBtultra2},  // level 17.
		{14, 15, 15, 7, 3, 256, strategyBtultra2},  // level 18.
		{14, 15, 15, 8, 3, 256, strategyBtultra2},  // level 19.
		{14, 15, 15, 8, 3, 512, strategyBtultra2},  // level 20.
		{14, 15, 15, 9, 3, 512, strategyBtultra2},  // level 21.
		{14, 15, 15, 10, 3, 999, strategyBtultra2}, // level 22.
	},
}
*/
