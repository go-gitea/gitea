// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

// CommentList defines a list of comments
type CommentList []*Comment

func (comments CommentList) getPosterIDs() []int64 {
	commentIDs := make(map[int64]struct{}, len(comments))
	for _, comment := range comments {
		if _, ok := commentIDs[comment.PosterID]; !ok {
			commentIDs[comment.PosterID] = struct{}{}
		}
	}
	return keysInt64(commentIDs)
}

// LoadPosters loads posters from database
func (comments CommentList) LoadPosters() error {
	return comments.loadPosters(x)
}

func (comments CommentList) loadPosters(e Engine) error {
	if len(comments) == 0 {
		return nil
	}

	posterIDs := comments.getPosterIDs()
	posterMaps := make(map[int64]*User, len(posterIDs))
	var left = len(posterIDs)
	for left > 0 {
		var limit = defaultMaxInSize
		if left < limit {
			limit = left
		}
		err := e.
			In("id", posterIDs[:limit]).
			Find(&posterMaps)
		if err != nil {
			return err
		}
		left = left - limit
		posterIDs = posterIDs[limit:]
	}

	for _, comment := range comments {
		if comment.PosterID <= 0 {
			continue
		}
		var ok bool
		if comment.Poster, ok = posterMaps[comment.PosterID]; !ok {
			comment.Poster = NewGhostUser()
		}
	}
	return nil
}
