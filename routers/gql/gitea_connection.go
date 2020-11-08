// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gql

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"

	"github.com/seripap/relay"
)

/*
 Implementation of a relay connection using gitea specific pagination information. Adapted from
 arrayconnection implementation in github.com/seripap/relay implementation.
*/

const prefix = "giteaconnection:"

// GetListOptions get the gitea list options for pagination based on the graphql pagination args and the total size of the data
func GetListOptions(totalSize int, args relay.ConnectionArguments, maxPageSize int) models.ListOptions {
	var (
		offset   int
		pageSize int
		first    = args.First
		last     = args.Last
		before   = getOffsetWithDefault(args.Before, -1)
		after    = getOffsetWithDefault(args.After, -1)
	)

	if first > -1 {
		if first > maxPageSize {
			first = maxPageSize
		}
		if after == -1 && before == -1 {
			//no cursor
			pageSize = first
		} else if after > -1 {
			//first x after cursor
			pageSize = first
			if after > totalSize {
				after = totalSize
			}
			offset = after
		} else if before > -1 {
			//first x before cursor
			if before > totalSize {
				before = totalSize
			}
			if first >= before {
				first = before - 1
			}
			pageSize = first
		}
	} else if last > -1 {
		if last > maxPageSize {
			last = maxPageSize
		}
		if after == -1 && before == -1 {
			//no cursor
			if last > totalSize {
				last = totalSize
			}
			offset = totalSize - last
			pageSize = last
		} else if before > -1 {
			//last x before cursor
			if before > totalSize {
				before = totalSize
			}
			offset = before - last - 1
			if offset < 0 {
				last += offset
				offset = 0
			}
			pageSize = last
		} else if after > -1 {
			//last x after cursor
			if after > totalSize {
				after = totalSize
			}
			offset = totalSize - last
			if offset < after {
				offset = after
			}
			pageSize = last
		}
	}
	return models.ListOptions{
		Page:     1, //gitea pagination will use offset rather than page number, but need to default to number other than 0
		Offset:   offset,
		PageSize: pageSize,
	}
}

// GiteaRelayConnection returns a relay connection object based on gitea paging state
func GiteaRelayConnection(data []interface{}, startPosition int, totalSize int) *relay.Connection {
	cursorPosition := startPosition
	edges := []*relay.Edge{}
	nodes := []interface{}{}
	for _, value := range data {
		edges = append(edges, &relay.Edge{
			Cursor: offsetToCursor(cursorPosition),
			Node:   value,
		})
		cursorPosition++
		nodes = append(nodes, value)
	}

	var firstEdgeCursor, lastEdgeCursor relay.ConnectionCursor
	if len(edges) > 0 {
		firstEdgeCursor = edges[0].Cursor
		lastEdgeCursor = edges[len(edges)-1:][0].Cursor
	}

	conn := relay.NewConnection()
	conn.Edges = edges
	conn.Nodes = nodes
	conn.PageInfo = relay.PageInfo{
		StartCursor:     firstEdgeCursor,
		EndCursor:       lastEdgeCursor,
		HasPreviousPage: startPosition > 1,
		HasNextPage:     (startPosition-1)+len(data) < totalSize,
	}
	conn.TotalCount = totalSize

	return conn
}

func offsetToCursor(offset int) relay.ConnectionCursor {
	str := fmt.Sprintf("%v%v", prefix, offset)
	return relay.ConnectionCursor(base64.StdEncoding.EncodeToString([]byte(str)))
}

func cursorToOffset(cursor relay.ConnectionCursor) (int, error) {
	str := ""
	b, err := base64.StdEncoding.DecodeString(string(cursor))
	if err == nil {
		str = string(b)
	}
	str = strings.ReplaceAll(str, prefix, "")
	offset, err := strconv.Atoi(str)
	if err != nil {
		return 0, errors.New("Invalid cursor")
	}
	return offset, nil
}

func getOffsetWithDefault(cursor relay.ConnectionCursor, defaultOffset int) int {
	if cursor == "" {
		return defaultOffset
	}
	offset, err := cursorToOffset(cursor)
	if err != nil {
		return defaultOffset
	}
	return offset
}
