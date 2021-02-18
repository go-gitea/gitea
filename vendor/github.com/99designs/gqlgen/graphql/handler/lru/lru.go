package lru

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	lru "github.com/hashicorp/golang-lru"
)

type LRU struct {
	lru *lru.Cache
}

var _ graphql.Cache = &LRU{}

func New(size int) *LRU {
	cache, err := lru.New(size)
	if err != nil {
		// An error is only returned for non-positive cache size
		// and we already checked for that.
		panic("unexpected error creating cache: " + err.Error())
	}
	return &LRU{cache}
}

func (l LRU) Get(ctx context.Context, key string) (value interface{}, ok bool) {
	return l.lru.Get(key)
}

func (l LRU) Add(ctx context.Context, key string, value interface{}) {
	l.lru.Add(key, value)
}
