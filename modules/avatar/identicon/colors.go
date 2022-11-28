// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package identicon

import "image/color"

// DarkColors are dark colors for avatar blocks, they come from image/color/palette.WebSafe, and light colors (0xff) are removed
var DarkColors = []color.Color{
	color.RGBA{0x00, 0x00, 0x33, 0xff},
	color.RGBA{0x00, 0x00, 0x66, 0xff},
	color.RGBA{0x00, 0x00, 0x99, 0xff},
	color.RGBA{0x00, 0x00, 0xcc, 0xff},
	color.RGBA{0x00, 0x33, 0x00, 0xff},
	color.RGBA{0x00, 0x33, 0x33, 0xff},
	color.RGBA{0x00, 0x33, 0x66, 0xff},
	color.RGBA{0x00, 0x33, 0x99, 0xff},
	color.RGBA{0x00, 0x33, 0xcc, 0xff},
	color.RGBA{0x00, 0x66, 0x00, 0xff},
	color.RGBA{0x00, 0x66, 0x33, 0xff},
	color.RGBA{0x00, 0x66, 0x66, 0xff},
	color.RGBA{0x00, 0x66, 0x99, 0xff},
	color.RGBA{0x00, 0x66, 0xcc, 0xff},
	color.RGBA{0x00, 0x99, 0x00, 0xff},
	color.RGBA{0x00, 0x99, 0x33, 0xff},
	color.RGBA{0x00, 0x99, 0x66, 0xff},
	color.RGBA{0x00, 0x99, 0x99, 0xff},
	color.RGBA{0x00, 0x99, 0xcc, 0xff},
	color.RGBA{0x00, 0xcc, 0x00, 0xff},
	color.RGBA{0x00, 0xcc, 0x33, 0xff},
	color.RGBA{0x00, 0xcc, 0x66, 0xff},
	color.RGBA{0x00, 0xcc, 0x99, 0xff},
	color.RGBA{0x00, 0xcc, 0xcc, 0xff},
	color.RGBA{0x33, 0x00, 0x00, 0xff},
	color.RGBA{0x33, 0x00, 0x33, 0xff},
	color.RGBA{0x33, 0x00, 0x66, 0xff},
	color.RGBA{0x33, 0x00, 0x99, 0xff},
	color.RGBA{0x33, 0x00, 0xcc, 0xff},
	color.RGBA{0x33, 0x33, 0x00, 0xff},
	color.RGBA{0x33, 0x33, 0x33, 0xff},
	color.RGBA{0x33, 0x33, 0x66, 0xff},
	color.RGBA{0x33, 0x33, 0x99, 0xff},
	color.RGBA{0x33, 0x33, 0xcc, 0xff},
	color.RGBA{0x33, 0x66, 0x00, 0xff},
	color.RGBA{0x33, 0x66, 0x33, 0xff},
	color.RGBA{0x33, 0x66, 0x66, 0xff},
	color.RGBA{0x33, 0x66, 0x99, 0xff},
	color.RGBA{0x33, 0x66, 0xcc, 0xff},
	color.RGBA{0x33, 0x99, 0x00, 0xff},
	color.RGBA{0x33, 0x99, 0x33, 0xff},
	color.RGBA{0x33, 0x99, 0x66, 0xff},
	color.RGBA{0x33, 0x99, 0x99, 0xff},
	color.RGBA{0x33, 0x99, 0xcc, 0xff},
	color.RGBA{0x33, 0xcc, 0x00, 0xff},
	color.RGBA{0x33, 0xcc, 0x33, 0xff},
	color.RGBA{0x33, 0xcc, 0x66, 0xff},
	color.RGBA{0x33, 0xcc, 0x99, 0xff},
	color.RGBA{0x33, 0xcc, 0xcc, 0xff},
	color.RGBA{0x66, 0x00, 0x00, 0xff},
	color.RGBA{0x66, 0x00, 0x33, 0xff},
	color.RGBA{0x66, 0x00, 0x66, 0xff},
	color.RGBA{0x66, 0x00, 0x99, 0xff},
	color.RGBA{0x66, 0x00, 0xcc, 0xff},
	color.RGBA{0x66, 0x33, 0x00, 0xff},
	color.RGBA{0x66, 0x33, 0x33, 0xff},
	color.RGBA{0x66, 0x33, 0x66, 0xff},
	color.RGBA{0x66, 0x33, 0x99, 0xff},
	color.RGBA{0x66, 0x33, 0xcc, 0xff},
	color.RGBA{0x66, 0x66, 0x00, 0xff},
	color.RGBA{0x66, 0x66, 0x33, 0xff},
	color.RGBA{0x66, 0x66, 0x66, 0xff},
	color.RGBA{0x66, 0x66, 0x99, 0xff},
	color.RGBA{0x66, 0x66, 0xcc, 0xff},
	color.RGBA{0x66, 0x99, 0x00, 0xff},
	color.RGBA{0x66, 0x99, 0x33, 0xff},
	color.RGBA{0x66, 0x99, 0x66, 0xff},
	color.RGBA{0x66, 0x99, 0x99, 0xff},
	color.RGBA{0x66, 0x99, 0xcc, 0xff},
	color.RGBA{0x66, 0xcc, 0x00, 0xff},
	color.RGBA{0x66, 0xcc, 0x33, 0xff},
	color.RGBA{0x66, 0xcc, 0x66, 0xff},
	color.RGBA{0x66, 0xcc, 0x99, 0xff},
	color.RGBA{0x66, 0xcc, 0xcc, 0xff},
	color.RGBA{0x99, 0x00, 0x00, 0xff},
	color.RGBA{0x99, 0x00, 0x33, 0xff},
	color.RGBA{0x99, 0x00, 0x66, 0xff},
	color.RGBA{0x99, 0x00, 0x99, 0xff},
	color.RGBA{0x99, 0x00, 0xcc, 0xff},
	color.RGBA{0x99, 0x33, 0x00, 0xff},
	color.RGBA{0x99, 0x33, 0x33, 0xff},
	color.RGBA{0x99, 0x33, 0x66, 0xff},
	color.RGBA{0x99, 0x33, 0x99, 0xff},
	color.RGBA{0x99, 0x33, 0xcc, 0xff},
	color.RGBA{0x99, 0x66, 0x00, 0xff},
	color.RGBA{0x99, 0x66, 0x33, 0xff},
	color.RGBA{0x99, 0x66, 0x66, 0xff},
	color.RGBA{0x99, 0x66, 0x99, 0xff},
	color.RGBA{0x99, 0x66, 0xcc, 0xff},
	color.RGBA{0x99, 0x99, 0x00, 0xff},
	color.RGBA{0x99, 0x99, 0x33, 0xff},
	color.RGBA{0x99, 0x99, 0x66, 0xff},
	color.RGBA{0x99, 0x99, 0x99, 0xff},
	color.RGBA{0x99, 0x99, 0xcc, 0xff},
	color.RGBA{0x99, 0xcc, 0x00, 0xff},
	color.RGBA{0x99, 0xcc, 0x33, 0xff},
	color.RGBA{0x99, 0xcc, 0x66, 0xff},
	color.RGBA{0x99, 0xcc, 0x99, 0xff},
	color.RGBA{0x99, 0xcc, 0xcc, 0xff},
	color.RGBA{0xcc, 0x00, 0x00, 0xff},
	color.RGBA{0xcc, 0x00, 0x33, 0xff},
	color.RGBA{0xcc, 0x00, 0x66, 0xff},
	color.RGBA{0xcc, 0x00, 0x99, 0xff},
	color.RGBA{0xcc, 0x00, 0xcc, 0xff},
	color.RGBA{0xcc, 0x33, 0x00, 0xff},
	color.RGBA{0xcc, 0x33, 0x33, 0xff},
	color.RGBA{0xcc, 0x33, 0x66, 0xff},
	color.RGBA{0xcc, 0x33, 0x99, 0xff},
	color.RGBA{0xcc, 0x33, 0xcc, 0xff},
	color.RGBA{0xcc, 0x66, 0x00, 0xff},
	color.RGBA{0xcc, 0x66, 0x33, 0xff},
	color.RGBA{0xcc, 0x66, 0x66, 0xff},
	color.RGBA{0xcc, 0x66, 0x99, 0xff},
	color.RGBA{0xcc, 0x66, 0xcc, 0xff},
	color.RGBA{0xcc, 0x99, 0x00, 0xff},
	color.RGBA{0xcc, 0x99, 0x33, 0xff},
	color.RGBA{0xcc, 0x99, 0x66, 0xff},
	color.RGBA{0xcc, 0x99, 0x99, 0xff},
	color.RGBA{0xcc, 0x99, 0xcc, 0xff},
	color.RGBA{0xcc, 0xcc, 0x00, 0xff},
	color.RGBA{0xcc, 0xcc, 0x33, 0xff},
	color.RGBA{0xcc, 0xcc, 0x66, 0xff},
	color.RGBA{0xcc, 0xcc, 0x99, 0xff},
	color.RGBA{0xcc, 0xcc, 0xcc, 0xff},
}
