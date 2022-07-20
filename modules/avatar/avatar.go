// Copyright 2022 The Gitea Authors. All rights reserved.
// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package avatar

import (
	"bytes"
	"fmt"
	"image"

	_ "image/gif"  // for processing gif images
	_ "image/jpeg" // for processing jpeg images
	_ "image/png"  // for processing png images

	"code.gitea.io/gitea/modules/avatar/dicebear"
	"code.gitea.io/gitea/modules/avatar/identicon"
	"code.gitea.io/gitea/modules/avatar/none"
	"code.gitea.io/gitea/modules/avatar/robot"
	"code.gitea.io/gitea/modules/setting"

	"github.com/nfnt/resize"
	"github.com/oliamb/cutter"
)

// AvatarSize returns avatar's size
const AvatarSize = 290

// Kind represent the type an avatar will be generated for
type Kind uint

const (
	// User represent users
	KindUser Kind = 0
	// Repo represent repositorys
	KindRepo Kind = 1
	// Org represent organisations
	KindOrg Kind = 2
)

type randomImageGenerator interface {
	Name() string
}

type randomUserImageGenerator interface {
	randomImageGenerator
	RandomUserImage(int, []byte) (image.Image, error)
}

type randomOrgImageGenerator interface {
	randomImageGenerator
	RandomOrgImage(int, []byte) (image.Image, error)
}

type randomRepoImageGenerator interface {
	randomImageGenerator
	RandomRepoImage(int, []byte) (image.Image, error)
}

var (
	userImageGenerator randomUserImageGenerator = identicon.Identicon{}
	orgImageGenerator  randomOrgImageGenerator  = identicon.Identicon{}
	repoImageGenerator randomRepoImageGenerator = identicon.Identicon{}
	generators                                  = []randomImageGenerator{
		dicebear.DiceBear{},
		identicon.Identicon{},
		none.None{},
		robot.Robot{},
	}
)

// TODO: Init()
func init() {
	userPreference := "none"
	orgPreference := "none"
	repoPreference := setting.RepoAvatar.FallbackImage

	for _, g := range generators {
		if g, ok := g.(randomUserImageGenerator); ok && userPreference == g.Name() {
			userImageGenerator = g
		}
		if g, ok := g.(randomOrgImageGenerator); ok && orgPreference == g.Name() {
			orgImageGenerator = g
		}
		if g, ok := g.(randomRepoImageGenerator); ok && repoPreference == g.Name() {
			repoImageGenerator = g
		}
	}
}

// RandomImageSize generates and returns a random avatar image unique to input data
// in custom size (height and width).
func RandomImageSize(kind Kind, size int, seed []byte) (image.Image, error) {
	switch kind {
	case KindUser:
		return userImageGenerator.RandomUserImage(size, seed)
	case KindOrg:
		return orgImageGenerator.RandomOrgImage(size, seed)
	case KindRepo:
		return repoImageGenerator.RandomRepoImage(size, seed)
	default:
		return nil, fmt.Errorf("avatar kind %v not supported", kind)
	}
}

// RandomImage generates and returns a random avatar image unique to input data
// in default size (height and width).
func RandomImage(kind Kind, seed []byte) (image.Image, error) {
	return RandomImageSize(kind, AvatarSize, seed)
}

// Prepare accepts a byte slice as input, validates it contains an image of an
// acceptable format, and crops and resizes it appropriately.
func Prepare(data []byte) (*image.Image, error) {
	imgCfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("DecodeConfig: %v", err)
	}
	if imgCfg.Width > setting.Avatar.MaxWidth {
		return nil, fmt.Errorf("Image width is too large: %d > %d", imgCfg.Width, setting.Avatar.MaxWidth)
	}
	if imgCfg.Height > setting.Avatar.MaxHeight {
		return nil, fmt.Errorf("Image height is too large: %d > %d", imgCfg.Height, setting.Avatar.MaxHeight)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("Decode: %v", err)
	}

	if imgCfg.Width != imgCfg.Height {
		var newSize, ax, ay int
		if imgCfg.Width > imgCfg.Height {
			newSize = imgCfg.Height
			ax = (imgCfg.Width - imgCfg.Height) / 2
		} else {
			newSize = imgCfg.Width
			ay = (imgCfg.Height - imgCfg.Width) / 2
		}

		img, err = cutter.Crop(img, cutter.Config{
			Width:  newSize,
			Height: newSize,
			Anchor: image.Point{ax, ay},
		})
		if err != nil {
			return nil, err
		}
	}

	img = resize.Resize(AvatarSize, AvatarSize, img, resize.Bilinear)
	return &img, nil
}
