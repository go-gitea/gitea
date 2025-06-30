// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package zstd

import "github.com/klauspost/compress/zstd"

type WriterOption = zstd.EOption

var (
	WithEncoderCRC               = zstd.WithEncoderCRC
	WithEncoderConcurrency       = zstd.WithEncoderConcurrency
	WithWindowSize               = zstd.WithWindowSize
	WithEncoderPadding           = zstd.WithEncoderPadding
	WithEncoderLevel             = zstd.WithEncoderLevel
	WithZeroFrames               = zstd.WithZeroFrames
	WithAllLitEntropyCompression = zstd.WithAllLitEntropyCompression
	WithNoEntropyCompression     = zstd.WithNoEntropyCompression
	WithSingleSegment            = zstd.WithSingleSegment
	WithLowerEncoderMem          = zstd.WithLowerEncoderMem
	WithEncoderDict              = zstd.WithEncoderDict
	WithEncoderDictRaw           = zstd.WithEncoderDictRaw
)

type EncoderLevel = zstd.EncoderLevel

const (
	SpeedFastest           EncoderLevel = zstd.SpeedFastest
	SpeedDefault           EncoderLevel = zstd.SpeedDefault
	SpeedBetterCompression EncoderLevel = zstd.SpeedBetterCompression
	SpeedBestCompression   EncoderLevel = zstd.SpeedBestCompression
)

type ReaderOption = zstd.DOption

var (
	WithDecoderLowmem      = zstd.WithDecoderLowmem
	WithDecoderConcurrency = zstd.WithDecoderConcurrency
	WithDecoderMaxMemory   = zstd.WithDecoderMaxMemory
	WithDecoderDicts       = zstd.WithDecoderDicts
	WithDecoderDictRaw     = zstd.WithDecoderDictRaw
	WithDecoderMaxWindow   = zstd.WithDecoderMaxWindow
	WithDecodeAllCapLimit  = zstd.WithDecodeAllCapLimit
	WithDecodeBuffersBelow = zstd.WithDecodeBuffersBelow
	IgnoreChecksum         = zstd.IgnoreChecksum
)
