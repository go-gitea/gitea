// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"testing"

	"gitea.dev/models/badges"
	giteajson "gitea.dev/modules/json"

	"github.com/stretchr/testify/assert"
)

func TestToBadgeOmitsColor(t *testing.T) {
	apiBadge := ToBadge(&badges.Badge{
		ID:          1,
		Slug:        "trusted",
		Description: "Trusted",
		Color:       "#ff0000",
		ImageURL:    "https://example.com/badge.png",
	})
	assert.Equal(t, int64(1), apiBadge.ID)
	assert.Equal(t, "trusted", apiBadge.Slug)
	assert.Equal(t, "Trusted", apiBadge.Description)
	assert.Equal(t, "https://example.com/badge.png", apiBadge.ImageURL)

	out, err := giteajson.Marshal(apiBadge)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"id":1,"slug":"trusted","description":"Trusted","image_url":"https://example.com/badge.png"}`, string(out))
	assert.NotContains(t, string(out), "Color")
	assert.NotContains(t, string(out), "color")
}

func TestToBadges(t *testing.T) {
	assert.Empty(t, ToBadges(nil))
	assert.Nil(t, ToBadge(nil))

	list := ToBadges([]*badges.Badge{
		{ID: 1, Slug: "a", Color: "#111111"},
		nil,
		{ID: 2, Slug: "b", Color: "#222222"},
	})
	assert.Len(t, list, 2)
	assert.Equal(t, "a", list[0].Slug)
	assert.Equal(t, "b", list[1].Slug)
}
