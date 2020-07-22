package relay

import (
	"fmt"
)

type ConnectionCursor string

type PageInfo struct {
	StartCursor     ConnectionCursor `json:"startCursor"`
	EndCursor       ConnectionCursor `json:"endCursor"`
	HasPreviousPage bool             `json:"hasPreviousPage"`
	HasNextPage     bool             `json:"hasNextPage"`
}

type Connection struct {
	Edges      []*Edge       `json:"edges"`
	Nodes      []interface{} `json:"nodes"`
	PageInfo   PageInfo      `json:"pageInfo"`
	TotalCount int           `json:"totalCount"`
}

func NewConnection() *Connection {
	return &Connection{
		Edges:      []*Edge{},
		Nodes:      []interface{}{},
		PageInfo:   PageInfo{},
		TotalCount: 0,
	}
}

type Edge struct {
	Node   interface{}      `json:"node"`
	Cursor ConnectionCursor `json:"cursor"`
}

// Use NewConnectionArguments() to properly initialize default values
type ConnectionArguments struct {
	Before ConnectionCursor `json:"before"`
	After  ConnectionCursor `json:"after"`
	First  int              `json:"first"` // -1 for undefined, 0 would return zero results
	Last   int              `json:"last"`  //  -1 for undefined, 0 would return zero results
}
type ConnectionArgumentsConfig struct {
	Before ConnectionCursor `json:"before"`
	After  ConnectionCursor `json:"after"`

	// use pointers for `First` and `Last` fields
	// so constructor would know when to use default values
	First *int `json:"first"`
	Last  *int `json:"last"`
}

func NewConnectionArguments(filters map[string]interface{}) ConnectionArguments {
	conn := ConnectionArguments{
		First:  -1,
		Last:   -1,
		Before: "",
		After:  "",
	}
	if filters != nil {
		if first, ok := filters["first"]; ok {
			if first, ok := first.(int); ok {
				conn.First = first
			}
		}
		if last, ok := filters["last"]; ok {
			if last, ok := last.(int); ok {
				conn.Last = last
			}
		}
		if before, ok := filters["before"]; ok {
			conn.Before = ConnectionCursor(fmt.Sprintf("%v", before))
		}
		if after, ok := filters["after"]; ok {
			conn.After = ConnectionCursor(fmt.Sprintf("%v", after))
		}
	}
	return conn
}
